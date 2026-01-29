/**
 * Update Service - Silent Sync with Atomic Swap
 *
 * Features:
 * - Version check against GitHub releases
 * - Atomic swap pattern (rename-to-delete) for locked files
 * - Automatic rollback if update fails verification
 * - Architecture-aware asset resolution
 * - Rate limiting (24-hour default) with electron-store
 * - 403 Forbidden handling with X-RateLimit-Reset
 * - Manual override option
 */

const fs = require("fs");
const path = require("path");
const https = require("https");
const { execSync, spawn } = require("child_process");
const os = require("os");
const { INSTALL_DIR, BINARY_PATH } = require("./config");

const GITHUB_OWNER = "ajranjith";
const GITHUB_REPO = "b2b-governance-action";
const BINARY_NAME = "gres-b2b.exe";

// Rate limiting constants
const RATE_LIMIT_INTERVAL = 24 * 60 * 60 * 1000; // 24 hours

// Store instance (loaded asynchronously)
let store = null;
let storePromise = null;

/**
 * Initialize electron-store with dynamic import (ESM compatibility)
 */
async function getStore() {
  if (store) return store;

  if (!storePromise) {
    storePromise = (async () => {
      const { default: Store } = await import("electron-store");
      store = new Store({
        name: "gres-b2b-update",
        defaults: {
          lastUpdateCheck: 0,
          rateLimitReset: 0,
          cachedVersion: null,
        },
      });
      return store;
    })();
  }

  return storePromise;
}

// Architecture patterns for matching release assets
const ARCH_PATTERNS = {
  x64: /(win|windows).*(x64|amd64|x86_64)/i,
  arm64: /(win|windows).*arm64/i,
};

/**
 * Get Windows architecture using environment variables (most reliable)
 */
function getWindowsArch() {
  const envArch = process.env.PROCESSOR_ARCHITECTURE;
  const is64Bit = process.env.PROCESSOR_ARCHITEW6432 || envArch === "AMD64";

  if (envArch === "ARM64") return "arm64";
  if (is64Bit) return "x64";
  return os.arch();
}

/**
 * Check if we should call GitHub API (rate limiting)
 * Returns { allowed, reason, nextCheckTime }
 */
async function shouldCheckForUpdate() {
  const s = await getStore();
  const now = Date.now();
  const lastCheck = s.get("lastUpdateCheck", 0);
  const rateLimitReset = s.get("rateLimitReset", 0);

  // Check if we're blocked by GitHub rate limit
  if (rateLimitReset > now) {
    const resetDate = new Date(rateLimitReset);
    return {
      allowed: false,
      reason: `GitHub API limit reached. Retry after ${resetDate.toLocaleTimeString()}`,
      nextCheckTime: rateLimitReset,
      rateLimited: true,
    };
  }

  // Check if 24 hours have passed since last check
  const timeSinceLastCheck = now - lastCheck;
  if (lastCheck > 0 && timeSinceLastCheck < RATE_LIMIT_INTERVAL) {
    const nextCheck = lastCheck + RATE_LIMIT_INTERVAL;
    const hoursRemaining = Math.round((nextCheck - now) / 3600000);
    return {
      allowed: false,
      reason: `Next auto-check in ${hoursRemaining} hour(s)`,
      nextCheckTime: nextCheck,
      rateLimited: false,
    };
  }

  return { allowed: true };
}

/**
 * Record successful API check
 */
async function recordApiCheck() {
  const s = await getStore();
  s.set("lastUpdateCheck", Date.now());
}

/**
 * Handle GitHub 403 rate limit response
 */
async function handleRateLimit(headers) {
  const s = await getStore();
  const resetTime = headers["x-ratelimit-reset"];
  if (resetTime) {
    const resetMs = parseInt(resetTime, 10) * 1000;
    s.set("rateLimitReset", resetMs);
    return new Date(resetMs);
  }
  // Default: wait 1 hour
  const fallbackReset = Date.now() + 60 * 60 * 1000;
  s.set("rateLimitReset", fallbackReset);
  return new Date(fallbackReset);
}

