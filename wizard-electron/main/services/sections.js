/**
 * Wizard Sections Registry
 *
 * Each section has a stable ID that never changes.
 * IDs are used in code, UI, and logs for traceability.
 *
 * Section Contract:
 * - id: Stable identifier (WZ-XXX)
 * - name: Human-readable name
 * - description: What this section does
 * - run: Execute the section's main action
 * - test: Verify the section completed successfully (gate test)
 * - repair: Optional repair actions if test fails
 * - retryPolicy: How many retries allowed
 * - failMessage: User-friendly error message
 * - repairActions: Available repair options
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { INSTALL_DIR, BINARY_PATH } = require("./config");

/**
 * Section result structure
 * @typedef {Object} SectionResult
 * @property {boolean} pass - Whether the test passed
 * @property {string} code - Result code (OK, ERR_xxx)
 * @property {string} message - Human-readable message
 * @property {Object} evidence - Supporting data for logs
 */

/**
 * WZ-001: Preflight
 * Checks OS, architecture, permissions, writable directories
 */
const WZ001_PREFLIGHT = {
  id: "WZ-001",
  name: "Preflight",
  description: "Check system requirements and permissions",
  retryPolicy: { maxRetries: 0, canSkip: false },
  failMessage: "System requirements not met",
  repairActions: ["runAsAdmin", "checkPermissions"],

  run: async (ctx) => {
    const results = {
      os: process.platform,
      arch: process.arch,
      nodeVersion: process.version,
      electronVersion: process.versions.electron,
      isWindows: process.platform === "win32",
      isSupported: false,
      canWriteInstallDir: false,
      canWriteConfigDir: false,
    };

    // Check Windows version
    if (results.isWindows) {
      results.isSupported = true;
    }

    // Check writable directories
    try {
      fs.mkdirSync(INSTALL_DIR, { recursive: true });
      const testFile = path.join(INSTALL_DIR, ".preflight-test");
      fs.writeFileSync(testFile, "test");
      fs.unlinkSync(testFile);
      results.canWriteInstallDir = true;
    } catch (e) {
      results.installDirError = e.message;
    }

    const configDir = path.join(os.homedir(), "AppData", "Local", "gres-b2b");
    try {
      fs.mkdirSync(configDir, { recursive: true });
      const testFile = path.join(configDir, ".preflight-test");
      fs.writeFileSync(testFile, "test");
      fs.unlinkSync(testFile);
      results.canWriteConfigDir = true;
    } catch (e) {
      results.configDirError = e.message;
    }

    ctx.preflight = results;
    return results;
  },

  test: async (ctx) => {
    const p = ctx.preflight || {};

    if (!p.isWindows) {
      return {
        pass: false,
        code: "ERR_UNSUPPORTED_OS",
        message: "This wizard requires Windows",
        evidence: { os: p.os },
      };
    }

    if (!p.canWriteInstallDir) {
      return {
        pass: false,
        code: "ERR_NO_WRITE_INSTALL",
        message: `Cannot write to install directory: ${INSTALL_DIR}`,
        evidence: { error: p.installDirError },
      };
    }

    if (!p.canWriteConfigDir) {
      return {
        pass: false,
        code: "ERR_NO_WRITE_CONFIG",
        message: "Cannot write to config directory",
        evidence: { error: p.configDirError },
      };
    }

    return {
      pass: true,
      code: "OK",
      message: "System requirements met",
      evidence: p,
    };
  },
};

/**
 * WZ-002: Agent Detection
 * Protocol-first detection of AI agents by config file signatures
 */
const WZ002_AGENT_DETECTION = {
  id: "WZ-002",
  name: "Agent Detection",
  description: "Detect installed AI agents",
  retryPolicy: { maxRetries: 1, canSkip: false },
  failMessage: "Could not detect any AI agents",
  repairActions: ["manual", "refresh"],

  run: async (ctx, services) => {
    const result = await services.detect.detectAgents();
    ctx.agents = result.agents || [];
    ctx.selectedAgent = result.agents?.[0] || null;
    ctx.isManualFallback = result.isManualFallback || false;
    return result;
  },

  test: async (ctx) => {
    // Agent detection NEVER fails - always returns at least Manual fallback
    if (!ctx.agents || ctx.agents.length === 0) {
      return {
        pass: false,
        code: "ERR_NO_AGENTS",
        message: "No agents detected and no fallback available",
        evidence: {},
      };
    }

    return {
      pass: true,
      code: ctx.isManualFallback ? "OK_MANUAL" : "OK",
      message: ctx.isManualFallback
        ? "No agents detected, using manual configuration"
        : `Detected ${ctx.agents.length} agent(s)`,
      evidence: {
        agents: ctx.agents.map((a) => ({ id: a.id, name: a.name })),
        isManualFallback: ctx.isManualFallback,
      },
    };
  },
};

