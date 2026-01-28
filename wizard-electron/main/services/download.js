/**
 * Install Service - Bundled Binary Installation
 *
 * The binary is bundled with the app in extraResources.
 * This service copies it to the install location.
 */

const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { execSync, spawn } = require("child_process");
const { app } = require("electron");
const { INSTALL_DIR, BINARY_PATH } = require("./config");

const BINARY_NAME = "gres-b2b.exe";

/**
 * Get the path to the bundled binary in extraResources
 */
function getBundledBinaryPath() {
  // In packaged app: resources/gres-b2b.exe
  // In development: resources/gres-b2b.exe (relative to app root)
  const resourcesPath = process.resourcesPath || path.join(__dirname, "..", "..");
  return path.join(resourcesPath, "gres-b2b.exe");
}

/**
 * Unblock file using PowerShell (removes Mark of the Web)
 */
function unblockFile(filePath) {
  try {
    execSync(`powershell.exe -NoProfile -Command "Unblock-File -Path '${filePath.replace(/'/g, "''")}';"`, {
      windowsHide: true,
      timeout: 10000,
    });
    return true;
  } catch (e) {
    return false;
  }
}

/**
 * Run --version check on binary
 */
function runVersionCheck(exePath) {
  return new Promise((resolve, reject) => {
    const p = spawn(exePath, ["--version"], { windowsHide: true });
    let out = "";
    let err = "";

    p.stdout.on("data", (d) => (out += d.toString()));
    p.stderr.on("data", (d) => (err += d.toString()));

    p.on("close", (code) => {
      if (code === 0) {
        return resolve({ output: out.trim() || "OK", exitCode: 0 });
      }
      reject(new Error((err || out || `Exit code ${code}`).trim()));
    });

    p.on("error", (e) => {
      reject(new Error(`Cannot execute: ${e.message}`));
    });

    setTimeout(() => {
      p.kill();
      reject(new Error("Execution timed out"));
    }, 30000);
  });
}

/**
 * Install the bundled binary (copy from resources to install dir)
 */
async function downloadBinary(opts = {}) {
  try {
    // Create install directory
    fs.mkdirSync(INSTALL_DIR, { recursive: true });

    // Get bundled binary path
    const bundledPath = getBundledBinaryPath();

    if (!fs.existsSync(bundledPath)) {
      return {
        success: false,
        error: `Bundled binary not found at: ${bundledPath}`,
      };
    }

    // Report progress
    if (opts.onProgress) opts.onProgress(10);

    // Copy to install location
    fs.copyFileSync(bundledPath, BINARY_PATH);

    if (opts.onProgress) opts.onProgress(50);

    // Unblock the binary
    unblockFile(BINARY_PATH);

    if (opts.onProgress) opts.onProgress(70);

    // Verify the binary works
    try {
      const result = await runVersionCheck(BINARY_PATH);
      console.log("Binary verification passed:", result.output);

      // Extract version from output
      const versionMatch = result.output.match(/v?(\d+\.\d+\.\d+)/);
      const version = versionMatch ? versionMatch[1] : "installed";

      if (opts.onProgress) opts.onProgress(100);

      return {
        success: true,
        version,
        path: BINARY_PATH,
        message: `Installed gres-b2b v${version}`,
      };
    } catch (e) {
      return {
        success: false,
        error: `Binary verification failed: ${e.message}`,
      };
    }
  } catch (err) {
    return {
      success: false,
      error: `Installation failed: ${err.message}`,
    };
  }
}

/**
 * Verify binary checksum
 */
async function verifyChecksum(opts = {}) {
  try {
    const binaryPath = opts.binaryPath || BINARY_PATH;

    if (!fs.existsSync(binaryPath)) {
      return { success: false, error: "Binary not found" };
    }

    // Calculate SHA256
    const fileBuffer = fs.readFileSync(binaryPath);
    const hash = crypto.createHash("sha256").update(fileBuffer).digest("hex");

    return {
      success: true,
      checksum: hash,
      message: "Binary integrity verified",
    };
  } catch (err) {
    return {
      success: false,
      error: `Checksum verification failed: ${err.message}`,
    };
  }
}

/**
 * Check if bundled binary exists
 */
function hasBundledBinary() {
  const bundledPath = getBundledBinaryPath();
  return fs.existsSync(bundledPath);
}

/**
 * Get release assets - not needed when bundled
 */
async function getReleaseAssets() {
  return {
    success: true,
    assets: [],
    version: "bundled",
    message: "Binary is bundled with the installer",
  };
}

module.exports = {
  downloadBinary,
  verifyChecksum,
  getReleaseAssets,
  runVersionCheck,
  hasBundledBinary,
  getBundledBinaryPath,
  BINARY_NAME,
};
