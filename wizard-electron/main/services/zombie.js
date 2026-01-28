/**
 * Zombie Process Guard Service
 *
 * Monitors running AI agent processes and warns when restart is needed.
 * Special handling for Codex CLI: terminal/CLI restart message.
 */

const { execSync, spawn } = require("child_process");
const { getAgentProcessNames, getRestartMessage } = require("./detect");

/**
 * Check if any processes matching the names are running
 */
function checkProcessesRunning(processNames) {
  if (!processNames || processNames.length === 0) {
    return { running: false, processes: [] };
  }

  try {
    const output = execSync("tasklist /FO CSV /NH", {
      encoding: "utf8",
      windowsHide: true,
    });

    const runningProcesses = [];

    for (const line of output.split("\n")) {
      const match = line.match(/"([^"]+)"/);
      if (match) {
        const processName = match[1];
        for (const targetName of processNames) {
          if (processName.toLowerCase() === targetName.toLowerCase()) {
            if (!runningProcesses.includes(processName)) {
              runningProcesses.push(processName);
            }
          }
        }
      }
    }

    return {
      running: runningProcesses.length > 0,
      processes: runningProcesses,
    };
  } catch (e) {
    return { running: false, processes: [], error: e.message };
  }
}

/**
 * Check if a specific agent is running
 */
function checkAgentRunning(agentName) {
  const processNames = getAgentProcessNames(agentName);
  const result = checkProcessesRunning(processNames);

  return {
    ...result,
    agentName,
    restartMessage: getRestartMessage(agentName),
    isCodex: agentName === "Codex CLI",
  };
}

/**
 * Wait for agent to exit with polling
 */
async function waitForAgentExit(agentName, options = {}) {
  const {
    timeout = 120000,
    pollInterval = 3000,
    onCheck = () => {},
  } = options;

  const processNames = getAgentProcessNames(agentName);
  const restartMessage = getRestartMessage(agentName);
  const isCodex = agentName === "Codex CLI";

  const startTime = Date.now();
  let checkCount = 0;

  return new Promise((resolve) => {
    const check = () => {
      checkCount++;
      const elapsed = Date.now() - startTime;

      if (elapsed >= timeout) {
        resolve({
          exited: false,
          timeout: true,
          agentName,
          restartMessage,
          isCodex,
        });
        return;
      }

      const result = checkProcessesRunning(processNames);

      onCheck({
        running: result.running,
        processes: result.processes,
        checkCount,
        elapsed,
        message: result.running
          ? `Still running: ${result.processes.join(", ")}`
          : "Process exited",
      });

      if (!result.running) {
        resolve({
          exited: true,
          agentName,
          restartMessage,
          isCodex,
        });
        return;
      }

      // Schedule next check
      setTimeout(check, pollInterval);
    };

    // Start checking
    check();
  });
}

/**
 * Force kill agent processes (requires user consent)
 */
function forceKillAgent(agentName) {
  const processNames = getAgentProcessNames(agentName);

  if (!processNames || processNames.length === 0) {
    return { success: false, error: "No known process names for this agent" };
  }

  const killed = [];
  const errors = [];

  for (const processName of processNames) {
    try {
      execSync(`taskkill /F /IM "${processName}"`, {
        windowsHide: true,
        encoding: "utf8",
      });
      killed.push(processName);
    } catch (e) {
      // Process might not be running, which is fine
      if (!e.message.includes("not found")) {
        errors.push({ process: processName, error: e.message });
      }
    }
  }

  return {
    success: killed.length > 0 || errors.length === 0,
    killed,
    errors,
    restartMessage: getRestartMessage(agentName),
    isCodex: agentName === "Codex CLI",
  };
}

/**
 * Check all agents for running processes
 */
function checkAllAgentsRunning(agents) {
  const results = [];

  for (const agent of agents) {
    const result = checkAgentRunning(agent.name);
    if (result.running) {
      results.push({
        ...result,
        agent,
      });
    }
  }

  return {
    hasRunning: results.length > 0,
    runningAgents: results,
    count: results.length,
  };
}

module.exports = {
  checkAgentRunning,
  checkProcessesRunning,
  waitForAgentExit,
  forceKillAgent,
  checkAllAgentsRunning,
};