/**
 * WZ-003: Config Validation + Non-destructive Merge
 * Parse, backup, merge, and validate config files
 */
const WZ003_CONFIG_MERGE = {
  id: "WZ-003",
  name: "Config Merge",
  description: "Configure MCP connection for selected agent",
  retryPolicy: { maxRetries: 2, canSkip: false },
  failMessage: "Failed to configure MCP connection",
  repairActions: ["repair", "openConfig", "manual"],

  run: async (ctx, services) => {
    if (!ctx.selectedAgent) {
      return { success: false, error: "No agent selected" };
    }

    const result = await services.config.writeConfig(ctx.selectedAgent);
    ctx.configResult = result;
    return result;
  },

  test: async (ctx, services) => {
    if (!ctx.configResult) {
      return {
        pass: false,
        code: "ERR_NO_CONFIG_RESULT",
        message: "Config write was not attempted",
        evidence: {},
      };
    }

    if (!ctx.configResult.success) {
      return {
        pass: false,
        code: ctx.configResult.parseError ? "ERR_PARSE_ERROR" : "ERR_WRITE_FAILED",
        message: ctx.configResult.error || "Config write failed",
        evidence: ctx.configResult,
      };
    }

    // Verify config can be read back
    const readResult = services.config.readConfig(
      ctx.selectedAgent.configPath,
      ctx.selectedAgent.configType
    );

    if (!readResult.success) {
      return {
        pass: false,
        code: "ERR_CONFIG_CORRUPT",
        message: "Config file corrupted after write",
        evidence: readResult,
      };
    }

    // Verify backup exists
    const hasBackup = ctx.configResult.backup && fs.existsSync(ctx.configResult.backup);

    return {
      pass: true,
      code: "OK",
      message: "Config merged successfully",
      evidence: {
        configPath: ctx.selectedAgent.configPath,
        hasBackup,
        backupPath: ctx.configResult.backup,
        created: ctx.configResult.created,
      },
    };
  },
};

/**
 * WZ-004: Download Binary
 * Copy bundled binary or download from GitHub
 */
const WZ004_DOWNLOAD = {
  id: "WZ-004",
  name: "Install Binary",
  description: "Install gres-b2b CLI binary",
  retryPolicy: { maxRetries: 3, canSkip: false },
  failMessage: "Failed to install binary",
  repairActions: ["retry", "manualDownload"],

  run: async (ctx, services, opts = {}) => {
    const result = await services.download.downloadBinary({
      onProgress: opts.onProgress,
    });
    ctx.downloadResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.downloadResult) {
      return {
        pass: false,
        code: "ERR_NO_DOWNLOAD",
        message: "Download was not attempted",
        evidence: {},
      };
    }

    if (!ctx.downloadResult.success) {
      return {
        pass: false,
        code: "ERR_DOWNLOAD_FAILED",
        message: ctx.downloadResult.error || "Download failed",
        evidence: ctx.downloadResult,
      };
    }

    // Verify file exists and has size > 0
    if (!fs.existsSync(BINARY_PATH)) {
      return {
        pass: false,
        code: "ERR_BINARY_MISSING",
        message: "Binary file not found after download",
        evidence: { expectedPath: BINARY_PATH },
      };
    }

    const stats = fs.statSync(BINARY_PATH);
    if (stats.size === 0) {
      return {
        pass: false,
        code: "ERR_BINARY_EMPTY",
        message: "Binary file is empty",
        evidence: { size: stats.size },
      };
    }

    return {
      pass: true,
      code: "OK",
      message: `Binary installed (${(stats.size / 1024 / 1024).toFixed(2)} MB)`,
      evidence: {
        path: BINARY_PATH,
        size: stats.size,
        version: ctx.downloadResult.version,
      },
    };
  },
};

/**
 * WZ-005: Binary Proof
 * Run --version to verify binary executes correctly
 */
