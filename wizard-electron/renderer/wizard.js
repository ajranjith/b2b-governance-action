/**
 * GRES B2B Setup Wizard - State Machine Controller
 *
 * Protocol-first detection with safe config merges:
 * 1. Detect agents by signature config files (Codex TOML, Claude/Cursor/Windsurf JSON)
 * 2. Offer "Sync All" if multiple agents detected
 * 3. Backup → Parse → Upsert GRES only → Never delete existing MCP
 * 4. Zombie guard with agent-specific restart messages
 * 5. MCP handshake verification
 * 6. Detached scan with progress UI
 */

const wizard = {
  currentStep: "welcome",
  steps: ["welcome", "detect", "install", "zombie", "verify", "success"],
  selectedAgents: [],
  detectedAgents: [],
  data: {
    version: "",
    binaryPath: "",
    installDir: "",
  },

  // ========================================================================
  // Initialization
  // ========================================================================
  init() {
    this.showStep("welcome");
    this.updateVersion();
  },

  updateVersion() {
    const versionEl = document.getElementById("wizardVersion");
    if (versionEl) {
      versionEl.textContent = "v1.1.0";
    }
  },

  // ========================================================================
  // Navigation
  // ========================================================================
  showStep(stepId) {
    document.querySelectorAll(".step").forEach((step) => {
      step.classList.remove("active");
    });

    const step = document.getElementById(`step-${stepId}`);
    if (step) {
      step.classList.add("active");
      this.currentStep = stepId;
    }
  },

  back() {
    const currentIndex = this.steps.indexOf(this.currentStep);
    if (currentIndex > 0) {
      this.showStep(this.steps[currentIndex - 1]);
    }
  },

  // ========================================================================
  // Step 1: Welcome → Start Detection
  // ========================================================================
  start() {
    this.showStep("detect");
    this.runDetection();
  },

  // ========================================================================
  // Step 2: Agent Detection (Protocol-First by Config Files)
  // ========================================================================
  async runDetection() {
    const agentList = document.getElementById("agentList");
    const detectStatus = document.getElementById("detectStatus");
    const btnNext = document.getElementById("btnDetectNext");
    const btnSyncAll = document.getElementById("btnSyncAll");

    agentList.innerHTML = '<div class="note">Scanning for config files...</div>';
    detectStatus.textContent = "Checking Codex TOML, Claude JSON, Cursor JSON, Windsurf JSON...";
    btnNext.disabled = true;
    if (btnSyncAll) btnSyncAll.style.display = "none";

    try {
      const result = await window.gres.detect.agents();

      if (!result.success) {
        throw new Error(result.error || "Detection failed");
      }

      this.detectedAgents = result.agents || [];

      if (this.detectedAgents.length === 0) {
        agentList.innerHTML = `
          <div class="note">No agent config files found.</div>
          <p class="note" style="margin-top: 12px;">
            Supported: Codex CLI, Claude Desktop, Cursor, Windsurf
          </p>
        `;
        detectStatus.textContent = "Create a config file for your AI agent first.";
        return;
      }

      // Check if this is a manual fallback
      const isManualFallback = result.isManualFallback;

      // Render agent list with status badges
      agentList.innerHTML = this.detectedAgents
        .map((agent, index) => `
          <label class="agent-item ${agent.hasGres ? 'configured' : ''} ${!agent.configValid ? 'invalid' : ''} ${agent.status === 'MANUAL' ? 'manual' : ''}" data-agent="${agent.name}" data-status="${agent.status || 'DETECTED'}">
            <input type="checkbox" name="agent" value="${agent.name}" data-index="${index}" ${!agent.hasGres ? 'checked' : ''}>
            <div class="agent-info">
              <div class="agent-name">${agent.name}${agent.status === 'MANUAL' ? ' <span class="text-warning">(Manual)</span>' : ''}</div>
              <div class="agent-source">${agent.configPath}</div>
              ${!agent.configValid && agent.status !== 'MANUAL' ? `<div class="agent-error">Parse error</div>` : ''}
              ${agent.status === 'MANUAL' ? `<div class="agent-note">Config will be created at this path</div>` : ''}
            </div>
            <span class="agent-badge ${agent.configType}">${agent.configType.toUpperCase()}</span>
            ${agent.hasGres ? '<span class="agent-badge configured">Configured</span>' : ''}
            ${agent.status === 'MANUAL' ? '<span class="agent-badge manual">Manual</span>' : ''}
          </label>
        `)
        .join("");

      // Add checkbox listeners
      agentList.querySelectorAll('input[type="checkbox"]').forEach((checkbox) => {
        checkbox.addEventListener("change", () => this.updateAgentSelection());
      });

      // Show Sync All button if multiple agents
      if (btnSyncAll && this.detectedAgents.length > 1) {
        btnSyncAll.style.display = "inline-block";
      }

      this.updateAgentSelection();

      if (isManualFallback) {
        detectStatus.textContent = "No agents auto-detected. Using manual configuration.";
      } else {
        const unconfigured = this.detectedAgents.filter(a => !a.hasGres).length;
        detectStatus.textContent = `Found ${this.detectedAgents.length} agent(s). ${unconfigured} need configuration.`;
      }

    } catch (err) {
      agentList.innerHTML = `<div class="note text-error">Error: ${err.message}</div>`;
      detectStatus.textContent = "Detection failed. Please try again.";
    }
  },

  updateAgentSelection() {
    const checkboxes = document.querySelectorAll('#agentList input[type="checkbox"]:checked');
    this.selectedAgents = Array.from(checkboxes).map((cb) => {
      const index = parseInt(cb.dataset.index, 10);
      return this.detectedAgents[index];
    });

    document.querySelectorAll(".agent-item").forEach((item) => {
      const checkbox = item.querySelector('input[type="checkbox"]');
      item.classList.toggle("selected", checkbox.checked);
    });

    const btnNext = document.getElementById("btnDetectNext");
    btnNext.disabled = this.selectedAgents.length === 0;
  },

  syncAll() {
    document.querySelectorAll('#agentList input[type="checkbox"]').forEach(cb => {
      cb.checked = true;
    });
    this.updateAgentSelection();
    this.selectAgents();
  },

  selectAgents() {
    if (this.selectedAgents.length === 0) return;
    this.showStep("install");
    this.runInstallation();
  },

  // ========================================================================
  // Step 3: Installation (Binary is bundled - no download needed)
  // ========================================================================
  async runInstallation() {
    const steps = ["checksum", "path", "config"];
    const progressFill = document.getElementById("installProgress");
    const progressStatus = document.getElementById("installStatus");
    const progressPercent = document.getElementById("installPercent");

    const setProgress = (percent, status) => {
      progressFill.style.width = percent + "%";
      progressPercent.textContent = percent + "%";
      progressStatus.textContent = status;
    };

    const setStepStatus = (stepId, status, detail) => {
      const stepEl = document.getElementById(`step-${stepId}`);
      const detailEl = document.getElementById(`detail-${stepId}`);
      if (!stepEl || !detailEl) return;
      stepEl.className = "progress-step " + status;
      if (detail) detailEl.textContent = detail;
      const statusEl = stepEl.querySelector(".step-status");
      if (status === "active") statusEl.innerHTML = "";
      else if (status === "success") statusEl.innerHTML = "&#10003;";
      else if (status === "error") statusEl.innerHTML = "&#10007;";
    };

    steps.forEach((s) => setStepStatus(s, "", "Waiting..."));
    setProgress(0, "Starting...");

    try {
      // Get install paths
      const paths = await window.gres.install.getPaths();
      this.data.installDir = paths.installDir;
      this.data.binaryPath = paths.binaryPath;

      // Step 1: Install bundled binary
      setStepStatus("checksum", "active", "Installing binary...");
      setProgress(5, "Installing gres-b2b...");

      window.gres.install.onProgress((percent) => {
        setProgress(5 + Math.floor(percent * 0.35), `Installing... ${percent}%`);
      });

      const installResult = await window.gres.install.downloadBinary();
      window.gres.install.offProgress();

      if (!installResult.success) {
        throw new Error(installResult.error || "Installation failed");
      }

      this.data.version = installResult.version;
      setStepStatus("checksum", "success", `v${installResult.version}`);

      // Step 2: Configure PATH
      setStepStatus("path", "active", "Updating PATH...");
      setProgress(50, "Configuring system PATH...");

      const pathResult = await window.gres.install.applyPath();
      if (!pathResult.success) {
        throw new Error(pathResult.error || "PATH configuration failed");
      }
      setStepStatus("path", "success", pathResult.alreadyInPath ? "Already set" : "Updated");

      // Step 3: Write agent configs (safe merge with backup)
      setStepStatus("config", "active", "Configuring agents...");
      setProgress(70, "Writing agent configurations...");

      let successCount = 0;
      let errorCount = 0;

      for (const agent of this.selectedAgents) {
        const configResult = await window.gres.config.write(agent);

        if (configResult.success) {
          successCount++;
        } else if (configResult.needsRepair) {
          const repaired = await this.handleConfigError(agent, configResult);
          if (repaired) successCount++;
          else errorCount++;
        } else {
          errorCount++;
        }
      }

      if (errorCount > 0 && successCount === 0) {
        throw new Error(`Failed to configure all ${errorCount} agent(s)`);
      }

      setStepStatus("config", "success", `${successCount} agent(s)`);
      setProgress(100, "Installation complete!");

      await this.sleep(500);
      await this.checkForRunningAgents();

    } catch (err) {
      const failedStep = steps.find((s) => {
        const el = document.getElementById(`step-${s}`);
        return el.classList.contains("active");
      });
      if (failedStep) setStepStatus(failedStep, "error", err.message);
      this.showError("Installation Failed", err.message);
    }
  },

  async handleConfigError(agent, result) {
    const choice = await this.showRepairDialog(agent, result.error);
    if (choice === "repair") {
      const repairResult = await window.gres.config.repair(agent);
      return repairResult.success;
    } else if (choice === "open") {
      await window.gres.config.open(agent.configPath);
      return false;
    }
    return false;
  },

  showRepairDialog(agent, error) {
    return new Promise((resolve) => {
      const overlay = document.createElement("div");
      overlay.className = "dialog-overlay";
      overlay.innerHTML = `
        <div class="repair-dialog">
          <h3>Config Parse Error</h3>
          <p><strong>${agent.name}</strong> config could not be parsed:</p>
          <div class="error-box" style="font-size: 0.75rem; max-height: 60px; overflow: auto;">${error}</div>
          <p style="margin-top: 12px;">What would you like to do?</p>
          <div class="step-actions" style="margin-top: 16px;">
            <button class="btn btn-danger" id="btnRepair">Repair (Overwrite)</button>
            <button class="btn btn-secondary" id="btnOpen">Open File</button>
            <button class="btn btn-secondary" id="btnSkip">Skip</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);

      overlay.querySelector("#btnRepair").onclick = () => {
        document.body.removeChild(overlay);
        resolve("repair");
      };
      overlay.querySelector("#btnOpen").onclick = () => {
        document.body.removeChild(overlay);
        resolve("open");
      };
      overlay.querySelector("#btnSkip").onclick = () => {
        document.body.removeChild(overlay);
        resolve("skip");
      };
    });
  },

  // ========================================================================
  // Step 4: Zombie Guard
  // ========================================================================
  async checkForRunningAgents() {
    const result = await window.gres.zombie.checkAll(this.selectedAgents);

    if (result.hasRunning) {
      const running = result.runningAgents[0];
      this.currentZombieAgent = running;

      document.getElementById("zombieAgent").textContent = running.agentName;
      document.getElementById("zombieProcess").textContent = running.processes?.join(", ") || running.agentName;

      // Agent-specific restart message (Codex = terminal restart)
      const restartEl = document.getElementById("zombieRestart");
      if (restartEl) {
        restartEl.textContent = running.restartMessage || "Please close the application.";
      }

      const statusEl = document.getElementById("zombieStatus");
      const forceBtn = document.getElementById("btnForceKill");
      const skipBtn = document.getElementById("btnSkipZombie");

      if (running.isCodex) {
        statusEl.textContent = "Restart your terminal/CLI session to apply changes.";
        if (forceBtn) forceBtn.style.display = "none";
        if (skipBtn) skipBtn.style.display = "inline-block";
      } else {
        statusEl.textContent = "Waiting for process to exit...";
        if (forceBtn) forceBtn.style.display = "inline-block";
        if (skipBtn) skipBtn.style.display = "none";
      }

      this.showStep("zombie");

      if (!running.isCodex) {
        this.startZombieWatch();
      }
    } else {
      this.showStep("verify");
      this.runVerification();
    }
  },

  async startZombieWatch() {
    const statusEl = document.getElementById("zombieStatus");

    window.gres.zombie.onStatus((status) => {
      statusEl.textContent = status.message || "Checking...";
    });

    try {
      const result = await window.gres.zombie.waitForExit({
        agentName: this.currentZombieAgent.agentName,
        timeout: 120000,
        pollInterval: 3000,
      });

      window.gres.zombie.offStatus();

      if (result.exited) {
        statusEl.textContent = "Process exited. Proceeding...";
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else if (result.timeout) {
        statusEl.textContent = "Timeout. Use Force Close or close manually.";
      }
    } catch (err) {
      window.gres.zombie.offStatus();
      statusEl.textContent = `Error: ${err.message}`;
    }
  },

  skipZombie() {
    this.showStep("verify");
    this.runVerification();
  },

  async forceKill() {
    const statusEl = document.getElementById("zombieStatus");
    statusEl.textContent = "Force closing...";

    try {
      const result = await window.gres.zombie.forceKill(this.currentZombieAgent.agentName);

      if (result.success) {
        statusEl.textContent = "Process closed. Proceeding...";
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else {
        statusEl.textContent = result.error || "Failed to close process";
      }
    } catch (err) {
      statusEl.textContent = `Error: ${err.message}`;
    }
  },

  // ========================================================================
  // Step 5: Verification (MCP Handshake)
  // ========================================================================
  async runVerification() {
    const tests = [
      { id: "binary", fn: () => window.gres.verify.binaryVersion() },
      { id: "path", fn: () => window.gres.verify.pathWhere() },
      { id: "mcp", fn: () => window.gres.mcp.selftest() },
    ];

    let allPassed = true;

    for (const test of tests) {
      const stepEl = document.getElementById(`verify-${test.id}`);
      const detailEl = document.getElementById(`verify-${test.id}-detail`);
      const statusEl = stepEl.querySelector(".step-status");

      stepEl.className = "progress-step active";
      statusEl.innerHTML = "";
      detailEl.textContent = "Testing...";

      try {
        const result = await test.fn();

        if (result.success) {
          stepEl.className = "progress-step success";
          statusEl.innerHTML = "&#10003;";
          detailEl.textContent = result.message || result.version || "Passed";
        } else {
          stepEl.className = "progress-step error";
          statusEl.innerHTML = "&#10007;";
          detailEl.textContent = result.error || "Failed";
          allPassed = false;
        }
      } catch (err) {
        stepEl.className = "progress-step error";
        statusEl.innerHTML = "&#10007;";
        detailEl.textContent = err.message;
        allPassed = false;
      }

      await this.sleep(400);
    }

    const resultEl = document.getElementById("verifyResult");

    if (allPassed) {
      resultEl.innerHTML = `<div class="note text-success" style="margin-top: 20px;">All tests passed!</div>`;
      await this.sleep(1500);
      this.showSuccess();
    } else {
      resultEl.innerHTML = `
        <div class="note text-error" style="margin-top: 20px;">Some tests failed.</div>
        <div class="step-actions">
          <button class="btn btn-secondary" onclick="wizard.retry()">Retry</button>
          <button class="btn btn-primary" onclick="wizard.openDocs()">Get Help</button>
        </div>
      `;
    }
  },

  // ========================================================================
  // Step 6: Success
  // ========================================================================
  async showSuccess() {
    document.getElementById("successAgent").textContent =
      this.selectedAgents.map((a) => a.name).join(", ");
    document.getElementById("successVersion").textContent = "v" + this.data.version;
    document.getElementById("successConfig").textContent =
      this.selectedAgents.length + " agent(s)";

    this.showStep("success");

    try {
      await window.gres.util.createShortcut({
        name: "GRES B2B Dashboard",
        target: this.data.binaryPath,
        args: ["dashboard"],
      });
    } catch (err) {
      console.warn("Shortcut failed:", err);
    }
  },

  // ========================================================================
  // Actions
  // ========================================================================
  async launchDashboard() {
    try {
      const portResult = await window.gres.scan.findPort(8080);
      if (portResult.success) {
        const scanResult = await window.gres.scan.startDetached({
          port: portResult.port,
          live: true,
        });
        await this.sleep(1500);
        await window.gres.util.openUrl(`http://localhost:${portResult.port}`);
      }
    } catch (err) {
      window.gres.util.openUrl("http://localhost:8080");
    }
  },

  showError(title, message, details = "") {
    document.getElementById("errorTitle").textContent = title;
    document.getElementById("errorMessage").textContent = message;
    document.getElementById("errorDetails").textContent = details;
    this.showStep("error");
  },

  retry() {
    ["binary", "path", "mcp"].forEach((id) => {
      const stepEl = document.getElementById(`verify-${id}`);
      const detailEl = document.getElementById(`verify-${id}-detail`);
      const statusEl = stepEl.querySelector(".step-status");
      stepEl.className = "progress-step";
      statusEl.innerHTML = "";
      detailEl.textContent = "Waiting...";
    });
    document.getElementById("verifyResult").innerHTML = "";
    this.runVerification();
  },

  openDocs() {
    window.gres.util.openUrl("https://github.com/ajranjith/b2b-governance-action#readme");
  },

  sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  },
};

document.addEventListener("DOMContentLoaded", () => {
  wizard.init();
});
