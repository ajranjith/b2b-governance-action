const { contextBridge, ipcRenderer } = require("electron");

// Expose secure API to renderer via contextBridge
// This is the ONLY way the UI can communicate with the system
contextBridge.exposeInMainWorld("gres", {
  // ========================================================================
  // Phase 1: Environment Probing
  // ========================================================================
  detect: {
    agents: () => ipcRenderer.invoke("detect:agents"),
  },

  // ========================================================================
  // Phase 2: MCP Handshake
  // ========================================================================
  mcp: {
    selftest: () => ipcRenderer.invoke("mcp:selftest"),
    testInitialize: (opts) => ipcRenderer.invoke("mcp:testInitialize", opts),
  },

  // ========================================================================
  // Phase 3: Installation
  // ========================================================================
  install: {
    downloadBinary: (opts) => ipcRenderer.invoke("install:downloadBinary", opts),
    verifyChecksum: (opts) => ipcRenderer.invoke("install:verifyChecksum", opts),
    applyPath: () => ipcRenderer.invoke("install:applyPath"),

    // Download progress listener
    onProgress: (callback) => {
      ipcRenderer.on("download:progress", (_, percent) => callback(percent));
    },
    offProgress: () => {
      ipcRenderer.removeAllListeners("download:progress");
    },
  },

  // ========================================================================
  // Phase 4: Config Management (Non-Destructive)
  // ========================================================================
  config: {
    read: (opts) => ipcRenderer.invoke("config:read", opts),
    check: (opts) => ipcRenderer.invoke("config:check", opts),
    write: (opts) => ipcRenderer.invoke("config:write", opts),
    repair: (opts) => ipcRenderer.invoke("config:repair", opts),
  },

  // ========================================================================
  // Phase 5: Zombie Guard
  // ========================================================================
  zombie: {
    check: (agentName) => ipcRenderer.invoke("zombie:check", agentName),
    waitForExit: (opts) => ipcRenderer.invoke("zombie:waitForExit", opts),
    forceKill: (agentName) => ipcRenderer.invoke("zombie:forceKill", agentName),

    // Status update listener
    onStatus: (callback) => {
      ipcRenderer.on("zombie:status", (_, status) => callback(status));
    },
    offStatus: () => {
      ipcRenderer.removeAllListeners("zombie:status");
    },
  },

  // ========================================================================
  // Phase 6: Verification
  // ========================================================================
  verify: {
    binaryVersion: () => ipcRenderer.invoke("verify:binaryVersion"),
    pathWhere: () => ipcRenderer.invoke("verify:pathWhere"),
    doctor: () => ipcRenderer.invoke("verify:doctor"),
  },

  // ========================================================================
  // Phase 7: Scan
  // ========================================================================
  scan: {
    start: (opts) => ipcRenderer.invoke("scan:start", opts),
    checkPort: (port) => ipcRenderer.invoke("scan:checkPort", port),
    findPort: (startPort) => ipcRenderer.invoke("scan:findPort", startPort),

    // Scan event listener
    onEvent: (callback) => {
      ipcRenderer.on("scan:event", (_, event) => callback(event));
    },
    offEvent: () => {
      ipcRenderer.removeAllListeners("scan:event");
    },
  },

  // ========================================================================
  // Utilities
  // ========================================================================
  util: {
    openUrl: (url) => ipcRenderer.invoke("util:openUrl", url),
    openPath: (path) => ipcRenderer.invoke("util:openPath", path),
    getInstallPath: () => ipcRenderer.invoke("util:getInstallPath"),
    createShortcut: (opts) => ipcRenderer.invoke("util:createShortcut", opts),
  },

  // ========================================================================
  // System Checks
  // ========================================================================
  system: {
    checkDevMode: () => ipcRenderer.invoke("system:checkDevMode"),
    checkExecutable: (path) => ipcRenderer.invoke("system:checkExecutable", path),
  },
});
