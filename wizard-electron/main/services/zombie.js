/**
 * Zombie Process Guard Service
 *
 * Monitors AI agent processes to ensure they've fully exited
 * before proceeding with config changes. This prevents file
 * locking issues and ensures clean restarts.
 */

const { exec, execSync } = require("child_process");

// Process name mappings for each agent
const PROCESS_NAMES = {
  "Claude Desktop": ["Claude.exe", "claude.exe"],
  "Cursor": ["Cursor.exe", "cursor.exe"],
  "VS Code (Windsurf)": ["Windsurf.exe", "windsurf.exe", "Code.exe", "code.exe"],
  "Codex CLI": ["codex.exe"],
};

/**
 * Check if a process is running
 */
function isProcessRunning(processName) {
  return new Promise((resolve) => {
    exec(`tasklist /FI "IMAGENAME eq ${processName}" /NH`, { windowsHide: true }, (err, stdout) => {
      if (err) {
        resolve(false);
        return;
      }
      // If process is running, tasklist will show its name
      resolve(stdout.toLowerCase().includes(processName.toLowerCase()));
    });
  });
}

/**
 * Check if any processes for an agent are running
 */
async function checkAgentRunning(agentName) {
  const processNames = PROCESS_NAMES[agentName] || [agentName.replace(/\s/g, "") + ".exe"];

  for (const procName of processNames) {
    if (await isProcessRunning(procName)) {
      return {
        running: true,
        processName: procName,
      };
    }
  }

  return { running: false };
}

/**
 * Poll for agent process to exit
 * Returns a promise that resolves when the agent is no longer running
 */
function waitForAgentExit(agentName, options = {}) {
  const { timeout = 60000, interval = 1500, onCheck } = options;

  return new Promise((resolve, reject) => {
    const startTime = Date.now();
    let checkCount = 0;

    const poll = async () => {
      checkCount++;
      const status = await checkAgentRunning(agentName);

      if (onCheck) {
        onCheck({ checkCount, status, elapsed: Date.now() - startTime });
      }

      if (!status.running) {
        resolve({
          success: true,
          checkCount,
          elapsed: Date.now() - startTime,
        });
        return;
      }

      if (Date.now() - startTime >= timeout) {
        resolve({
          success: false,
          timeout: true,
          processName: status.processName,
          checkCount,
        });
        return;
      }

      setTimeout(poll, interval);
    };

    poll();
  });
}

/**
 * Force kill an agent process (requires user consent)
 */
async function forceKillAgent(agentName) {
  const processNames = PROCESS_NAMES[agentName] || [agentName.replace(/\s/g, "") + ".exe"];

  const killed = [];
  const failed = [];

  for (const procName of processNames) {
    try {
      execSync(`taskkill /F /IM ${procName}`, { windowsHide: true, stdio: "pipe" });
      killed.push(procName);
    } catch (e) {
      // Process might not exist or access denied
      if (e.message && !e.message.includes("not found")) {
        failed.push({ process: procName, error: e.message });
      }
    }
  }

  // Wait a moment for processes to fully terminate
  await new Promise((r) => setTimeout(r, 1000));

  // Verify kill
  const stillRunning = await checkAgentRunning(agentName);

  return {
    success: !stillRunning.running,
    killed,
    failed,
    stillRunning: stillRunning.running ? stillRunning.processName : null,
  };
}

/**
 * Get running status of all known agents
 */
async function getAllAgentStatus() {
  const statuses = {};

  for (const agentName of Object.keys(PROCESS_NAMES)) {
    statuses[agentName] = await checkAgentRunning(agentName);
  }

  return statuses;
}

module.exports = {
  checkAgentRunning,
  waitForAgentExit,
  forceKillAgent,
  getAllAgentStatus,
  PROCESS_NAMES,
};
