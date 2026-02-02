const { app, BrowserWindow, ipcMain, shell } = require("electron");
const path = require("path");
const net = require("net");
const fs = require("fs");

// Import services
const mcp = require("./services/mcp");
const download = require("./services/download");
const pathService = require("./services/path");
const verify = require("./services/verify");
const cli = require("./services/cli");
const scan = require("./services/scan");
const detect = require("./services/detect");
const config = require("./services/config");
const zombie = require("./services/zombie");
const update = require("./services/update");

// Import wizard state machine
const wizardState = require("./services/wizard-state");
const wizardLogger = require("./services/wizard-logger");
const { SECTIONS } = require("./services/sections");

let mainWindow;

// Services object for state machine
const services = {
  mcp,
  download,
  verify,
  scan,
  detect,
  config,
  zombie,
  update,
  pathService,
};

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 520,
    height: 720,
    resizable: false,
    autoHideMenuBar: true,
    backgroundColor: "#0f1115",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWindow.loadFile(path.join(__dirname, "../renderer/index.html"));

  // Enable DevTools in development
  if (process.env.NODE_ENV === "development") {
    mainWindow.webContents.openDevTools();
  }
}

app.whenReady().then(() => {
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});

// ============================================================================
// Phase 1: Agent Detection (Protocol-First)
// ============================================================================

// Detect agents by signature config files
ipcMain.handle("detect:agents", async () => {
  try {
    return await detect.detectAgents();
  } catch (err) {
    return { success: false, error: err.message, agents: [] };
  }
});

// ============================================================================
// Phase 2: Config Management (Non-Destructive)
// ============================================================================

