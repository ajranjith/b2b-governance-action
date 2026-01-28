/**
 * Verify Service - Handles binary and PATH verification
 */

const { spawn, execSync } = require("child_process");
const path = require("path");
const os = require("os");
const fs = require("fs");

const INSTALL_DIR = path.join(os.homedir(), "AppData", "Local", "Programs", "gres-b2b");
const BINARY_NAME = "gres-b2b.exe";
const CONFIG_DIR = path.join(os.homedir(), "AppData", "Local", "gres-b2b");
const CONFIG_NAME = "config.toml";

/**
 * Verify binary version by running --version
 */
async function binaryVersion() {
  return new Promise((resolve) => {
    const binaryPath = path.join(INSTALL_DIR, BINARY_NAME);

    if (!fs.existsSync(binaryPath)) {
      return resolve({
        success: false,
        error: "Binary not found at " + binaryPath,
      });
    }

    const proc = spawn(binaryPath, ["--version"], {
      windowsHide: true,
      timeout: 10000,
    });

    let stdout = "";
    let stderr = "";

    proc.stdout.on("data", (data) => {
      stdout += data.toString();
    });

    proc.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    proc.on("close", (code) => {
      if (code === 0) {
        resolve({
          success: true,
          version: stdout.trim(),
          message: "Binary version verified",
        });
      } else {
        resolve({
          success: false,
          error: `Binary --version failed (exit ${code})`,
          details: { stdout, stderr },
        });
      }
    });

    proc.on("error", (err) => {
      resolve({
        success: false,
        error: `Failed to run binary: ${err.message}`,
      });
    });
  });
}

/**
 * Verify PATH by running "where gres-b2b" in a new process
 * This ensures the PATH update has taken effect
 */
async function pathWhere() {
  return new Promise((resolve) => {
    // Use cmd.exe /c to start a fresh environment that picks up PATH changes
    const proc = spawn("cmd.exe", ["/c", "where", "gres-b2b"], {
      windowsHide: true,
      timeout: 5000,
    });

    let stdout = "";
    let stderr = "";

    proc.stdout.on("data", (data) => {
      stdout += data.toString();
    });

    proc.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    proc.on("close", (code) => {
      const foundPath = stdout.trim();

      if (code === 0 && foundPath) {
        // Verify it's our installation
        const expectedPath = path.join(INSTALL_DIR, BINARY_NAME).toLowerCase();
        const foundLower = foundPath.toLowerCase();

        if (foundLower.includes(INSTALL_DIR.toLowerCase())) {
          resolve({
            success: true,
            path: foundPath,
            message: "PATH verified - gres-b2b is accessible",
          });
        } else {
          resolve({
            success: true,
            path: foundPath,
            message: "gres-b2b found (different location)",
            warning: `Found at ${foundPath}, expected ${INSTALL_DIR}`,
          });
        }
      } else {
        resolve({
          success: false,
          error: "gres-b2b not found in PATH",
          details: { stderr, code },
          hint: "Try opening a new terminal window",
        });
      }
    });

    proc.on("error", (err) => {
      resolve({
        success: false,
        error: `PATH verification failed: ${err.message}`,
      });
    });
  });
}

/**
 * Run gres-b2b doctor to check prerequisites
 */
async function doctor() {
  return new Promise((resolve) => {
    const binaryPath = path.join(INSTALL_DIR, BINARY_NAME);
    const configPath = path.join(CONFIG_DIR, CONFIG_NAME);

    if (!fs.existsSync(binaryPath)) {
      return resolve({
        success: false,
        error: "Binary not found",
      });
    }

    const args = fs.existsSync(configPath)
      ? ["--config", configPath, "doctor"]
      : ["doctor"];

    const proc = spawn(binaryPath, args, {
      windowsHide: true,
      timeout: 30000,
    });

    let stdout = "";
    let stderr = "";

    proc.stdout.on("data", (data) => {
      stdout += data.toString();
    });

    proc.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    proc.on("close", (code) => {
      if (code === 0) {
        resolve({
          success: true,
          output: stdout.trim(),
          message: "Doctor check passed",
        });
      } else {
        resolve({
          success: false,
          error: `Doctor check failed (exit ${code})`,
          output: stdout.trim(),
          details: { stderr },
        });
      }
    });

    proc.on("error", (err) => {
      resolve({
        success: false,
        error: `Failed to run doctor: ${err.message}`,
      });
    });
  });
}

module.exports = {
  binaryVersion,
  pathWhere,
  doctor,
};
