const { app, BrowserWindow, ipcMain } = require("electron");
const path = require("path");

// Import services
const mcp = require("./services/mcp");
const download = require("./services/download");
const pathService = require("./services/path");
const verify = require("./services/verify");
const scan = require("./services/scan");

let mainWindow;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 800,
    height: 650,
    resizable: false,
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWindow.loadFile(path.join(__dirname, "../renderer/index.html"));

  // Uncomment for debugging:
  // mainWindow.webContents.openDevTools();
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
// IPC Handlers - State Validation Loop
// ============================================================================

// Step 1: MCP Bridge Test
ipcMain.handle("mcp:testInitialize", async (_, opts) => {
  return mcp.testInitialize(opts);
});

ipcMain.handle("mcp:selftest", async () => {
  return mcp.selftest();
});

// Step 2: Binary Download & Verification
ipcMain.handle("install:downloadBinary", async (_, opts) => {
  return download.downloadBinary(opts);
});

ipcMain.handle("install:verifyChecksum", async (_, opts) => {
  return download.verifyChecksum(opts);
});

// Step 3: PATH Update & Verification
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

// Scan operations
ipcMain.handle("scan:start", async (evt, opts) => {
  return scan.start(evt.sender, opts);
});

// Config operations
ipcMain.handle("config:write", async (_, opts) => {
  return pathService.writeConfig(opts);
});

// Utility
ipcMain.handle("util:openUrl", async (_, url) => {
  const { shell } = require("electron");
  shell.openExternal(url);
  return { success: true };
});

ipcMain.handle("util:getInstallPath", async () => {
  return pathService.getInstallPath();
});
