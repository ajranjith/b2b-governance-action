/**
 * Config Management Service - Safe Non-Destructive Merges
 *
 * Rules:
 * 1. Always backup before editing
 * 2. Parse config - if fails, offer Repair/Overwrite or Open file
 * 3. Upsert only "GRES B2B Governance" / "gres_b2b" entry
 * 4. Never delete existing MCP connections
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { parseJSONSafe, parseTOML } = require("./detect");
const { runCLI } = require("./cli");

// Install location: %LOCALAPPDATA%\Programs\gres-b2b\
const INSTALL_DIR = path.join(
  process.env.LOCALAPPDATA || path.join(os.homedir(), "AppData", "Local"),
  "Programs",
  "gres-b2b"
);
const BINARY_NAME = "gres-b2b.exe";
const BINARY_PATH = path.join(INSTALL_DIR, BINARY_NAME);

/**
 * Create timestamped backup of config file
 */
function createBackup(configPath) {
  if (!fs.existsSync(configPath)) {
    return { success: true, skipped: true };
  }

  const dir = path.dirname(configPath);
  const ext = path.extname(configPath);
  const base = path.basename(configPath, ext);
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  const backupName = `${base}.backup-${timestamp}${ext}`;
  const backupPath = path.join(dir, backupName);

  try {
    fs.copyFileSync(configPath, backupPath);
    return { success: true, path: backupPath };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

/**
 * Read and parse config file
 */
function readConfig(configPath, configType = "json") {
  if (!fs.existsSync(configPath)) {
    return { success: true, exists: false, data: null };
  }

  try {
    const content = fs.readFileSync(configPath, "utf8");

    if (configType === "toml") {
      const data = parseTOML(content);
      return { success: true, exists: true, data, raw: content };
    } else {
      const result = parseJSONSafe(content);
      if (result.success) {
        return {
          success: true,
          exists: true,
          data: result.data,
          wasRepaired: result.wasRepaired,
          raw: content,
        };
      } else {
        return {
          success: false,
          exists: true,
          parseError: true,
          error: result.error,
          raw: content,
        };
      }
    }
  } catch (e) {
    return { success: false, error: e.message };
  }
}

/**
 * Check if GRES is already configured in the config
 */
function checkGresConfigured(configPath, mcpKey, gresKey, configType = "json") {
  const result = readConfig(configPath, configType);

  if (!result.success || !result.exists) {
    return { configured: false, error: result.error };
  }

  const mcpServers = getNestedValue(result.data, mcpKey);
  const configured = !!(mcpServers && mcpServers[gresKey]);

  return {
    configured,
    data: result.data,
    currentEntry: configured ? mcpServers[gresKey] : null,
  };
}

/**
 * Get nested value from object using dot notation
 */
function getNestedValue(obj, keyPath) {
  const keys = keyPath.split(".");
  let current = obj;

  for (const key of keys) {
    if (current === undefined || current === null) return undefined;
    current = current[key];
  }

  return current;
}

/**
 * Set nested value in object using dot notation
 */
function setNestedValue(obj, keyPath, value) {
  const keys = keyPath.split(".");
  let current = obj;

  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i];
    if (!current[key] || typeof current[key] !== "object") {
      current[key] = {};
    }
    current = current[key];
  }

  current[keys[keys.length - 1]] = value;
}

/**
 * Build the GRES server entry for MCP config (JSON)
 * Uses JSON.stringify internally to ensure correct escaping
 */
function buildGresEntry() {
  // Use absolute path - JSON.stringify handles escaping when written
  return {
    command: BINARY_PATH,
    args: ["mcp", "serve"],
  };
}

/**
 * Build TOML section for GRES (Codex-compatible format)
 * Uses [mcp_servers.gres_b2b] with enabled = true
 * Single quotes with double backslashes for proper TOML escaping
 */
function buildGresToml() {
  // Use single quotes in TOML to avoid escaping issues
  // Double backslashes for Windows paths
  const escapedPath = BINARY_PATH.replace(/\\/g, "\\\\");
  return `
[mcp_servers.gres_b2b]
enabled = true
command = '${escapedPath}'
args = ["mcp", "serve"]
`;
}

/**
 * Write config with safe merge (JSON)
 */
