/**
 * Agent Detection Service - Protocol-First Approach
 *
 * Detects AI agents by their signature config files:
 * 1. Codex CLI - TOML config
 * 2. Claude Desktop - JSON config
 * 3. Cursor - JSON config
 * 4. Windsurf - JSON config
 * 5. Generic MCP - JSON config
 */

const fs = require("fs");
const path = require("path");
const os = require("os");

// Protocol-first agent definitions - detect by config file signature
const AGENT_SIGNATURES = {
  "Codex CLI": {
    configPaths: [
      path.join(os.homedir(), ".codex", "config.toml"),
      path.join(os.homedir(), ".config", "codex", "config.toml"),
    ],
    configType: "toml",
    mcpKey: "mcp.servers",
    gresKey: "gres_b2b",
    processNames: ["codex", "codex.exe"],
    restartMessage: "Please restart your terminal/CLI session to apply changes.",
    icon: "terminal",
  },
  "Claude Desktop": {
    configPaths: [
      path.join(os.homedir(), "AppData", "Roaming", "Claude", "claude_desktop_config.json"),
      path.join(os.homedir(), ".config", "claude", "claude_desktop_config.json"),
      path.join(os.homedir(), "Library", "Application Support", "Claude", "claude_desktop_config.json"),
    ],
    configType: "json",
    mcpKey: "mcpServers",
    gresKey: "gres-b2b",
    processNames: ["Claude.exe", "claude.exe", "Claude"],
    restartMessage: "Please close and reopen Claude Desktop to apply changes.",
    icon: "chat",
  },
  "Cursor": {
    configPaths: [
      path.join(os.homedir(), ".cursor", "mcp.json"),
      path.join(os.homedir(), "AppData", "Roaming", "Cursor", "User", "mcp.json"),
    ],
    configType: "json",
    mcpKey: "mcpServers",
    gresKey: "gres-b2b",
    processNames: ["Cursor.exe", "cursor.exe", "Cursor"],
    restartMessage: "Please close and reopen Cursor to apply changes.",
    icon: "code",
  },
  "Windsurf": {
    configPaths: [
      path.join(os.homedir(), ".codeium", "windsurf", "mcp_config.json"),
      path.join(os.homedir(), "AppData", "Roaming", "Windsurf", "User", "mcp.json"),
      path.join(os.homedir(), ".config", "windsurf", "mcp.json"),
    ],
    configType: "json",
    mcpKey: "mcpServers",
    gresKey: "gres-b2b",
    processNames: ["Windsurf.exe", "windsurf.exe", "Windsurf"],
    restartMessage: "Please close and reopen Windsurf to apply changes.",
    icon: "wind",
  },
  "Generic MCP": {
    configPaths: [
      path.join(os.homedir(), ".mcp", "config.json"),
      path.join(os.homedir(), ".config", "mcp", "servers.json"),
    ],
    configType: "json",
    mcpKey: "servers",
    gresKey: "gres-b2b",
    processNames: [],
    restartMessage: "Please restart your MCP client to apply changes.",
    icon: "puzzle",
  },
};

/**
 * Parse TOML config (basic parser for MCP section)
 */
function parseTOML(content) {
  const result = {};
  let currentSection = result;
  const lines = content.split("\n");

  for (const line of lines) {
    const trimmed = line.trim();

    // Skip comments and empty lines
    if (!trimmed || trimmed.startsWith("#")) continue;

    // Section header [section] or [section.subsection]
    const sectionMatch = trimmed.match(/^\[([^\]]+)\]$/);
    if (sectionMatch) {
      const path = sectionMatch[1].split(".");
      currentSection = result;
      for (const key of path) {
        if (!currentSection[key]) currentSection[key] = {};
        currentSection = currentSection[key];
      }
      continue;
    }

    // Key-value pair
    const kvMatch = trimmed.match(/^([^=]+)=\s*(.+)$/);
    if (kvMatch) {
      const key = kvMatch[1].trim();
      let value = kvMatch[2].trim();

      // Parse value type
      if (value.startsWith('"') && value.endsWith('"')) {
        value = value.slice(1, -1);
      } else if (value === "true") {
        value = true;
      } else if (value === "false") {
        value = false;
      } else if (!isNaN(value)) {
        value = Number(value);
      }

      currentSection[key] = value;
    }
  }

  return result;
}

