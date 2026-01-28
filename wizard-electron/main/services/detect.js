/**
 * Agent Detection Service
 *
 * Detects installed AI agents via:
 * 1. Registry (HKCU/HKLM Uninstall keys) - match on DisplayName
 * 2. Disk fallback for portable/user-only installs
 */

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const os = require("os");

// Agent definitions with detection rules and config paths
const AGENT_DEFINITIONS = {
  "Claude Desktop": {
    registryNames: ["Claude", "Claude Desktop"],
    diskPaths: [
      path.join(os.homedir(), "AppData", "Local", "Programs", "Claude"),
      path.join(os.homedir(), "AppData", "Local", "Claude"),
    ],
    configPath: path.join(os.homedir(), "AppData", "Roaming", "Claude", "claude_desktop_config.json"),
    configType: "json",
    mcpKey: "mcpServers",
  },
  "Cursor": {
    registryNames: ["Cursor", "Cursor Editor"],
    diskPaths: [
      path.join(os.homedir(), "AppData", "Local", "Programs", "cursor"),
      path.join(os.homedir(), ".cursor"),
    ],
    configPath: path.join(os.homedir(), ".cursor", "mcp.json"),
    configType: "json",
    mcpKey: "mcpServers",
  },
  "VS Code (Windsurf)": {
    registryNames: ["Windsurf", "Visual Studio Code", "VS Code"],
    diskPaths: [
      path.join(os.homedir(), ".codeium", "windsurf"),
      path.join(os.homedir(), "AppData", "Local", "Programs", "Microsoft VS Code"),
    ],
    configPath: path.join(os.homedir(), "AppData", "Roaming", "Windsurf", "User", "settings.json"),
    altConfigPath: path.join(os.homedir(), "AppData", "Roaming", "Code", "User", "globalStorage", "mcp.json"),
    configType: "json",
    mcpKey: "mcp.servers",
  },
  "Codex CLI": {
    registryNames: ["Codex CLI", "OpenAI Codex"],
    diskPaths: [
      path.join(os.homedir(), ".codex"),
      path.join(os.homedir(), "AppData", "Local", "codex"),
    ],
    configPath: path.join(os.homedir(), ".codex", "config.json"),
    configType: "json",
    mcpKey: "mcpServers",
  },
};

/**
 * Query Windows registry for installed applications
 */
function queryRegistry(hive, path) {
  try {
    const cmd = `reg query "${hive}\\${path}" /s 2>nul`;
    const output = execSync(cmd, { encoding: "utf8", windowsHide: true });
    return output;
  } catch (e) {
    return "";
  }
}

/**
 * Parse registry output to find DisplayNames
 */
function parseRegistryForApps(registryOutput) {
  const apps = [];
  const lines = registryOutput.split("\n");
  let currentKey = "";
  let currentApp = {};

  for (const line of lines) {
    if (line.startsWith("HKEY_")) {
      if (currentApp.displayName) {
        apps.push({ ...currentApp, registryKey: currentKey });
      }
      currentKey = line.trim();
      currentApp = {};
    } else if (line.includes("DisplayName")) {
      const match = line.match(/DisplayName\s+REG_SZ\s+(.+)/);
      if (match) {
        currentApp.displayName = match[1].trim();
      }
    } else if (line.includes("InstallLocation")) {
      const match = line.match(/InstallLocation\s+REG_SZ\s+(.+)/);
      if (match) {
        currentApp.installLocation = match[1].trim();
      }
    }
  }

  if (currentApp.displayName) {
    apps.push({ ...currentApp, registryKey: currentKey });
  }

  return apps;
}

/**
 * Detect agents from registry
 */
function detectFromRegistry() {
  const detected = [];
  const registryPaths = [
    { hive: "HKCU", path: "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall" },
    { hive: "HKLM", path: "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall" },
    { hive: "HKLM", path: "Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall" },
  ];

  for (const { hive, path: regPath } of registryPaths) {
    const output = queryRegistry(hive, regPath);
    const apps = parseRegistryForApps(output);

    for (const [agentName, agentDef] of Object.entries(AGENT_DEFINITIONS)) {
      for (const app of apps) {
        if (agentDef.registryNames.some((name) =>
          app.displayName && app.displayName.toLowerCase().includes(name.toLowerCase())
        )) {
          if (!detected.find((d) => d.name === agentName)) {
            detected.push({
              name: agentName,
              source: "registry",
              evidence: `${hive}\\${regPath}`,
              installPath: app.installLocation || null,
              configPath: agentDef.configPath,
              configType: agentDef.configType,
              mcpKey: agentDef.mcpKey,
            });
          }
        }
      }
    }
  }

  return detected;
}

/**
 * Detect agents from disk (fallback for portable installs)
 */
function detectFromDisk() {
  const detected = [];

  for (const [agentName, agentDef] of Object.entries(AGENT_DEFINITIONS)) {
    for (const diskPath of agentDef.diskPaths) {
      if (fs.existsSync(diskPath)) {
        if (!detected.find((d) => d.name === agentName)) {
          detected.push({
            name: agentName,
            source: "disk",
            evidence: diskPath,
            installPath: diskPath,
            configPath: agentDef.configPath,
            configType: agentDef.configType,
            mcpKey: agentDef.mcpKey,
          });
        }
        break;
      }
    }
  }

  return detected;
}

/**
 * Main detection function - combines registry and disk detection
 */
async function detectAgents() {
  const registryAgents = detectFromRegistry();
  const diskAgents = detectFromDisk();

  // Merge results, preferring registry source
  const allAgents = [...registryAgents];

  for (const diskAgent of diskAgents) {
    if (!allAgents.find((a) => a.name === diskAgent.name)) {
      allAgents.push(diskAgent);
    }
  }

  // Add config file existence check
  for (const agent of allAgents) {
    agent.configExists = fs.existsSync(agent.configPath);

    // Check if config is valid JSON/TOML
    if (agent.configExists) {
      try {
        const content = fs.readFileSync(agent.configPath, "utf8");
        if (agent.configType === "json") {
          JSON.parse(content);
          agent.configValid = true;
        } else {
          agent.configValid = true; // TOML validation would go here
        }
      } catch (e) {
        agent.configValid = false;
        agent.configError = e.message;
      }
    }
  }

  return {
    agents: allAgents,
    hasAgents: allAgents.length > 0,
  };
}

/**
 * Get agent process name for zombie detection
 */
function getAgentProcessName(agentName) {
  const processMap = {
    "Claude Desktop": "Claude",
    "Cursor": "Cursor",
    "VS Code (Windsurf)": "Windsurf",
    "Codex CLI": "codex",
  };
  return processMap[agentName] || agentName.split(" ")[0];
}

module.exports = {
  detectAgents,
  getAgentProcessName,
  AGENT_DEFINITIONS,
};