/**
 * Clear rate limit (for manual override)
 */
async function clearRateLimit() {
  const s = await getStore();
  s.set("rateLimitReset", 0);
  s.set("lastUpdateCheck", 0);
}

/**
 * Get current installed version
 */
async function getCurrentVersion() {
  if (!fs.existsSync(BINARY_PATH)) {
    return { success: false, error: "Binary not installed" };
  }

  return new Promise((resolve) => {
    const proc = spawn(BINARY_PATH, ["--version"], { windowsHide: true });
    let stdout = "";
    let stderr = "";

    proc.stdout.on("data", (d) => (stdout += d.toString()));
    proc.stderr.on("data", (d) => (stderr += d.toString()));

    proc.on("close", (code) => {
      if (code === 0) {
        const match = stdout.match(/v?(\d+\.\d+\.\d+)/);
        resolve({
          success: true,
          version: match ? match[1] : stdout.trim(),
          raw: stdout.trim(),
        });
      } else {
        resolve({ success: false, error: stderr || stdout || `Exit code ${code}` });
      }
    });

    proc.on("error", (e) => {
      resolve({ success: false, error: e.message });
    });

    setTimeout(() => {
      proc.kill();
      resolve({ success: false, error: "Version check timed out" });
    }, 10000);
  });
}

/**
 * Fetch latest release info from GitHub
 */
async function getLatestRelease() {
  return new Promise((resolve, reject) => {
    const url = `https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest`;

    https
      .get(url, { headers: { "User-Agent": "gres-b2b-updater" } }, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          // Follow redirect
          https
            .get(res.headers.location, { headers: { "User-Agent": "gres-b2b-updater" } }, (res2) => {
              handleResponse(res2, resolve, reject);
            })
            .on("error", reject);
          return;
        }
        handleResponse(res, resolve, reject);
      })
      .on("error", reject);
  });

  async function handleResponse(res, resolve, reject) {
    // Handle rate limit (403 Forbidden)
    if (res.statusCode === 403) {
      const resetTime = await handleRateLimit(res.headers);
      reject(new Error(`GitHub API rate limit exceeded. Retry after ${resetTime.toLocaleTimeString()}`));
      return;
    }

    if (res.statusCode !== 200) {
      reject(new Error(`GitHub API returned ${res.statusCode}`));
      return;
    }

    let data = "";
    res.on("data", (chunk) => (data += chunk));
    res.on("end", () => {
      try {
        resolve(JSON.parse(data));
      } catch (e) {
        reject(new Error("Invalid JSON from GitHub"));
      }
    });
  }
}

/**
 * Find compatible binary asset in release
 */
function findCompatibleAsset(release) {
  const arch = getWindowsArch();
  const pattern = ARCH_PATTERNS[arch] || ARCH_PATTERNS.x64;

  // Look for matching exe
  const asset = release.assets.find((a) => {
    const name = a.name.toLowerCase();
    return (
      name.endsWith(".exe") &&
      !name.includes("setup") &&
      !name.includes("sha256") &&
      pattern.test(a.name)
    );
  });

  if (!asset) {
    // Fallback: any exe that's not the setup
    return release.assets.find((a) => {
      const name = a.name.toLowerCase();
      return name.endsWith(".exe") && !name.includes("setup") && !name.includes("sha256");
    });
  }

  return asset;
}

/**
 * Check if update is available (with rate limiting)
 * Use forceCheck: true to bypass rate limiting (manual override)
 */
