/**
 * Config Service - Non-Destructive MCP Config Management
 *
 * Critical Requirements:
 * - NEVER delete or overwrite existing MCP connections
 * - Only upsert the "GRES B2B Governance" entry
 * - Preserve all other keys, servers, and settings
 * - Create timestamped backup before any write
 * - Use absolute path for binary to avoid PATH issues
 */

const fs = require("fs");
const path = require("path");
const os = require("os");

const INSTALL_DIR = path.join(os.homedir(), "AppData", "Local", "Programs", "gres-b2b");
const BINARY_PATH = path.join(INSTALL_DIR, "gres-b2b.exe");
const GRES_SERVER_NAME = "gres-b2b";

/**
 * Attempt to repair broken JSON using common fixes
 */
function repairJson(content) {
  let fixed = content;

  // Remove trailing commas before } or ]
  fixed = fixed.replace(/,(\s*[}\]])/g, "$1");

  // Remove BOM
  fixed = fixed.replace(/^\uFEFF/, "");

  // Try to fix unquoted keys (simple cases)
  fixed = fixed.replace(/([{,]\s*)(\w+)(\s*:)/g, '$1"$2"$3');

  // Remove comments (// and /* */)
  fixed = fixed.replace(/\/\/.*$/gm, "");
  fixed = fixed.replace(/\/\*[\s\S]*?\*\//g, "");

  return fixed;
}

/**
 * Parse JSON with repair attempt
 */
function parseJsonSafe(content) {
  try {
    return { success: true, data: JSON.parse(content) };
  } catch (firstError) {
    // Try to repair
    try {
      const repaired = repairJson(content);
      return { success: true, data: JSON.parse(repaired), repaired: true };
    } catch (secondError) {
      return {
        success: false,
        error: firstError.message,
        position: firstError.message.match(/position (\d+)/)?.[1],
      };
    }
  }
}

/**
 * Create timestamped backup of config file
 */
function createBackup(configPath) {
  if (!fs.existsSync(configPath)) {
    return null;
  }

  const timestamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
  const backupPath = `${configPath}.bak-${timestamp}`;

  fs.copyFileSync(configPath, backupPath);
  return backupPath;
}

/**
 * Build the GRES MCP server entry
 */
function buildGresServerEntry() {
  return {
    command: BINARY_PATH.replace(/\\/g, "\\\\"), // Escape backslashes for JSON
    args: ["mcp", "serve"],
  };
}

/**
 * Non-destructive merge for JSON configs
 * Only upserts the GRES entry, preserves everything else
 */
function mergeJsonConfig(existingData, mcpKey) {
  const data = JSON.parse(JSON.stringify(existingData)); // Deep clone

  // Handle nested key like "mcp.servers"
  const keys = mcpKey.split(".");
  let current = data;

  // Navigate to parent of final key, creating objects as needed
  for (let i = 0; i < keys.length - 1; i++) {
    if (!current[keys[i]]) {
      current[keys[i]] = {};
    }
    current = current[keys[i]];
  }

  const finalKey = keys[keys.length - 1];

  // Create the servers object if it doesn't exist
  if (!current[finalKey]) {
    current[finalKey] = {};
  }

  // Upsert only the GRES entry - preserve all others
  current[finalKey][GRES_SERVER_NAME] = buildGresServerEntry();

  return data;
}

/**
 * Read and validate agent config
 */
async function readConfig(configPath) {
  if (!fs.existsSync(configPath)) {
    return {
      success: true,
      exists: false,
      data: null,
    };
  }

  const content = fs.readFileSync(configPath, "utf8");
  const parseResult = parseJsonSafe(content);

  return {
    success: parseResult.success,
    exists: true,
    data: parseResult.data,
    repaired: parseResult.repaired,
    error: parseResult.error,
    rawContent: content,
  };
}

/**
 * Write config with non-destructive merge
 * This is the main entry point for config updates
 */
async function writeConfig(opts) {
  const { configPath, mcpKey, forceOverwrite = false } = opts;

  try {
    // Ensure config directory exists
    const configDir = path.dirname(configPath);
    fs.mkdirSync(configDir, { recursive: true });

    // Read existing config
    const existing = await readConfig(configPath);

    let finalData;

    if (!existing.exists) {
      // No existing config - create new with just GRES entry
      finalData = {};
      const keys = mcpKey.split(".");
      let current = finalData;
      for (let i = 0; i < keys.length - 1; i++) {
        current[keys[i]] = {};
        current = current[keys[i]];
      }
      current[keys[keys.length - 1]] = {
        [GRES_SERVER_NAME]: buildGresServerEntry(),
      };
    } else if (!existing.success && !forceOverwrite) {
      // Config exists but is invalid - don't write without explicit permission
      return {
        success: false,
        error: "Config file is corrupted",
        parseError: existing.error,
        needsRepair: true,
      };
    } else if (!existing.success && forceOverwrite) {
      // Force overwrite with clean config
      finalData = {};
      const keys = mcpKey.split(".");
      let current = finalData;
      for (let i = 0; i < keys.length - 1; i++) {
        current[keys[i]] = {};
        current = current[keys[i]];
      }
      current[keys[keys.length - 1]] = {
        [GRES_SERVER_NAME]: buildGresServerEntry(),
      };
    } else {
      // Valid existing config - perform non-destructive merge
      finalData = mergeJsonConfig(existing.data, mcpKey);
    }

    // Create backup before writing
    const backupPath = createBackup(configPath);

    // Write the merged config
    const jsonContent = JSON.stringify(finalData, null, 2);
    fs.writeFileSync(configPath, jsonContent, "utf8");

    // Count existing MCP servers (excluding GRES)
    let existingServerCount = 0;
    try {
      const keys = mcpKey.split(".");
      let servers = finalData;
      for (const key of keys) {
        servers = servers[key];
      }
      existingServerCount = Object.keys(servers).filter((k) => k !== GRES_SERVER_NAME).length;
    } catch (e) {
      existingServerCount = 0;
    }

    return {
      success: true,
      configPath,
      backupPath,
      preservedServers: existingServerCount,
      message: existingServerCount > 0
        ? `Config updated. ${existingServerCount} existing MCP server(s) preserved.`
        : "Config created with GRES B2B entry.",
    };
  } catch (err) {
    return {
      success: false,
      error: `Failed to write config: ${err.message}`,
    };
  }
}

/**
 * Repair a corrupted config file
 */
async function repairConfig(configPath, mcpKey) {
  const backup = createBackup(configPath);

  // Read raw content
  const content = fs.readFileSync(configPath, "utf8");

  // Try to repair
  const repaired = repairJson(content);

  try {
    const data = JSON.parse(repaired);

    // Merge GRES entry
    const finalData = mergeJsonConfig(data, mcpKey);

    fs.writeFileSync(configPath, JSON.stringify(finalData, null, 2), "utf8");

    return {
      success: true,
      backupPath: backup,
      message: "Config repaired successfully",
    };
  } catch (e) {
    return {
      success: false,
      error: "Config is too corrupted to auto-repair",
      suggestion: "Please manually fix the JSON syntax or delete the file to start fresh",
    };
  }
}

/**
 * Check if GRES is already configured in agent
 */
async function checkGresConfigured(configPath, mcpKey) {
  const existing = await readConfig(configPath);

  if (!existing.success || !existing.exists) {
    return { configured: false };
  }

  try {
    const keys = mcpKey.split(".");
    let servers = existing.data;
    for (const key of keys) {
      servers = servers[key];
    }

    if (servers && servers[GRES_SERVER_NAME]) {
      return {
        configured: true,
        entry: servers[GRES_SERVER_NAME],
      };
    }
  } catch (e) {
    // Key path doesn't exist
  }

  return { configured: false };
}

module.exports = {
  readConfig,
  writeConfig,
  repairConfig,
  checkGresConfigured,
  createBackup,
  GRES_SERVER_NAME,
  BINARY_PATH,
};
