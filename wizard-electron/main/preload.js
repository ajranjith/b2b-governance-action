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

  // ========================================================================
  // Auto-Update (Silent Sync)
  // ========================================================================
  update: {
    // Check for updates (respects rate limiting)
    check: () => ipcRenderer.invoke("update:check"),

    // Force check (manual override, bypasses rate limiting)
    forceCheck: () => ipcRenderer.invoke("update:forceCheck"),

    // Get current installed version
    currentVersion: () => ipcRenderer.invoke("update:currentVersion"),

    // Perform the update
    perform: () => ipcRenderer.invoke("update:perform"),

    // Get system architecture
    arch: () => ipcRenderer.invoke("update:arch"),

    // Cleanup temp files
    cleanup: () => ipcRenderer.invoke("update:cleanup"),

    // Get rate limit status
    rateLimitStatus: () => ipcRenderer.invoke("update:rateLimitStatus"),

    // Clear rate limit (for troubleshooting)
    clearRateLimit: () => ipcRenderer.invoke("update:clearRateLimit"),

    // Progress listener
    onProgress: (callback) => {
      ipcRenderer.on("update:progress", (_, percent) => callback(percent));
    },
    offProgress: () => {
      ipcRenderer.removeAllListeners("update:progress");
    },

    // Status listener
    onStatus: (callback) => {
      ipcRenderer.on("update:status", (_, status) => callback(status));
    },
    offStatus: () => {
      ipcRenderer.removeAllListeners("update:status");
    },
  },

  // ========================================================================
  // Wizard State Machine (ID-based Sections with Gate Tests)
  // ========================================================================
  wizard: {
    // Initialize wizard state machine
    initialize: () => ipcRenderer.invoke("wizard:initialize"),

    // Get current wizard status
    status: () => ipcRenderer.invoke("wizard:status"),

    // Run preflight check (WZ-001) - must pass before wizard proceeds
    preflight: () => ipcRenderer.invoke("wizard:preflight"),

    // Run current section
    runSection: (opts) => ipcRenderer.invoke("wizard:runSection", opts),

    // Run a specific section by ID
    runSectionById: (sectionId, opts) => ipcRenderer.invoke("wizard:runSectionById", sectionId, opts),

    // Advance to next section (only if current passed)
    advance: () => ipcRenderer.invoke("wizard:advance"),

    // Skip current section (only if allowed)
    skip: () => ipcRenderer.invoke("wizard:skip"),

    // Get wizard summary
    summary: () => ipcRenderer.invoke("wizard:summary"),

    // Get/set context value
    getContext: (key) => ipcRenderer.invoke("wizard:getContext", key),
    setContext: (key, value) => ipcRenderer.invoke("wizard:setContext", key, value),

    // Reset to a specific section (for repair/retry)
    resetToSection: (sectionId) => ipcRenderer.invoke("wizard:resetToSection", sectionId),

    // Get all sections info
    sections: () => ipcRenderer.invoke("wizard:sections"),

    // Get section result
    getSectionResult: (sectionId) => ipcRenderer.invoke("wizard:getSectionResult", sectionId),

    // Get log file path
    logPath: () => ipcRenderer.invoke("wizard:logPath"),

    // Read recent logs
    logs: (count) => ipcRenderer.invoke("wizard:logs", count),

    // Progress listener
    onProgress: (callback) => {
      ipcRenderer.on("wizard:progress", (_, percent) => callback(percent));
    },
    offProgress: () => {
      ipcRenderer.removeAllListeners("wizard:progress");
    },

    // Section result listener
    onSectionResult: (callback) => {
      ipcRenderer.on("wizard:sectionResult", (_, result) => callback(result));
    },
    offSectionResult: () => {
      ipcRenderer.removeAllListeners("wizard:sectionResult");
    },
  },
});
