/**
 * MCP Service - Handles MCP initialization and testing
 *
 * Uses gres-b2b mcp selftest as the agent-agnostic gatekeeper
 */

const { spawn } = require("child_process");
const path = require("path");
const os = require("os");

const INSTALL_DIR = path.join(os.homedir(), "AppData", "Local", "Programs", "gres-b2b");
const BINARY_NAME = "gres-b2b.exe";

/**
 * Test MCP initialize handshake
 * This is the agent-agnostic approach using gres-b2b mcp selftest
 */
async function testInitialize(opts = {}) {
  const timeout = opts.timeout || 5000;

  return new Promise((resolve) => {
    const binaryPath = path.join(INSTALL_DIR, BINARY_NAME);

    const proc = spawn(binaryPath, ["mcp", "selftest"], {
      windowsHide: true,
      timeout: timeout,
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
 * Run gres-b2b mcp selftest directly
 * This is the recommended agent-agnostic approach
 */
async function selftest() {
  return testInitialize({ timeout: 10000 });
}

module.exports = {
  testInitialize,
  selftest,
};