// Read config file
ipcMain.handle("config:read", async (_, opts) => {
  try {
    return config.readConfig(opts.configPath, opts.configType);
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Check if GRES is configured
ipcMain.handle("config:check", async (_, opts) => {
  try {
    return config.checkGresConfigured(
      opts.configPath,
      opts.mcpKey,
      opts.gresKey,
      opts.configType
    );
  } catch (err) {
    return { configured: false, error: err.message };
  }
});

// Write config with safe merge
ipcMain.handle("config:write", async (_, agent) => {
  try {
    return await config.writeConfig(agent);
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Repair corrupted config
ipcMain.handle("config:repair", async (_, agent) => {
  try {
    return await config.repairConfig(agent);
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Open config in editor
ipcMain.handle("config:open", async (_, configPath) => {
  try {
    return config.openConfig(configPath);
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// ============================================================================
// Phase 3: Binary Installation
// ============================================================================

// Download binary from GitHub Releases
ipcMain.handle("install:downloadBinary", async (event, opts) => {
  return download.downloadBinary({
    ...opts,
    installDir: config.INSTALL_DIR,
    onProgress: (percent) => {
      mainWindow.webContents.send("download:progress", percent);
    },
  });
});

// Get release assets for manual selection
ipcMain.handle("install:getReleaseAssets", async () => {
  return download.getReleaseAssets();
});

// Verify checksum
ipcMain.handle("install:verifyChecksum", async (_, opts) => {
  return download.verifyChecksum({
    ...opts,
    binaryPath: config.BINARY_PATH,
  });
});

// Apply PATH
ipcMain.handle("install:applyPath", async () => {
  return pathService.applyPath(config.INSTALL_DIR);
});

// Get install paths
ipcMain.handle("install:getPaths", async () => {
  return {
    installDir: config.INSTALL_DIR,
    binaryPath: config.BINARY_PATH,
  };
});

// Create desktop shortcut
ipcMain.handle("install:createShortcut", async () => {
  return download.createDesktopShortcut();
});

// Open documentation
ipcMain.handle("install:openDocs", async () => {
  return download.openDocs();
});

// Open dashboard/onboarding
ipcMain.handle("install:openDashboard", async () => {
  return download.openDashboard();
});

// ============================================================================
// Phase 4: Zombie Guard
// ============================================================================

// Check if agent is running
ipcMain.handle("zombie:check", async (_, agentName) => {
  return zombie.checkAgentRunning(agentName);
});

// Check all agents for running processes
ipcMain.handle("zombie:checkAll", async (_, agents) => {
  return zombie.checkAllAgentsRunning(agents);
});

// Poll for agent to exit
ipcMain.handle("zombie:waitForExit", async (event, opts) => {
  const { agentName, timeout = 60000, pollInterval = 3000 } = opts;
  return zombie.waitForAgentExit(agentName, {
    timeout,
    pollInterval,
    onCheck: (status) => {
      mainWindow.webContents.send("zombie:status", status);
    },
  });
});

// Force kill agent
ipcMain.handle("zombie:forceKill", async (_, agentName) => {
  return zombie.forceKillAgent(agentName);
});

// ============================================================================
// Phase 5: Verification (MCP Handshake)
// ============================================================================

// MCP selftest with real handshake
ipcMain.handle("mcp:selftest", async () => {
  return mcp.selftest();
});

// MCP test initialize
ipcMain.handle("mcp:testInitialize", async (_, opts) => {
  return mcp.testInitialize(opts);
});

// Binary version check
ipcMain.handle("verify:binaryVersion", async () => {
  return verify.binaryVersion(config.BINARY_PATH);
});

// PATH verification
ipcMain.handle("verify:pathWhere", async () => {
  return verify.pathWhere();
});

// Full doctor check
ipcMain.handle("verify:doctor", async () => {
  return verify.doctor(config.BINARY_PATH);
});


// ============================================================================
// CLI Bridge
// ============================================================================

ipcMain.handle("cli:run", async (_, args, cwd) => {
  return cli.runCLI(args || [], cwd || process.cwd());
});

ipcMain.handle("cli:readJSON", async (_, filePath) => {
  try {
    const data = fs.readFileSync(filePath, "utf8");
    return { success: true, data: JSON.parse(data) };
  } catch (e) {
    return { success: false, error: e.message };
  }
});

// ============================================================================
// Phase 6: Scan
// ============================================================================

// Start scan with streaming
ipcMain.handle("scan:start", async (evt, opts) => {
  return scan.start(evt.sender, opts);
});

// Start detached scan
ipcMain.handle("scan:startDetached", async (_, opts) => {
  return scan.startDetached(opts);
});

// Check if port is available
ipcMain.handle("scan:checkPort", async (_, port) => {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve({ available: false }));
    server.once("listening", () => {
      server.close();
      resolve({ available: true });
    });
    server.listen(port, "127.0.0.1");
  });
});

// Find available port
ipcMain.handle("scan:findPort", async (_, startPort = 8080) => {
  for (let port = startPort; port < startPort + 100; port++) {
    const available = await new Promise((resolve) => {
      const server = net.createServer();
      server.once("error", () => resolve(false));
      server.once("listening", () => {
        server.close();
        resolve(true);
      });
      server.listen(port, "127.0.0.1");
    });
    if (available) {
      return { port, success: true };
    }
  }
  return { success: false, error: "No available port found" };
});

// ============================================================================
// Utilities
// ============================================================================

ipcMain.handle("util:openUrl", async (_, url) => {
  shell.openExternal(url);
  return { success: true };
});

ipcMain.handle("util:openPath", async (_, filePath) => {
  shell.showItemInFolder(filePath);
  return { success: true };
});

ipcMain.handle("util:writeJSON", async (_, filePath, payload) => {
  try {
    const dir = path.dirname(filePath);
    fs.mkdirSync(dir, { recursive: true });
    const tmpPath = `${filePath}.tmp.${process.pid}`;
    const content = JSON.stringify(payload, null, 2);
    const fd = fs.openSync(tmpPath, "w");
    fs.writeFileSync(fd, content, "utf8");
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    fs.renameSync(tmpPath, filePath);
    return { success: true };
  } catch (err) {
    return { success: false, error: err.message };
  }
});

ipcMain.handle("util:getInstallPath", async () => {
  return {
    installDir: config.INSTALL_DIR,
    binaryPath: config.BINARY_PATH,
  };
});

// Create desktop shortcut
ipcMain.handle("util:createShortcut", async (_, opts) => {
  const { name, target, args = [] } = opts;
  try {
    const desktopPath = path.join(require("os").homedir(), "Desktop");
    const shortcutPath = path.join(desktopPath, `${name}.lnk`);

    // Create a Windows shortcut using PowerShell
    const psCommand = `
      $WshShell = New-Object -ComObject WScript.Shell
      $Shortcut = $WshShell.CreateShortcut('${shortcutPath.replace(/'/g, "''")}')
      $Shortcut.TargetPath = '${target.replace(/'/g, "''")}'
      $Shortcut.Arguments = '${args.join(" ").replace(/'/g, "''")}'
      $Shortcut.WorkingDirectory = '${path.dirname(target).replace(/'/g, "''")}'
      $Shortcut.Save()
    `;

    const { execSync } = require("child_process");
    execSync(`powershell -Command "${psCommand}"`, { windowsHide: true });

    return { success: true, path: shortcutPath };
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// ============================================================================
// System Checks
// ============================================================================

// Check Windows Developer Mode
ipcMain.handle("system:checkDevMode", async () => {
  try {
    const { execSync } = require("child_process");
    const result = execSync(
      'reg query "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock" /v AllowDevelopmentWithoutDevLicense',
      { windowsHide: true, encoding: "utf8" }
    );
    return { enabled: result.includes("0x1") };
  } catch (e) {
    return { enabled: false };
  }
});

// Check if file is executable
ipcMain.handle("system:checkExecutable", async (_, filePath) => {
  try {
    fs.accessSync(filePath, fs.constants.X_OK);
    return { executable: true };
  } catch (e) {
    return { executable: false, error: e.message };
  }
});

// ============================================================================
// Auto-Update (Silent Sync)
// ============================================================================

// Check for updates (respects rate limiting)
ipcMain.handle("update:check", async () => {
  try {
    return await update.checkForUpdate();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Force check for updates (bypasses rate limiting - manual override)
ipcMain.handle("update:forceCheck", async () => {
  try {
    return await update.forceCheckForUpdate();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get rate limit status
ipcMain.handle("update:rateLimitStatus", async () => {
  try {
    return update.getRateLimitStatus();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Clear rate limit (for troubleshooting)
ipcMain.handle("update:clearRateLimit", async () => {
  try {
    update.clearRateLimit();
    return { success: true };
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get current installed version
ipcMain.handle("update:currentVersion", async () => {
  try {
    return await update.getCurrentVersion();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Perform update with atomic swap
ipcMain.handle("update:perform", async () => {
  try {
    return await update.performUpdate({
      onProgress: (percent) => {
        mainWindow.webContents.send("update:progress", percent);
      },
      onStatus: (status) => {
        mainWindow.webContents.send("update:status", status);
      },
    });
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get system architecture
ipcMain.handle("update:arch", async () => {
  return { arch: update.getWindowsArch() };
});

// Cleanup temp files from previous updates
ipcMain.handle("update:cleanup", async () => {
  try {
    update.cleanupTempFiles();
    return { success: true };
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// ============================================================================
// Wizard State Machine (ID-based Sections with Gate Tests)
// ============================================================================

// Initialize wizard state machine
ipcMain.handle("wizard:initialize", async () => {
  try {
    return wizardState.initialize(services);
  } catch (err) {
    wizardLogger.logError("INIT", err);
    return { success: false, error: err.message };
  }
});

// Get current wizard status
ipcMain.handle("wizard:status", async () => {
  try {
    return wizardState.getStatus();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Run preflight check (WZ-001) - must pass before wizard proceeds
ipcMain.handle("wizard:preflight", async () => {
  try {
    const result = await wizardState.runPreflight();
    return result;
  } catch (err) {
    wizardLogger.logError("WZ-001", err);
    return { pass: false, code: "ERR_EXCEPTION", message: err.message };
  }
});

// Run current section
ipcMain.handle("wizard:runSection", async (_, opts = {}) => {
  try {
    const result = await wizardState.runCurrentSection({
      ...opts,
      onProgress: (percent) => {
        mainWindow.webContents.send("wizard:progress", percent);
      },
    });
    mainWindow.webContents.send("wizard:sectionResult", result);
    return result;
  } catch (err) {
    wizardLogger.logError("SECTION", err);
    return { pass: false, code: "ERR_EXCEPTION", message: err.message };
  }
});

// Run a specific section by ID
ipcMain.handle("wizard:runSectionById", async (_, sectionId, opts = {}) => {
  try {
    const result = await wizardState.runSection(sectionId, {
      ...opts,
      onProgress: (percent) => {
        mainWindow.webContents.send("wizard:progress", percent);
      },
    });
    mainWindow.webContents.send("wizard:sectionResult", result);
    return result;
  } catch (err) {
    wizardLogger.logError(sectionId, err);
    return { pass: false, code: "ERR_EXCEPTION", message: err.message };
  }
});

// Advance to next section (only if current passed)
ipcMain.handle("wizard:advance", async () => {
  try {
    return wizardState.advance();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Skip current section (only if allowed)
ipcMain.handle("wizard:skip", async () => {
  try {
    return wizardState.skip();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get wizard summary
ipcMain.handle("wizard:summary", async () => {
  try {
    return wizardState.getSummary();
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get/set context value
ipcMain.handle("wizard:getContext", async (_, key) => {
  return wizardState.getContext(key);
});

ipcMain.handle("wizard:setContext", async (_, key, value) => {
  wizardState.setContext(key, value);
  return { success: true };
});

// Reset to a specific section (for repair/retry)
ipcMain.handle("wizard:resetToSection", async (_, sectionId) => {
  try {
    return wizardState.resetToSection(sectionId);
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// Get all sections info
ipcMain.handle("wizard:sections", async () => {
  return SECTIONS.map((s) => ({
    id: s.id,
    name: s.name,
    description: s.description,
    retryPolicy: s.retryPolicy,
    repairActions: s.repairActions,
    failMessage: s.failMessage,
  }));
});

// Get section result
ipcMain.handle("wizard:getSectionResult", async (_, sectionId) => {
  return wizardState.getSectionResult(sectionId);
});

// Get log file path
ipcMain.handle("wizard:logPath", async () => {
  return { path: wizardLogger.getLogPath(), sessionId: wizardLogger.getSessionId() };
});

// Read recent logs
ipcMain.handle("wizard:logs", async (_, count = 50) => {
  return wizardLogger.readRecentLogs(count);
});