const WZ005_BINARY_PROOF = {
  id: "WZ-005",
  name: "Binary Proof",
  description: "Verify binary executes correctly",
  retryPolicy: { maxRetries: 2, canSkip: false },
  failMessage: "Binary verification failed",
  repairActions: ["unblock", "retry", "redownload"],

  run: async (ctx, services) => {
    const result = await services.verify.binaryVersion(BINARY_PATH);
    ctx.binaryProofResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.binaryProofResult) {
      return {
        pass: false,
        code: "ERR_NO_PROOF",
        message: "Binary proof was not attempted",
        evidence: {},
      };
    }

    if (!ctx.binaryProofResult.success) {
      return {
        pass: false,
        code: "ERR_BINARY_FAILED",
        message: ctx.binaryProofResult.error || "Binary --version failed",
        evidence: ctx.binaryProofResult,
      };
    }

    return {
      pass: true,
      code: "OK",
      message: `Binary verified: ${ctx.binaryProofResult.version}`,
      evidence: {
        version: ctx.binaryProofResult.version,
        path: BINARY_PATH,
      },
    };
  },
};

/**
 * WZ-006: MCP Selftest Gate
 * Run mcp selftest to verify MCP protocol works
 */
const WZ006_MCP_SELFTEST = {
  id: "WZ-006",
  name: "MCP Selftest",
  description: "Verify MCP protocol handshake",
  retryPolicy: { maxRetries: 2, canSkip: true },
  failMessage: "MCP selftest failed",
  repairActions: ["retry", "skip", "checkConfig"],

  run: async (ctx, services) => {
    const result = await services.mcp.selftest();
    ctx.mcpSelftestResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.mcpSelftestResult) {
      return {
        pass: false,
        code: "ERR_NO_SELFTEST",
        message: "MCP selftest was not attempted",
        evidence: {},
      };
    }

    if (!ctx.mcpSelftestResult.success) {
      return {
        pass: false,
        code: "ERR_MCP_FAILED",
        message: ctx.mcpSelftestResult.error || "MCP selftest failed",
        evidence: ctx.mcpSelftestResult,
      };
    }

    return {
      pass: true,
      code: "OK",
      message: "MCP handshake successful",
      evidence: {
        protocol: ctx.mcpSelftestResult.protocol,
        serverInfo: ctx.mcpSelftestResult.serverInfo,
      },
    };
  },
};

/**
 * WZ-007: Restart Enforcement
 * Check if selected agent is running and needs restart
 */
const WZ007_RESTART = {
  id: "WZ-007",
  name: "Restart Check",
  description: "Check if agent needs restart",
  retryPolicy: { maxRetries: 0, canSkip: true },
  failMessage: "Agent is still running",
  repairActions: ["waitForExit", "forceKill", "skip"],

  run: async (ctx, services) => {
    if (!ctx.selectedAgent) {
      return { running: false };
    }

    const result = await services.zombie.checkAgentRunning(ctx.selectedAgent.name);
    ctx.restartCheckResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.restartCheckResult) {
      return {
        pass: true,
        code: "OK_SKIPPED",
        message: "No agent selected, skipping restart check",
        evidence: {},
      };
    }

    if (ctx.restartCheckResult.running) {
      return {
        pass: false,
        code: "ERR_AGENT_RUNNING",
        message: `${ctx.selectedAgent.name} is still running. Please close it to apply changes.`,
        evidence: {
          agentName: ctx.selectedAgent.name,
          pids: ctx.restartCheckResult.pids,
        },
      };
    }

    return {
      pass: true,
      code: "OK",
      message: ctx.restartCheckResult.wasRunning
        ? "Agent closed successfully"
        : "Agent is not running",
      evidence: ctx.restartCheckResult,
    };
  },
};

/**
 * WZ-008: Scan Target Selection
 * Select local directory or Git URL for scanning
 */
const WZ008_SCAN_TARGET = {
  id: "WZ-008",
  name: "Scan Target",
  description: "Select project to scan",
  retryPolicy: { maxRetries: 0, canSkip: true },
  failMessage: "No scan target selected",
  repairActions: ["browse", "enterUrl", "skip"],

  run: async (ctx, opts = {}) => {
    // This is driven by UI - just store the selection
    ctx.scanTarget = opts.target || null;
    ctx.scanType = opts.type || null; // "local" or "git"
    return { target: ctx.scanTarget, type: ctx.scanType };
  },

  test: async (ctx) => {
    // Scan is optional - pass if skipped or if target is valid
    if (!ctx.scanTarget) {
      return {
        pass: true,
        code: "OK_SKIPPED",
        message: "Scan skipped",
        evidence: {},
      };
    }

    if (ctx.scanType === "local") {
      if (!fs.existsSync(ctx.scanTarget)) {
        return {
          pass: false,
          code: "ERR_TARGET_NOT_FOUND",
          message: "Scan target directory not found",
          evidence: { target: ctx.scanTarget },
        };
      }
    }

    return {
      pass: true,
      code: "OK",
      message: `Scan target selected: ${ctx.scanTarget}`,
      evidence: { target: ctx.scanTarget, type: ctx.scanType },
    };
  },
};

