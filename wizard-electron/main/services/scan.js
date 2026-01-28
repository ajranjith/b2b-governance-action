/**
 * Scan Service - Handles governance scans with streaming output
 *
 * Supports detached mode: gres-b2b scan --live --source <target>
 * Opens progress UI and continues after wizard closes.
 */

const { spawn } = require("child_process");
const fs = require("fs");
const { BINARY_PATH, INSTALL_DIR } = require("./config");

/**
 * Start a governance scan with streaming output
 * @param {WebContents} sender - Electron WebContents to send events to
 * @param {Object} opts - Scan options
 */
async function start(sender, opts = {}) {
  return new Promise((resolve) => {
    if (!fs.existsSync(BINARY_PATH)) {
      return resolve({
        success: false,
        error: "Binary not found at " + BINARY_PATH,
      });
    }

    // Build args
    const args = ["scan"];

    if (opts.live) {
      args.push("--live");
    }

    if (opts.source) {
      args.push("--source", opts.source);
    }

    if (opts.port) {
      args.push("--port", String(opts.port));
    }

    // Spawn the process
    const proc = spawn(BINARY_PATH, args, {
      cwd: opts.source || process.cwd(),
      windowsHide: true,
      detached: opts.detached || false,
    });

    // If detached, unref so parent can exit
    if (opts.detached) {
      proc.unref();
    }

    let stdout = "";
    let stderr = "";

    // Stream stdout to renderer
    proc.stdout.on("data", (data) => {
      const text = data.toString();
      stdout += text;

      // Send event to renderer
      sender.send("scan:event", {
        type: "stdout",
        data: text,
      });

      // Try to parse structured output
      try {
        const lines = text.split("\n");
        for (const line of lines) {
          if (line.trim()) {
            // Check for RED/AMBER/GREEN indicators
            if (line.includes("RED")) {
              sender.send("scan:event", { type: "violation", level: "red", line });
            } else if (line.includes("AMBER")) {
              sender.send("scan:event", { type: "violation", level: "amber", line });
            } else if (line.includes("GREEN")) {
              sender.send("scan:event", { type: "status", level: "green", line });
            }
          }
        }
      } catch (e) {
        // Ignore parse errors
      }
    });

    // Stream stderr to renderer
    proc.stderr.on("data", (data) => {
      const text = data.toString();
      stderr += text;

      sender.send("scan:event", {
        type: "stderr",
        data: text,
      });
    });

    proc.on("close", (code) => {
      sender.send("scan:event", {
        type: "complete",
        code: code,
        success: code === 0,
      });

      if (code === 0) {
        resolve({
          success: true,
          output: stdout.trim(),
          message: "Scan completed successfully",
        });
      } else {
        resolve({
          success: false,
          error: `Scan failed (exit ${code})`,
          output: stdout.trim(),
          stderr: stderr.trim(),
        });
      }
    });

    proc.on("error", (err) => {
      sender.send("scan:event", {
        type: "error",
        error: err.message,
      });

      resolve({
        success: false,
        error: `Failed to run scan: ${err.message}`,
      });
    });

    // For detached mode, resolve immediately
    if (opts.detached) {
      resolve({
        success: true,
        detached: true,
        pid: proc.pid,
        message: "Scan started in background",
      });
    }
  });
}

/**
 * Start a detached scan that continues after wizard closes
 * Opens the progress UI in browser
 */
async function startDetached(opts = {}) {
  if (!fs.existsSync(BINARY_PATH)) {
    return {
      success: false,
      error: "Binary not found at " + BINARY_PATH,
    };
  }

  const args = ["scan", "--live"];

  if (opts.source) {
    args.push("--source", opts.source);
  }

  if (opts.port) {
    args.push("--port", String(opts.port));
  }

  try {
    const proc = spawn(BINARY_PATH, args, {
      cwd: opts.source || process.cwd(),
      windowsHide: true,
      detached: true,
      stdio: "ignore",
    });

    proc.unref();

    return {
      success: true,
      detached: true,
      pid: proc.pid,
      port: opts.port || 8080,
      url: `http://localhost:${opts.port || 8080}`,
      message: "Scan started in background",
    };
  } catch (err) {
    return {
      success: false,
      error: `Failed to start scan: ${err.message}`,
    };
  }
}

module.exports = {
  start,
  startDetached,
};
