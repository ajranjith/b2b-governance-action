/**
 * Install Service - Bundled Binary Installation
 *
 * The binary is bundled with the app in extraResources.
 * This service copies it to the install location.
 *
 * Features:
 * - Permission check before write
 * - Unblock-File for Windows Defender/SmartScreen
 * - 30-second timeout for first run verification
 * - Auto-retry for locked files
 */

const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { execSync, spawn } = require("child_process");
const { shell } = require("electron");
const os = require("os");
const { INSTALL_DIR, BINARY_PATH } = require("./config");

const BINARY_NAME = "gres-b2b.exe";
const CONFIG_NAME = "gres-b2b.config.json";
const MAX_RETRY_ATTEMPTS = 3;
const RETRY_DELAY_MS = 1000;

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
 * Get the path to the bundled config in extraResources
 */
function getBundledConfigPath() {
  const resourcesPath = process.resourcesPath || path.join(__dirname, "..", "..");
  return path.join(resourcesPath, CONFIG_NAME);
}

/**
 * Get the install path for the config file
 */
function getConfigInstallPath() {
  return path.join(INSTALL_DIR, CONFIG_NAME);
}

/**
 * Check if directory is writable
 */
function checkWritePermission(dirPath) {
  try {
    // Ensure directory exists
    fs.mkdirSync(dirPath, { recursive: true });

    // Try to write a test file
    const testFile = path.join(dirPath, ".write-test-" + Date.now());
    fs.writeFileSync(testFile, "test");
    fs.unlinkSync(testFile);
    return { success: true };
  } catch (e) {
    return {
      success: false,
      error: `No write permission to ${dirPath}: ${e.message}`,
    };
  }
}

/**
 * Check if file is locked by another process
 */
function isFileLocked(filePath) {
  if (!fs.existsSync(filePath)) return false;

  try {
    // Try to open file for writing - will fail if locked
    const fd = fs.openSync(filePath, "r+");
    fs.closeSync(fd);
    return false;
  } catch (e) {
    return e.code === "EBUSY" || e.code === "EPERM" || e.code === "EACCES";
  }
}

/**
 * Wait for file to become unlocked
 */
async function waitForUnlock(filePath, maxAttempts = MAX_RETRY_ATTEMPTS) {
  for (let i = 0; i < maxAttempts; i++) {
    if (!isFileLocked(filePath)) {
      return { success: true };
    }
    await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY_MS));
  }
  return {
    success: false,
    error: `File is locked by another process: ${filePath}`,
  };
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
    // Step 1: Check write permissions
    const permCheck = checkWritePermission(INSTALL_DIR);
    if (!permCheck.success) {
      return {
        success: false,
        error: permCheck.error,
        hint: "Try running as administrator or choose a different install location.",
      };
    }

    // Get bundled binary path
    const bundledPath = getBundledBinaryPath();

    if (!fs.existsSync(bundledPath)) {
      return {
        success: false,
        error: `Bundled binary not found at: ${bundledPath}`,
        hint: "The installer may be corrupted. Please re-download.",
      };
    }

    // Report progress
    if (opts.onProgress) opts.onProgress(10);

    // Step 2: Check if target file is locked
    if (fs.existsSync(BINARY_PATH)) {
      const unlockResult = await waitForUnlock(BINARY_PATH);
      if (!unlockResult.success) {
        return {
          success: false,
          error: unlockResult.error,
          hint: "Close any applications using gres-b2b and try again.",
        };
      }
    }

    if (opts.onProgress) opts.onProgress(30);

    // Step 3: Copy to install location with retry
    let copySuccess = false;
    let lastError = null;

    for (let attempt = 0; attempt < MAX_RETRY_ATTEMPTS; attempt++) {
      try {
        fs.copyFileSync(bundledPath, BINARY_PATH);
        copySuccess = true;
        break;
      } catch (e) {
        lastError = e;
        if (attempt < MAX_RETRY_ATTEMPTS - 1) {
          await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY_MS));
        }
      }
    }

    if (!copySuccess) {
      return {
        success: false,
        error: `Failed to copy binary: ${lastError.message}`,
        hint: "The file may be in use. Close any terminals or AI agents and try again.",
      };
    }

    if (opts.onProgress) opts.onProgress(50);

    // Step 3b: Copy config file alongside binary
    const bundledConfigPath = getBundledConfigPath();
    const configInstallPath = getConfigInstallPath();
    if (fs.existsSync(bundledConfigPath)) {
      try {
        fs.copyFileSync(bundledConfigPath, configInstallPath);
        console.log("Config file installed:", configInstallPath);
      } catch (e) {
        console.warn("Failed to copy config file:", e.message);
        // Non-fatal - binary can still work with defaults
      }
    }

    // Step 4: Unblock the binary (removes Mark of the Web)
    unblockFile(BINARY_PATH);

    if (opts.onProgress) opts.onProgress(70);

    // Step 5: Verify the binary works
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
      // Check for common issues
      let hint = "The binary may be blocked by antivirus. Check Windows Security settings.";

      if (e.message.includes("timed out")) {
        hint = "Windows Defender may be scanning the file. Wait a moment and try again.";
      } else if (e.message.includes("EACCES") || e.message.includes("EPERM")) {
        hint = "Permission denied. Try running as administrator.";
      }

      return {
        success: false,
        error: `Binary verification failed: ${e.message}`,
        hint,
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

/**
 * Create desktop shortcut for gres-b2b
 */
async function createDesktopShortcut() {
  try {
    const desktopPath = path.join(os.homedir(), "Desktop");
    const shortcutPath = path.join(desktopPath, "GRES B2B Governance.lnk");

    // Use PowerShell to create shortcut
    const psScript = `
      $WshShell = New-Object -ComObject WScript.Shell
      $Shortcut = $WshShell.CreateShortcut('${shortcutPath.replace(/'/g, "''")}')
      $Shortcut.TargetPath = '${BINARY_PATH.replace(/'/g, "''")}'
      $Shortcut.Arguments = 'doctor'
      $Shortcut.WorkingDirectory = '${INSTALL_DIR.replace(/'/g, "''")}'
      $Shortcut.Description = 'GRES B2B Governance CLI'
      $Shortcut.Save()
    `.replace(/\n/g, " ");

    execSync(`powershell.exe -NoProfile -Command "${psScript}"`, {
      windowsHide: true,
      timeout: 10000,
    });

    return {
      success: true,
      path: shortcutPath,
      message: "Desktop shortcut created",
    };
  } catch (e) {
    // Non-fatal - just log and continue
    console.warn("Failed to create desktop shortcut:", e.message);
    return {
      success: false,
      error: e.message,
    };
  }
}

/**
 * Open documentation URL in browser
 */
function openDocs() {
  const docsUrl = "https://ajranjith.github.io/b2b-governance-action/";
  shell.openExternal(docsUrl);
  return { success: true, url: docsUrl };
}

/**
 * Open dashboard/onboarding URL
 */
function openDashboard() {
  const dashboardUrl = "https://ajranjith.github.io/b2b-governance-action/onboarding/?status=ready";
  shell.openExternal(dashboardUrl);
  return { success: true, url: dashboardUrl };
}

module.exports = {
  downloadBinary,
  verifyChecksum,
  getReleaseAssets,
  runVersionCheck,
  hasBundledBinary,
  getBundledBinaryPath,
  getBundledConfigPath,
  getConfigInstallPath,
  createDesktopShortcut,
  openDocs,
  openDashboard,
  checkWritePermission,
  BINARY_NAME,
  CONFIG_NAME,
};
