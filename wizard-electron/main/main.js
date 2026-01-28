const { app, BrowserWindow, ipcMain, shell } = require("electron");
const path = require("path");
const net = require("net");

// Import services
const mcp = require("./services/mcp");
const download = require("./services/download");
const pathService = require("./services/path");
const verify = require("./services/verify");
const scan = require("./services/scan");
const detect = require("./services/detect");
const config = require("./services/config");
const zombie = require("./services/zombie");

let mainWindow;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 520,
    height: 680,
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
// Phase 1: Environment Probe (Pre-flight)
// ============================================================================

// Agent Detection (Registry + Disk fallback)
ipcMain.handle("detect:agents", async () => {
  try {
    return await detect.detectAgents();
  } catch (err) {
    return { success: false, error: err.message, agents: [] };
  }
});

// Config Validation
ipcMain.handle("config:read", async (_, opts) => {
  return config.readConfig(opts.configPath);
});

ipcMain.handle("config:check", async (_, opts) => {
  return config.checkGresConfigured(opts.configPath, opts.mcpKey);
});

// ============================================================================
// Phase 2: Active Handshake
// ============================================================================

// MCP Selftest (the deterministic gatekeeper)
ipcMain.handle("mcp:selftest", async () => {
  return mcp.selftest();
});

ipcMain.handle("mcp:testInitialize", async (_, opts) => {
  return mcp.testInitialize(opts);
});

// Binary Download with progress
ipcMain.handle("install:downloadBinary", async (event, opts) => {
  return download.downloadBinary({
    ...opts,
    onProgress: (percent) => {
      mainWindow.webContents.send("download:progress", percent);
    },
  });
});

ipcMain.handle("install:verifyChecksum", async (_, opts) => {
  return download.verifyChecksum(opts);
});

// ============================================================================
// Phase 3: Zombie Guard
// ============================================================================

// Check if agent is running
ipcMain.handle("zombie:check", async (_, agentName) => {
  return zombie.checkAgentRunning(agentName);
});

// Poll for agent to exit
ipcMain.handle("zombie:waitForExit", async (event, opts) => {
  const { agentName, timeout = 60000 } = opts;
  return zombie.waitForAgentExit(agentName, {
    timeout,
    onCheck: (status) => {
      mainWindow.webContents.send("zombie:status", status);
    },
  });
});

// Force kill agent (requires user consent)
ipcMain.handle("zombie:forceKill", async (_, agentName) => {
  return zombie.forceKillAgent(agentName);
});

// ============================================================================
// Phase 4: Config Write (Non-Destructive)
// ============================================================================

// Write config with merge (preserves existing MCP connections)
ipcMain.handle("config:write", async (_, opts) => {
  return config.writeConfig(opts);
});

// Repair corrupted config
ipcMain.handle("config:repair", async (_, opts) => {
  return config.repairConfig(opts.configPath, opts.mcpKey);
});

// ============================================================================
// Phase 5: PATH & Verification
// ============================================================================

ipcMain.handle("install:applyPath", async () => {
  return pathService.applyPath();
});

ipcMain.handle("verify:pathWhere", async () => {
  return verify.pathWhere();
});

ipcMain.handle("verify:binaryVersion", async () => {
  return verify.binaryVersion();
});

ipcMain.handle("verify:doctor", async () => {
  return verify.doctor();
});

// ============================================================================
// Phase 6: Scan Handoff
// ============================================================================

ipcMain.handle("scan:start", async (evt, opts) => {
  return scan.start(evt.sender, opts);
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

ipcMain.handle("util:getInstallPath", async () => {
  return pathService.getInstallPath();
});

// Create desktop shortcut
ipcMain.handle("util:createShortcut", async (_, opts) => {
  const { name, url } = opts;
  try {
    const desktopPath = path.join(require("os").homedir(), "Desktop");
    const shortcutPath = path.join(desktopPath, `${name}.url`);

    const content = `[InternetShortcut]\nURL=${url}\nIconIndex=0\n`;
    require("fs").writeFileSync(shortcutPath, content);

    return { success: true, path: shortcutPath };
  } catch (err) {
    return { success: false, error: err.message };
  }
});

// ============================================================================
// System Checks
// ============================================================================

// Check Windows Developer Mode (for symlinks)
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

// Check execution policy
ipcMain.handle("system:checkExecutable", async (_, filePath) => {
  const fs = require("fs");
  try {
    fs.accessSync(filePath, fs.constants.X_OK);
    return { executable: true };
  } catch (e) {
    return { executable: false, error: e.message };
  }
});
