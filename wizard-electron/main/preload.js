const { contextBridge, ipcRenderer } = require("electron");

// Expose safe API to renderer
contextBridge.exposeInMainWorld("gres", {
  // MCP operations
  mcp: {
    testInitialize: (opts) => ipcRenderer.invoke("mcp:testInitialize", opts),
    selftest: () => ipcRenderer.invoke("mcp:selftest"),
  },

  // Installation operations
  install: {
    downloadBinary: (opts) => ipcRenderer.invoke("install:downloadBinary", opts),
    verifyChecksum: (opts) => ipcRenderer.invoke("install:verifyChecksum", opts),
    applyPath: () => ipcRenderer.invoke("install:applyPath"),
  },

  // Verification operations
  verify: {
    binaryVersion: () => ipcRenderer.invoke("verify:binaryVersion"),
    pathWhere: () => ipcRenderer.invoke("verify:pathWhere"),
    doctor: () => ipcRenderer.invoke("verify:doctor"),
  },

  // Scan operations
  scan: {
    start: (opts) => ipcRenderer.invoke("scan:start", opts),
    onEvent: (callback) => {
      ipcRenderer.on("scan:event", (_, event) => callback(event));
    },
    offEvent: () => {
      ipcRenderer.removeAllListeners("scan:event");
    },
  },

  // Config operations
  config: {
    write: (opts) => ipcRenderer.invoke("config:write", opts),
  },

  // Utility operations
  util: {
    openUrl: (url) => ipcRenderer.invoke("util:openUrl", url),
    getInstallPath: () => ipcRenderer.invoke("util:getInstallPath"),
  },
});