/**
 * Try to parse JSON with recovery for common issues
 */
function parseJSONSafe(content) {
  try {
    return { success: true, data: JSON.parse(content) };
  } catch (e) {
    // Try to fix common JSON issues
    let fixed = content;

    // Remove trailing commas
    fixed = fixed.replace(/,(\s*[}\]])/g, "$1");

    // Try again
    try {
      return { success: true, data: JSON.parse(fixed), wasRepaired: true };
    } catch (e2) {
      return { success: false, error: e.message, original: e };
    }
  }
}

/**
 * Detect agents by their signature config files
 * UNIVERSAL: Never returns error - always returns at least Manual fallback
 */
async function detectAgents() {
  const detected = [];

  for (const [agentName, signature] of Object.entries(AGENT_SIGNATURES)) {
    for (const configPath of signature.configPaths) {
      if (fs.existsSync(configPath)) {
        const agent = {
          id: agentName.toLowerCase().replace(/\s+/g, "-"),
          name: agentName,
          configPath,
          configType: signature.configType,
          mcpKey: signature.mcpKey,
          gresKey: signature.gresKey,
          processNames: signature.processNames,
          restartMessage: signature.restartMessage,
          icon: signature.icon,
          configExists: true,
          status: "DETECTED",
        };

        // Try to parse the config
        try {
          const content = fs.readFileSync(configPath, "utf8");

          if (signature.configType === "toml") {
            const parsed = parseTOML(content);
            agent.configValid = true;
            agent.configData = parsed;
            agent.hasGres = !!(parsed.mcp_servers?.gres_b2b || parsed.mcp?.servers?.gres_b2b);
          } else {
            const result = parseJSONSafe(content);
            if (result.success) {
              agent.configValid = true;
              agent.configData = result.data;
              agent.wasRepaired = result.wasRepaired;

              // Check if GRES is already configured
              const mcpServers = getNestedValue(result.data, signature.mcpKey);
              agent.hasGres = !!(mcpServers && mcpServers[signature.gresKey]);
            } else {
              agent.configValid = false;
              agent.configError = result.error;
            }
          }
        } catch (e) {
          agent.configValid = false;
          agent.configError = e.message;
        }

        detected.push(agent);
        break; // Found config for this agent, move to next
      }
    }
  }

  // UNIVERSAL: If no agents detected, return Manual fallback
  // NEVER return "Detection failed"
  if (detected.length === 0) {
    return {
      success: true,
      agents: [createManualFallback()],
      hasAgents: true,
      multipleAgents: false,
      isManualFallback: true,
    };
  }

  return {
    success: true,
    agents: detected,
    hasAgents: true,
    multipleAgents: detected.length > 1,
    isManualFallback: false,
  };
}

/**
 * Create manual fallback agent for when no signature files are found
 */
function createManualFallback() {
  // Default to Claude Desktop config path
  const defaultConfigPath = path.join(
    os.homedir(),
    "AppData",
    "Roaming",
    "Claude",
    "claude_desktop_config.json"
  );

  return {
    id: "manual",
    name: "Generic AI Agent",
    configPath: defaultConfigPath,
    configType: "json",
    mcpKey: "mcpServers",
    gresKey: "gres-b2b",
    processNames: [],
    restartMessage: "Please restart your AI agent to apply changes.",
    icon: "puzzle",
    configExists: false,
    configValid: true,
    hasGres: false,
    status: "MANUAL",
  };
}

/**
 * Get nested value from object using dot notation
 */
function getNestedValue(obj, path) {
  const keys = path.split(".");
  let current = obj;

  for (const key of keys) {
    if (current === undefined || current === null) return undefined;
    current = current[key];
  }

  return current;
}

/**
 * Get agent process names for zombie detection
 */
function getAgentProcessNames(agentName) {
  const signature = AGENT_SIGNATURES[agentName];
  return signature ? signature.processNames : [];
}

/**
 * Get restart message for agent
 */
function getRestartMessage(agentName) {
  const signature = AGENT_SIGNATURES[agentName];
  return signature ? signature.restartMessage : "Please restart the application to apply changes.";
}

module.exports = {
  detectAgents,
  getAgentProcessNames,
  getRestartMessage,
  createManualFallback,
  AGENT_SIGNATURES,
  parseJSONSafe,
  parseTOML,
};
