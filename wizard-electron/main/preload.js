const { contextBridge, ipcRenderer } = require("electron");

// Expose secure API to renderer via contextBridge
// This is the ONLY way the UI can communicate with the system
contextBridge.exposeInMainWorld("gres", {
  // ========================================================================
  // Phase 1: Agent Detection (Protocol-First)
  // ========================================================================
  detect: {
    agents: () => ipcRenderer.invoke("detect:agents"),
  },

  // ========================================================================
  // Phase 2: Config Management (Non-Destructive)
  // ========================================================================
  config: {
    read: (opts) => ipcRenderer.invoke("config:read", opts),
    check: (opts) => ipcRenderer.invoke("config:check", opts),
    write: (agent) => ipcRenderer.invoke("config:write", agent),
    repair: (agent) => ipcRenderer.invoke("config:repair", agent),
    open: (configPath) => ipcRenderer.invoke("config:open", configPath),
  },

  // ========================================================================
  // Phase 3: Binary Installation
  // ========================================================================
  install: {
    downloadBinary: (opts) => ipcRenderer.invoke("install:downloadBinary", opts),
    verifyChecksum: (opts) => ipcRenderer.invoke("install:verifyChecksum", opts),
    applyPath: () => ipcRenderer.invoke("install:applyPath"),
    getPaths: () => ipcRenderer.invoke("install:getPaths"),
    getReleaseAssets: () => ipcRenderer.invoke("install:getReleaseAssets"),
    createShortcut: () => ipcRenderer.invoke("install:createShortcut"),
    openDocs: () => ipcRenderer.invoke("install:openDocs"),
    openDashboard: () => ipcRenderer.invoke("install:openDashboard"),

    // Download progress listener
    onProgress: (callback) => {
      ipcRenderer.on("download:progress", (_, percent) => callback(percent));
    },
    offProgress: () => {
      ipcRenderer.removeAllListeners("download:progress");
    },
  },

  // ========================================================================
  // Phase 4: Zombie Guard
  // ========================================================================
  zombie: {
    check: (agentName) => ipcRenderer.invoke("zombie:check", agentName),
    checkAll: (agents) => ipcRenderer.invoke("zombie:checkAll", agents),
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
  // Phase 5: Verification (MCP Handshake)
  // ========================================================================
  mcp: {
    selftest: () => ipcRenderer.invoke("mcp:selftest"),
    testInitialize: (opts) => ipcRenderer.invoke("mcp:testInitialize", opts),
  },

  verify: {
    binaryVersion: () => ipcRenderer.invoke("verify:binaryVersion"),
    pathWhere: () => ipcRenderer.invoke("verify:pathWhere"),
    doctor: () => ipcRenderer.invoke("verify:doctor"),
  },

  // ========================================================================
  // Phase 6: Scan
  // ========================================================================
  scan: {
    start: (opts) => ipcRenderer.invoke("scan:start", opts),
    startDetached: (opts) => ipcRenderer.invoke("scan:startDetached", opts),
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
