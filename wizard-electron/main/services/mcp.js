/**
 * MCP Service - Handles MCP initialization and testing
 *
 * Verification gatekeeper: gres-b2b mcp selftest must perform
 * mock initialize handshake (spawn serve, pipe initialize, validate response)
 */

const { spawn } = require("child_process");
const { BINARY_PATH } = require("./config");

/**
 * MCP Initialize request (JSON-RPC 2.0)
 */
const INITIALIZE_REQUEST = JSON.stringify({
  jsonrpc: "2.0",
  id: 1,
  method: "initialize",
  params: {
    protocolVersion: "2024-11-05",
    capabilities: {},
    clientInfo: {
      name: "gres-b2b-wizard",
      version: "1.0.0",
    },
  },
});

/**
 * Perform real MCP handshake test
 * Spawns gres-b2b mcp serve, pipes initialize request, validates response
 */
async function testHandshake(opts = {}) {
  const timeout = opts.timeout || 10000;

  return new Promise((resolve) => {
    let proc;

    try {
      // Spawn the MCP serve command
      proc = spawn(BINARY_PATH, ["mcp", "serve"], {
        windowsHide: true,
        stdio: ["pipe", "pipe", "pipe"],
      });
    } catch (err) {
      resolve({
        success: false,
        error: `Failed to spawn MCP server: ${err.message}`,
        phase: "spawn",
      });
      return;
    }

    let stdout = "";
    let stderr = "";
    let resolved = false;

    const cleanup = () => {
      if (!resolved) {
        resolved = true;
        try {
          proc.kill();
        } catch (e) {
          // Ignore kill errors
        }
      }
    };

    // Timeout handler
    const timeoutId = setTimeout(() => {
      cleanup();
      resolve({
        success: false,
        error: "MCP handshake timed out waiting for response",
        phase: "timeout",
        details: { stdout, stderr },
      });
    }, timeout);

    proc.stdout.on("data", (data) => {
      stdout += data.toString();

      // Check if we got a valid JSON-RPC response
      try {
        const lines = stdout.split("\n").filter((l) => l.trim());
        for (const line of lines) {
          const response = JSON.parse(line);

          if (response.jsonrpc === "2.0" && response.id === 1) {
            clearTimeout(timeoutId);
            cleanup();

            if (response.result) {
              // Successful initialize response
              resolve({
                success: true,
                message: "MCP handshake successful",
                protocol: "stdio",
                serverInfo: response.result.serverInfo,
                capabilities: response.result.capabilities,
                protocolVersion: response.result.protocolVersion,
              });
            } else if (response.error) {
              resolve({
                success: false,
                error: `MCP error: ${response.error.message || JSON.stringify(response.error)}`,
                phase: "response",
                details: response.error,
              });
            }
            return;
          }
        }
      } catch (e) {
        // Not valid JSON yet, keep waiting
      }
    });

    proc.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    proc.on("close", (code) => {
      clearTimeout(timeoutId);
      if (!resolved) {
        resolved = true;
        if (code === 0 && stdout.includes('"result"')) {
          resolve({
            success: true,
            message: "MCP handshake completed",
            protocol: "stdio",
          });
        } else {
          resolve({
            success: false,
            error: `MCP server exited with code ${code}`,
            phase: "exit",
            details: { stdout, stderr, code },
          });
        }
      }
    });

    proc.on("error", (err) => {
      clearTimeout(timeoutId);
      cleanup();
      resolve({
        success: false,
        error: `MCP server error: ${err.message}`,
        phase: "error",
        details: { error: err.message },
      });
    });

    // Send the initialize request
    try {
      proc.stdin.write(INITIALIZE_REQUEST + "\n");
    } catch (err) {
      clearTimeout(timeoutId);
      cleanup();
      resolve({
        success: false,
        error: `Failed to send initialize request: ${err.message}`,
        phase: "write",
      });
    }
  });
}

/**
 * Run gres-b2b mcp selftest command (fallback method)
 */
async function testSelftest(opts = {}) {
  const timeout = opts.timeout || 10000;

  return new Promise((resolve) => {
    const proc = spawn(BINARY_PATH, ["mcp", "selftest"], {
      windowsHide: true,
    });

    let stdout = "";
    let stderr = "";

    proc.stdout.on("data", (data) => {
      stdout += data.toString();
    });

    proc.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    const timeoutId = setTimeout(() => {
      proc.kill();
      resolve({
        success: false,
        error: "MCP selftest timed out",
        details: { stdout, stderr },
      });
    }, timeout);

    proc.on("close", (code) => {
      clearTimeout(timeoutId);

      if (code === 0) {
        resolve({
          success: true,
          message: "MCP selftest passed",
          protocol: "stdio",
          details: { stdout: stdout.trim() },
        });
      } else {
        resolve({
          success: false,
          error: `MCP selftest failed (exit code ${code})`,
          details: { stdout, stderr },
        });
      }
    });

    proc.on("error", (err) => {
      clearTimeout(timeoutId);
      resolve({
        success: false,
        error: `Failed to run MCP selftest: ${err.message}`,
        details: { error: err.message },
      });
    });
  });
}

/**
 * Main selftest function - tries handshake first, falls back to selftest command
 */
async function selftest(opts = {}) {
  // First try the real handshake
  const handshakeResult = await testHandshake(opts);

  if (handshakeResult.success) {
    return handshakeResult;
  }

  // If handshake fails, try the selftest command as fallback
  const selftestResult = await testSelftest(opts);

  if (selftestResult.success) {
    return selftestResult;
  }

  // Both failed - return the handshake error as it's more informative
  return handshakeResult;
}

/**
 * Test MCP initialize (alias for selftest)
 */
async function testInitialize(opts = {}) {
  return selftest(opts);
}

module.exports = {
  testInitialize,
  testHandshake,
  testSelftest,
  selftest,
};