async function checkForUpdate(opts = {}) {
  const { forceCheck = false } = opts;

  try {
    const s = await getStore();

    // Check rate limiting unless forced
    if (!forceCheck) {
      const rateCheck = await shouldCheckForUpdate();
      if (!rateCheck.allowed) {
        // Return cached version info if available
        const cached = s.get("cachedVersion");
        return {
          success: true,
          skipped: true,
          reason: rateCheck.reason,
          nextCheckTime: rateCheck.nextCheckTime,
          rateLimited: rateCheck.rateLimited,
          cached: cached || null,
        };
      }
    } else {
      // Manual override - clear any rate limit blocks
      await clearRateLimit();
    }

    // Get current version
    const current = await getCurrentVersion();
    if (!current.success) {
      return { updateAvailable: false, error: current.error };
    }

    // Get latest release
    const release = await getLatestRelease();
    const latestVersion = release.tag_name.replace(/^v/, "");

    // Record successful API check
    await recordApiCheck();

    // Compare versions
    const updateAvailable = latestVersion !== current.version;

    // Find compatible asset
    const asset = findCompatibleAsset(release);

    // Cache the result
    const result = {
      success: true,
      updateAvailable,
      currentVersion: current.version,
      latestVersion,
      downloadUrl: asset?.browser_download_url,
      assetName: asset?.name,
      releaseNotes: release.body,
      checkedAt: Date.now(),
    };

    s.set("cachedVersion", result);

    return result;
  } catch (err) {
    return { success: false, error: err.message };
  }
}

/**
 * Force check for update (manual override, bypasses rate limiting)
 */
async function forceCheckForUpdate() {
  return checkForUpdate({ forceCheck: true });
}

/**
 * Get rate limit status
 */
async function getRateLimitStatus() {
  const s = await getStore();
  const rateCheck = await shouldCheckForUpdate();
  return {
    allowed: rateCheck.allowed,
    reason: rateCheck.reason || "Ready to check",
    nextCheckTime: rateCheck.nextCheckTime,
    rateLimited: rateCheck.rateLimited || false,
    lastCheck: s.get("lastUpdateCheck", 0),
  };
}

/**
 * Download file to path
 */
function downloadFile(url, destPath, onProgress) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destPath);

    const download = (downloadUrl) => {
      https
        .get(downloadUrl, { headers: { "User-Agent": "gres-b2b-updater" } }, (res) => {
          if (res.statusCode === 301 || res.statusCode === 302) {
            return download(res.headers.location);
          }

          if (res.statusCode !== 200) {
            file.close();
            fs.unlinkSync(destPath);
            reject(new Error(`Download failed: HTTP ${res.statusCode}`));
            return;
          }

          const totalSize = parseInt(res.headers["content-length"], 10) || 0;
          let downloaded = 0;

          res.on("data", (chunk) => {
            downloaded += chunk.length;
            if (totalSize > 0 && onProgress) {
              onProgress(Math.round((downloaded / totalSize) * 100));
            }
          });

          res.pipe(file);

          file.on("finish", () => {
            file.close();
            resolve();
          });
        })
        .on("error", (err) => {
          file.close();
          fs.unlinkSync(destPath);
          reject(err);
        });
    };

    download(url);
  });
}

/**
 * Unblock file using PowerShell
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
 * Verify binary works by running --version
 */
async function verifyBinary(binaryPath) {
  return new Promise((resolve) => {
    const proc = spawn(binaryPath, ["--version"], { windowsHide: true });
    let stdout = "";

    proc.stdout.on("data", (d) => (stdout += d.toString()));

    proc.on("close", (code) => {
      resolve({ success: code === 0, output: stdout.trim() });
    });

    proc.on("error", (e) => {
      resolve({ success: false, error: e.message });
    });

    setTimeout(() => {
      proc.kill();
      resolve({ success: false, error: "Verification timed out" });
    }, 30000);
  });
}

/**
 * Perform atomic swap update
 *
 * 1. Download new binary to .new file
 * 2. Rename current to .old (Windows allows renaming running files)
 * 3. Rename .new to current
 * 4. Verify new binary works
 * 5. If verification fails, rollback
 * 6. Schedule .old for deletion
 */
