/**
 * Scan Service - Handles governance scans with streaming output
 */

const { spawn } = require("child_process");
const path = require("path");
const os = require("os");
const fs = require("fs");

const INSTALL_DIR = path.join(os.homedir(), "AppData", "Local", "Programs", "gres-b2b");
const BINARY_NAME = "gres-b2b.exe";
const CONFIG_DIR = path.join(os.homedir(), "AppData", "Local", "gres-b2b");
const CONFIG_NAME = "config.toml";

/**
 * Start a governance scan with streaming output
 * @param {WebContents} sender - Electron WebContents to send events to
 * @param {Object} opts - Scan options
 */
async function start(sender, opts = {}) {
  return new Promise((resolve) => {
    const binaryPath = path.join(INSTALL_DIR, BINARY_NAME);
    const configPath = path.join(CONFIG_DIR, CONFIG_NAME);
    const projectPath = opts.projectPath || process.cwd();

    if (!fs.existsSync(binaryPath)) {
      return resolve({
        success: false,
        error: "Binary not found",
      });
    }

    // Build args
    const args = [];

    if (fs.existsSync(configPath)) {
      args.push("--config", configPath);
    }

    args.push("scan");

    if (opts.live) {
      args.push("--live");
    }

    if (opts.source) {
      args.push("--source", opts.source);
    } else if (projectPath) {
      args.push("--source", projectPath);
    }

    // Spawn the process
    const proc = spawn(binaryPath, args, {
      cwd: projectPath,
      windowsHide: true,
    });

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
  });
}

module.exports = {
  start,
};
