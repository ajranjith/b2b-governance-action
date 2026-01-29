/**
 * Wizard Logger - Structured NDJSON Logging
 *
 * Writes all wizard events to:
 * %LOCALAPPDATA%\gres-b2b\wizard\logs\wizard.ndjson
 *
 * Every event includes:
 * - timestamp
 * - sectionId
 * - stage (RUNNING, PASSED, FAILED, SKIPPED)
 * - result.pass
 * - result.message
 * - evidence
 */

const fs = require("fs");
const path = require("path");
const os = require("os");

const LOG_DIR = path.join(os.homedir(), "AppData", "Local", "gres-b2b", "wizard", "logs");
const LOG_FILE = path.join(LOG_DIR, "wizard.ndjson");

// Session ID for this wizard run
const SESSION_ID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

/**
 * Ensure log directory exists
 */
function ensureLogDir() {
  try {
    fs.mkdirSync(LOG_DIR, { recursive: true });
    return true;
  } catch (e) {
    console.error("Failed to create log directory:", e.message);
    return false;
  }
}

/**
 * Write a log entry
 * @param {Object} entry - Log entry
 */
function log(entry) {
  if (!ensureLogDir()) return;

  const logEntry = {
    timestamp: new Date().toISOString(),
    sessionId: SESSION_ID,
    ...entry,
  };

  try {
    fs.appendFileSync(LOG_FILE, JSON.stringify(logEntry) + "\n", "utf8");
  } catch (e) {
    console.error("Failed to write log:", e.message);
  }

  // Also log to console for debugging
  console.log(`[${entry.sectionId || "WIZARD"}] ${entry.stage}: ${entry.message || ""}`);
}

/**
 * Log section start
 */
function logSectionStart(sectionId, sectionName) {
  log({
    sectionId,
    sectionName,
    stage: "RUNNING",
    message: `Starting ${sectionName}`,
  });
}

/**
 * Log section result
 */
function logSectionResult(sectionId, sectionName, result) {
  log({
    sectionId,
    sectionName,
    stage: result.pass ? "PASSED" : "FAILED",
    pass: result.pass,
    code: result.code,
    message: result.message,
    evidence: result.evidence,
  });
}

/**
 * Log section skip
 */
function logSectionSkip(sectionId, sectionName, reason) {
  log({
    sectionId,
    sectionName,
    stage: "SKIPPED",
    pass: true,
    message: reason || "Section skipped by user",
  });
}

/**
 * Log wizard start
 */
function logWizardStart() {
  log({
    stage: "WIZARD_START",
    message: "Wizard started",
    platform: process.platform,
    arch: process.arch,
    nodeVersion: process.version,
    electronVersion: process.versions.electron,
  });
}

/**
 * Log wizard complete
 */
function logWizardComplete(success, summary) {
  log({
    stage: success ? "WIZARD_COMPLETE" : "WIZARD_FAILED",
    message: success ? "Wizard completed successfully" : "Wizard failed",
    summary,
  });
}

/**
 * Log preflight result
 */
function logPreflightResult(result) {
  log({
    sectionId: "WZ-001",
    sectionName: "Preflight",
    stage: result.pass ? "PREFLIGHT_PASSED" : "PREFLIGHT_FAILED",
    pass: result.pass,
    code: result.code,
    message: result.message,
    evidence: result.evidence,
  });
}

/**
 * Log error
 */
function logError(sectionId, error) {
  log({
    sectionId,
    stage: "ERROR",
    error: error.message || String(error),
    stack: error.stack,
  });
}

/**
 * Get log file path
 */
function getLogPath() {
  return LOG_FILE;
}

/**
 * Get session ID
 */
function getSessionId() {
  return SESSION_ID;
}

/**
 * Read recent logs (last N entries)
 */
function readRecentLogs(count = 50) {
  try {
    if (!fs.existsSync(LOG_FILE)) {
      return [];
    }

    const content = fs.readFileSync(LOG_FILE, "utf8");
    const lines = content.trim().split("\n").filter(Boolean);
    const recent = lines.slice(-count);

    return recent.map((line) => {
      try {
        return JSON.parse(line);
      } catch (e) {
        return { raw: line, parseError: true };
      }
    });
  } catch (e) {
    return [];
  }
}

/**
 * Clear old logs (keep last N sessions)
 */
function cleanupOldLogs(keepSessions = 10) {
  try {
    if (!fs.existsSync(LOG_FILE)) return;

    const content = fs.readFileSync(LOG_FILE, "utf8");
    const lines = content.trim().split("\n").filter(Boolean);

    // Group by session
    const sessions = new Map();
    for (const line of lines) {
      try {
        const entry = JSON.parse(line);
        if (!sessions.has(entry.sessionId)) {
          sessions.set(entry.sessionId, []);
        }
        sessions.get(entry.sessionId).push(line);
      } catch (e) {
        // Skip malformed lines
      }
    }

    // Keep only recent sessions
    const sessionIds = Array.from(sessions.keys());
    const keepIds = sessionIds.slice(-keepSessions);

    const keepLines = [];
    for (const id of keepIds) {
      keepLines.push(...sessions.get(id));
    }

    fs.writeFileSync(LOG_FILE, keepLines.join("\n") + "\n", "utf8");
  } catch (e) {
    console.error("Failed to cleanup logs:", e.message);
  }
}

module.exports = {
  log,
  logSectionStart,
  logSectionResult,
  logSectionSkip,
  logWizardStart,
  logWizardComplete,
  logPreflightResult,
  logError,
  getLogPath,
  getSessionId,
  readRecentLogs,
  cleanupOldLogs,
  LOG_FILE,
  LOG_DIR,
};