async function performUpdate(opts = {}) {
  const { onProgress, onStatus } = opts;

  const newPath = BINARY_PATH + ".new";
  const oldPath = BINARY_PATH + ".old";

  try {
    // Step 1: Check for update
    if (onStatus) onStatus("Checking for updates...");
    const updateInfo = await checkForUpdate();

    if (!updateInfo.success) {
      return { success: false, error: updateInfo.error };
    }

    if (!updateInfo.updateAvailable) {
      return { success: true, message: "Already up to date", version: updateInfo.currentVersion };
    }

    if (!updateInfo.downloadUrl) {
      return { success: false, error: "No compatible binary found in release" };
    }

    // Step 2: Download new binary
    if (onStatus) onStatus(`Downloading v${updateInfo.latestVersion}...`);

    await downloadFile(updateInfo.downloadUrl, newPath, onProgress);

    // Step 3: Unblock the new binary
    if (onStatus) onStatus("Preparing update...");
    unblockFile(newPath);

    // Step 4: Verify new binary before swap
    const preVerify = await verifyBinary(newPath);
    if (!preVerify.success) {
      fs.unlinkSync(newPath);
      return { success: false, error: `New binary verification failed: ${preVerify.error}` };
    }

    // Step 5: Atomic swap
    if (onStatus) onStatus("Applying update...");

    // Clean up any leftover .old file
    if (fs.existsSync(oldPath)) {
      try {
        fs.unlinkSync(oldPath);
      } catch (e) {
        // May be locked, will be cleaned up later
      }
    }

    // Rename current to .old (works even if running)
    if (fs.existsSync(BINARY_PATH)) {
      try {
        fs.renameSync(BINARY_PATH, oldPath);
      } catch (e) {
        // If rename fails, try copy+delete
        fs.copyFileSync(BINARY_PATH, oldPath);
      }
    }

    // Rename .new to current
    fs.renameSync(newPath, BINARY_PATH);

    // Step 6: Post-update verification
    if (onStatus) onStatus("Verifying update...");
    const postVerify = await verifyBinary(BINARY_PATH);

    if (!postVerify.success) {
      // Rollback
      if (onStatus) onStatus("Rolling back...");
      if (fs.existsSync(oldPath)) {
        fs.renameSync(oldPath, BINARY_PATH);
      }
      return { success: false, error: `Update verification failed, rolled back: ${postVerify.error}` };
    }

    // Step 7: Schedule old file deletion
    scheduleCleanup(oldPath);

    return {
      success: true,
      message: `Updated to v${updateInfo.latestVersion}`,
      previousVersion: updateInfo.currentVersion,
      newVersion: updateInfo.latestVersion,
    };
  } catch (err) {
    // Cleanup on error
    if (fs.existsSync(newPath)) {
      try {
        fs.unlinkSync(newPath);
      } catch (e) {
        // Ignore
      }
    }
    return { success: false, error: err.message };
  }
}

/**
 * Schedule file for cleanup (delete after delay or on next run)
 */
function scheduleCleanup(filePath) {
  // Try to delete after 30 seconds
  setTimeout(() => {
    try {
      if (fs.existsSync(filePath)) {
        fs.unlinkSync(filePath);
      }
    } catch (e) {
      // File still in use, will be cleaned up on next run or reboot
      console.log("Scheduled cleanup failed, will retry later:", filePath);
    }
  }, 30000);
}

/**
 * Clean up any leftover .old or .new files
 */
function cleanupTempFiles() {
  const files = [BINARY_PATH + ".old", BINARY_PATH + ".new"];

  for (const file of files) {
    try {
      if (fs.existsSync(file)) {
        fs.unlinkSync(file);
      }
    } catch (e) {
      // Ignore
    }
  }
}

module.exports = {
  checkForUpdate,
  forceCheckForUpdate,
  performUpdate,
  getCurrentVersion,
  getLatestRelease,
  getWindowsArch,
  cleanupTempFiles,
  shouldCheckForUpdate,
  getRateLimitStatus,
  clearRateLimit,
};