async function writeJsonConfig(agent) {
  const { configPath, mcpKey, gresKey } = agent;

  // Step 1: Create backup
  const backupResult = createBackup(configPath);
  if (!backupResult.success && !backupResult.skipped) {
    return { success: false, error: `Backup failed: ${backupResult.error}` };
  }

  // Step 2: Read existing config
  const readResult = readConfig(configPath, "json");

  if (readResult.parseError) {
    // Config exists but is corrupted - return error with repair option
    return {
      success: false,
      parseError: true,
      error: readResult.error,
      raw: readResult.raw,
      backup: backupResult.path,
      needsRepair: true,
    };
  }

  // Step 3: Merge config
  let data = readResult.exists ? { ...readResult.data } : {};

  // Navigate to MCP servers section, creating path if needed
  const keys = mcpKey.split(".");
  let current = data;

  for (let i = 0; i < keys.length; i++) {
    const key = keys[i];
    if (i === keys.length - 1) {
      // Last key - this is the servers object
      if (!current[key] || typeof current[key] !== "object") {
        current[key] = {};
      }
      // UPSERT only the GRES entry - preserve ALL other entries
      current[key][gresKey] = buildGresEntry();
    } else {
      if (!current[key] || typeof current[key] !== "object") {
        current[key] = {};
      }
      current = current[key];
    }
  }

  // Step 4: Ensure parent directory exists
  const dir = path.dirname(configPath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  // Step 5: Write config
  try {
    fs.writeFileSync(configPath, JSON.stringify(data, null, 2), "utf8");
    return {
      success: true,
      path: configPath,
      backup: backupResult.path,
      created: !readResult.exists,
    };
  } catch (e) {
    return { success: false, error: e.message, backup: backupResult.path };
  }
}

/**
 * Write config with safe merge (TOML)
 * Uses Codex-compatible format: [mcp_servers.gres_b2b] with enabled = true
 */
async function writeTomlConfig(agent) {
  const { configPath } = agent;

  // Step 1: Create backup
  const backupResult = createBackup(configPath);
  if (!backupResult.success && !backupResult.skipped) {
    return { success: false, error: `Backup failed: ${backupResult.error}` };
  }

  // Step 2: Read existing config
  let content = "";
  let exists = false;

  if (fs.existsSync(configPath)) {
    exists = true;
    content = fs.readFileSync(configPath, "utf8");
  }

  // Step 3: Check if GRES section already exists (check both old and new format)
  const gresSectionNew = "[mcp_servers.gres_b2b]";
  const gresSectionOld = "[mcp.servers.gres_b2b]";
  const hasGresNew = content.includes(gresSectionNew);
  const hasGresOld = content.includes(gresSectionOld);

  if (hasGresNew) {
    // Update existing new-format section
    const sectionRegex = /\[mcp_servers\.gres_b2b\][^\[]*(?=\[|$)/s;
    content = content.replace(sectionRegex, buildGresToml().trim() + "\n\n");
  } else if (hasGresOld) {
    // Remove old format and add new format
    const sectionRegex = /\[mcp\.servers\.gres_b2b\][^\[]*(?=\[|$)/s;
    content = content.replace(sectionRegex, "");
    content = content.trimEnd() + "\n" + buildGresToml();
  } else {
    // Append new section
    content = content.trimEnd() + "\n" + buildGresToml();
  }

  // Step 4: Ensure parent directory exists
  const dir = path.dirname(configPath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  // Step 5: Write config
  try {
    fs.writeFileSync(configPath, content, "utf8");
    return {
      success: true,
      path: configPath,
      backup: backupResult.path,
      created: !exists,
    };
  } catch (e) {
    return { success: false, error: e.message, backup: backupResult.path };
  }
}

/**
 * Main write config function - routes to JSON or TOML handler
 */
async function writeConfig(agent) {
  const args = ["connect-agent", "--client", agent.id || agent.name || "cursor", "--bin", BINARY_PATH];
  if (agent.configPath) {
    args.push("--config", agent.configPath);
  }
  const res = runCLI(args, process.cwd());
  if (!res.success) {
    return { success: false, error: res.stderr || res.error || "connect-agent failed" };
  }
  return { success: true, path: agent.configPath || "", created: false };
}

/**
/**
 * Repair corrupted config by creating fresh with GRES only
 */
async function repairConfig(agent) {
  const { configPath, mcpKey, gresKey, configType } = agent;

  // Create backup first
  const backupResult = createBackup(configPath);

  // Ensure parent directory exists
  const dir = path.dirname(configPath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  if (configType === "toml") {
    const content = buildGresToml();
    fs.writeFileSync(configPath, content, "utf8");
  } else {
    const data = {};
    setNestedValue(data, mcpKey, { [gresKey]: buildGresEntry() });
    fs.writeFileSync(configPath, JSON.stringify(data, null, 2), "utf8");
  }

  return {
    success: true,
    path: configPath,
    backup: backupResult.path,
    repaired: true,
  };
}

/**
 * Open config file in default editor
 */
function openConfig(configPath) {
  const { shell } = require("electron");
  shell.openPath(configPath);
  return { success: true };
}

/**
 * Get install directory path
 */
function getInstallDir() {
  return INSTALL_DIR;
}

/**
 * Get binary path
 */
function getBinaryPath() {
  return BINARY_PATH;
}

module.exports = {
  readConfig,
  writeConfig,
  repairConfig,
  openConfig,
  createBackup,
  checkGresConfigured,
  getInstallDir,
  getBinaryPath,
  INSTALL_DIR,
  BINARY_PATH,
};