/**
 * WZ-009: Detached Scan Start
 * Start scan in background and open progress URL
 */
const WZ009_SCAN_START = {
  id: "WZ-009",
  name: "Start Scan",
  description: "Start governance scan",
  retryPolicy: { maxRetries: 1, canSkip: true },
  failMessage: "Failed to start scan",
  repairActions: ["retry", "skip"],

  run: async (ctx, services) => {
    if (!ctx.scanTarget) {
      return { skipped: true };
    }

    const result = await services.scan.startDetached({
      source: ctx.scanTarget,
      port: 8080,
    });
    ctx.scanStartResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.scanTarget) {
      return {
        pass: true,
        code: "OK_SKIPPED",
        message: "Scan skipped",
        evidence: {},
      };
    }

    if (!ctx.scanStartResult) {
      return {
        pass: false,
        code: "ERR_NO_SCAN",
        message: "Scan was not started",
        evidence: {},
      };
    }

    if (!ctx.scanStartResult.success) {
      return {
        pass: false,
        code: "ERR_SCAN_FAILED",
        message: ctx.scanStartResult.error || "Scan failed to start",
        evidence: ctx.scanStartResult,
      };
    }

    return {
      pass: true,
      code: "OK",
      message: "Scan started in background",
      evidence: {
        pid: ctx.scanStartResult.pid,
        url: ctx.scanStartResult.url,
      },
    };
  },
};

/**
 * WZ-010: Doctor Final Verification
 * Run full doctor check as final gate
 */
const WZ010_DOCTOR = {
  id: "WZ-010",
  name: "Final Verification",
  description: "Run final system check",
  retryPolicy: { maxRetries: 1, canSkip: true },
  failMessage: "Final verification found issues",
  repairActions: ["viewDetails", "skip"],

  run: async (ctx, services) => {
    const result = await services.verify.doctor(BINARY_PATH);
    ctx.doctorResult = result;
    return result;
  },

  test: async (ctx) => {
    if (!ctx.doctorResult) {
      return {
        pass: true,
        code: "OK_SKIPPED",
        message: "Doctor check skipped",
        evidence: {},
      };
    }

    if (!ctx.doctorResult.success) {
      return {
        pass: false,
        code: "ERR_DOCTOR_FAILED",
        message: ctx.doctorResult.error || "Doctor check failed",
        evidence: ctx.doctorResult,
      };
    }

    return {
      pass: true,
      code: "OK",
      message: "All checks passed",
      evidence: { output: ctx.doctorResult.output },
    };
  },
};

/**
 * All sections in order
 */
const SECTIONS = [
  WZ001_PREFLIGHT,
  WZ002_AGENT_DETECTION,
  WZ003_CONFIG_MERGE,
  WZ004_DOWNLOAD,
  WZ005_BINARY_PROOF,
  WZ006_MCP_SELFTEST,
  WZ007_RESTART,
  WZ008_SCAN_TARGET,
  WZ009_SCAN_START,
  WZ010_DOCTOR,
];

/**
 * Get section by ID
 */
function getSection(id) {
  return SECTIONS.find((s) => s.id === id);
}

/**
 * Get all section IDs
 */
function getSectionIds() {
  return SECTIONS.map((s) => s.id);
}

/**
 * Get section index
 */
function getSectionIndex(id) {
  return SECTIONS.findIndex((s) => s.id === id);
}

module.exports = {
  SECTIONS,
  getSection,
  getSectionIds,
  getSectionIndex,
  // Export individual sections for direct access
  WZ001_PREFLIGHT,
  WZ002_AGENT_DETECTION,
  WZ003_CONFIG_MERGE,
  WZ004_DOWNLOAD,
  WZ005_BINARY_PROOF,
  WZ006_MCP_SELFTEST,
  WZ007_RESTART,
  WZ008_SCAN_TARGET,
  WZ009_SCAN_START,
  WZ010_DOCTOR,
};
