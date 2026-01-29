/**
 * GRES B2B Setup Wizard - ID-based State Machine Controller
 *
 * Section-based architecture with gate tests:
 * - Each section has a stable ID (WZ-001 to WZ-010)
 * - "Next" is disabled until section test passes
 * - Preflight (WZ-001) runs immediately on start
 * - All transitions logged to ndjson
 *
 * Sections:
 * WZ-001: Preflight (OS/arch, permissions)
 * WZ-002: Agent Detection
 * WZ-003: Config Merge
 * WZ-004: Install Binary
 * WZ-005: Binary Proof
 * WZ-006: MCP Selftest
 * WZ-007: Restart Check
 * WZ-008-010: Scan (optional)
 */

const wizard = {
  currentStep: "preflight",
  steps: ["preflight", "welcome", "detect", "install", "zombie", "verify", "success"],
  selectedAgents: [],
  detectedAgents: [],
  sectionResults: new Map(),
  data: {
    version: "",
    binaryPath: "",
    installDir: "",
  },

  // ========================================================================
  // Initialization with Preflight-First Boot
  // ========================================================================
  async init() {
    this.showStep("preflight");
    this.updateVersion();

    try {
      // Initialize wizard state machine
      await window.gres.wizard.initialize();

      // Run preflight (WZ-001) - must pass before proceeding
      const preflightResult = await window.gres.wizard.preflight();
      this.sectionResults.set("WZ-001", preflightResult);

      if (!preflightResult.pass) {
        this.showPreflightError(preflightResult);
        return;
      }

      // Preflight passed - show welcome
      await this.sleep(500);
      this.showStep("welcome");

    } catch (err) {
      this.showPreflightError({
        pass: false,
        code: "ERR_INIT",
        message: err.message,
      });
    }
  },

  showPreflightError(result) {
    const statusEl = document.getElementById("preflightStatus");
    const detailEl = document.getElementById("preflightDetail");
    const retryBtn = document.getElementById("btnPreflightRetry");

    statusEl.innerHTML = `
      <div class="section-badge error">WZ-001 FAILED</div>
      <div class="error-message">${result.message}</div>
    `;

    if (result.evidence) {
      detailEl.innerHTML = `<pre>${JSON.stringify(result.evidence, null, 2)}</pre>`;
    }

    retryBtn.style.display = "inline-block";
  },

  async retryPreflight() {
    document.getElementById("preflightStatus").innerHTML = `
      <div class="section-badge">WZ-001</div>
      <div>Running preflight checks...</div>
    `;
    document.getElementById("preflightDetail").innerHTML = "";
    document.getElementById("btnPreflightRetry").style.display = "none";

    await this.init();
  },

  updateVersion() {
    const versionEl = document.getElementById("wizardVersion");
    if (versionEl) {
      versionEl.textContent = "v1.2.0";
    }
  },

  // ========================================================================
  // Navigation with Gate Tests
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
  // Step 1: Welcome -> Start Detection
  // ========================================================================
  start() {
    this.showStep("detect");
    this.runDetection();
  },

  // ========================================================================
  // Step 2: Agent Detection (WZ-002)
  // ========================================================================
  async runDetection() {
    const agentList = document.getElementById("agentList");
    const detectStatus = document.getElementById("detectStatus");
    const btnNext = document.getElementById("btnDetectNext");
    const btnSyncAll = document.getElementById("btnSyncAll");
    const sectionBadge = document.getElementById("detectSectionBadge");

    if (sectionBadge) sectionBadge.textContent = "WZ-002";

    agentList.innerHTML = '<div class="note">Scanning for config files...</div>';
    detectStatus.textContent = "Checking Codex TOML, Claude JSON, Cursor JSON, Windsurf JSON...";
    btnNext.disabled = true;
    if (btnSyncAll) btnSyncAll.style.display = "none";

    try {
      // Run WZ-002 through state machine
      const result = await window.gres.wizard.runSectionById("WZ-002");
      this.sectionResults.set("WZ-002", result);

      // Get context to access detected agents
      const ctx = await window.gres.wizard.getContext();
      this.detectedAgents = ctx.agents || [];

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

      const isManualFallback = ctx.isManualFallback;

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

      // Update status with section result
      const statusIcon = result.pass ? "PASSED" : "FAILED";
      if (isManualFallback) {
        detectStatus.innerHTML = `<span class="section-badge ${result.pass ? 'success' : 'error'}">${statusIcon}</span> No agents auto-detected. Using manual configuration.`;
      } else {
        const unconfigured = this.detectedAgents.filter(a => !a.hasGres).length;
        detectStatus.innerHTML = `<span class="section-badge ${result.pass ? 'success' : 'error'}">${statusIcon}</span> Found ${this.detectedAgents.length} agent(s). ${unconfigured} need configuration.`;
      }

    } catch (err) {
      agentList.innerHTML = `<div class="note text-error">Error: ${err.message}</div>`;
      detectStatus.innerHTML = `<span class="section-badge error">FAILED</span> Detection failed. Please try again.`;
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

    // Gate test: can only advance if at least one agent selected AND WZ-002 passed
    const btnNext = document.getElementById("btnDetectNext");
    const wz002Result = this.sectionResults.get("WZ-002");
    btnNext.disabled = this.selectedAgents.length === 0 || !wz002Result?.pass;
  },

  syncAll() {
    document.querySelectorAll('#agentList input[type="checkbox"]').forEach(cb => {
      cb.checked = true;
    });
    this.updateAgentSelection();
    this.selectAgents();
  },

  async selectAgents() {
    if (this.selectedAgents.length === 0) return;

    // Store selected agent in context
    await window.gres.wizard.setContext("selectedAgent", this.selectedAgents[0]);

    this.showStep("install");
    this.runInstallation();
  },

  // ========================================================================
  // Step 3: Installation (WZ-003, WZ-004, WZ-005)
  // ========================================================================
  async runInstallation() {
    const progressFill = document.getElementById("installProgress");
    const progressStatus = document.getElementById("installStatus");
    const progressPercent = document.getElementById("installPercent");
    const sectionBadge = document.getElementById("installSectionBadge");

    const setProgress = (percent, status, sectionId) => {
      progressFill.style.width = percent + "%";
      progressPercent.textContent = percent + "%";
      if (sectionId) {
        progressStatus.innerHTML = `<span class="section-badge">${sectionId}</span> ${status}`;
      } else {
        progressStatus.textContent = status;
      }
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

    ["checksum", "path", "config"].forEach((s) => setStepStatus(s, "", "Waiting..."));
    setProgress(0, "Starting...", null);

    try {
      // Get install paths
      const paths = await window.gres.install.getPaths();
      this.data.installDir = paths.installDir;
      this.data.binaryPath = paths.binaryPath;

      // WZ-004: Install Binary
      if (sectionBadge) sectionBadge.textContent = "WZ-004";
      setStepStatus("checksum", "active", "Installing binary...");
      setProgress(5, "Installing gres-b2b...", "WZ-004");

      window.gres.wizard.onProgress((percent) => {
        setProgress(5 + Math.floor(percent * 0.35), `Installing... ${percent}%`, "WZ-004");
      });

      const wz004Result = await window.gres.wizard.runSectionById("WZ-004");
      window.gres.wizard.offProgress();
      this.sectionResults.set("WZ-004", wz004Result);

      if (!wz004Result.pass) {
        throw new Error(wz004Result.message || "Installation failed");
      }

      const ctx = await window.gres.wizard.getContext();
      this.data.version = ctx.downloadResult?.version || "unknown";
      setStepStatus("checksum", "success", `v${this.data.version}`);

      // WZ-005: Binary Proof
      if (sectionBadge) sectionBadge.textContent = "WZ-005";
      setStepStatus("path", "active", "Verifying binary...");
      setProgress(50, "Running binary verification...", "WZ-005");

      const wz005Result = await window.gres.wizard.runSectionById("WZ-005");
      this.sectionResults.set("WZ-005", wz005Result);

      if (!wz005Result.pass) {
        throw new Error(wz005Result.message || "Binary verification failed");
      }
      setStepStatus("path", "success", "Verified");

      // WZ-003: Config Merge
      if (sectionBadge) sectionBadge.textContent = "WZ-003";
      setStepStatus("config", "active", "Configuring agents...");
      setProgress(70, "Writing agent configurations...", "WZ-003");

      let successCount = 0;
      let errorCount = 0;

      for (const agent of this.selectedAgents) {
        await window.gres.wizard.setContext("selectedAgent", agent);
        const configResult = await window.gres.wizard.runSectionById("WZ-003");

        if (configResult.pass) {
          successCount++;
        } else if (configResult.code === "ERR_PARSE_ERROR") {
          const repaired = await this.handleConfigError(agent, configResult.message);
          if (repaired) successCount++;
          else errorCount++;
        } else {
          errorCount++;
        }
      }

      this.sectionResults.set("WZ-003", { pass: successCount > 0, successCount, errorCount });

      if (errorCount > 0 && successCount === 0) {
        throw new Error(`Failed to configure all ${errorCount} agent(s)`);
      }

      setStepStatus("config", "success", `${successCount} agent(s)`);
      setProgress(100, "Installation complete!", null);

      await this.sleep(500);
      await this.checkForRunningAgents();

    } catch (err) {
      const failedStep = ["checksum", "path", "config"].find((s) => {
        const el = document.getElementById(`step-${s}`);
        return el?.classList.contains("active");
      });
      if (failedStep) setStepStatus(failedStep, "error", err.message);
      this.showError("Installation Failed", err.message);
    }
  },

  async handleConfigError(agent, error) {
    const choice = await this.showRepairDialog(agent, error);
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
          <h3>WZ-003 Config Parse Error</h3>
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
  // Step 4: Zombie Guard (WZ-007)
  // ========================================================================
  async checkForRunningAgents() {
    // Run WZ-007 through state machine
    const wz007Result = await window.gres.wizard.runSectionById("WZ-007");
    this.sectionResults.set("WZ-007", wz007Result);

    const ctx = await window.gres.wizard.getContext();
    const restartCheck = ctx.restartCheckResult;

    if (restartCheck?.running) {
      const running = {
        agentName: this.selectedAgents[0]?.name,
        processes: restartCheck.pids,
        restartMessage: this.selectedAgents[0]?.restartMessage,
        isCodex: this.selectedAgents[0]?.name?.toLowerCase().includes("codex"),
      };
      this.currentZombieAgent = running;

      document.getElementById("zombieAgent").textContent = running.agentName;
      document.getElementById("zombieProcess").textContent = running.processes?.join(", ") || running.agentName;

      const sectionBadge = document.getElementById("zombieSectionBadge");
      if (sectionBadge) sectionBadge.textContent = "WZ-007";

      const restartEl = document.getElementById("zombieRestart");
      if (restartEl) {
        restartEl.textContent = running.restartMessage || "Please close the application.";
      }

      const statusEl = document.getElementById("zombieStatus");
      const forceBtn = document.getElementById("btnForceKill");
      const skipBtn = document.getElementById("btnSkipZombie");

      if (running.isCodex) {
        statusEl.innerHTML = `<span class="section-badge">WZ-007</span> Restart your terminal/CLI session to apply changes.`;
        if (forceBtn) forceBtn.style.display = "none";
        if (skipBtn) skipBtn.style.display = "inline-block";
      } else {
        statusEl.innerHTML = `<span class="section-badge">WZ-007</span> Waiting for process to exit...`;
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
      statusEl.innerHTML = `<span class="section-badge">WZ-007</span> ${status.message || "Checking..."}`;
    });

    try {
      const result = await window.gres.zombie.waitForExit({
        agentName: this.currentZombieAgent.agentName,
        timeout: 120000,
        pollInterval: 3000,
      });

      window.gres.zombie.offStatus();

      if (result.exited) {
        statusEl.innerHTML = `<span class="section-badge success">WZ-007 PASSED</span> Process exited. Proceeding...`;
        this.sectionResults.set("WZ-007", { pass: true, code: "OK" });
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else if (result.timeout) {
        statusEl.innerHTML = `<span class="section-badge error">WZ-007</span> Timeout. Use Force Close or close manually.`;
      }
    } catch (err) {
      window.gres.zombie.offStatus();
      statusEl.innerHTML = `<span class="section-badge error">WZ-007 ERROR</span> ${err.message}`;
    }
  },

  skipZombie() {
    this.sectionResults.set("WZ-007", { pass: true, code: "OK_SKIPPED" });
    this.showStep("verify");
    this.runVerification();
  },

  async forceKill() {
    const statusEl = document.getElementById("zombieStatus");
    statusEl.innerHTML = `<span class="section-badge">WZ-007</span> Force closing...`;

    try {
      const result = await window.gres.zombie.forceKill(this.currentZombieAgent.agentName);

      if (result.success) {
        statusEl.innerHTML = `<span class="section-badge success">WZ-007 PASSED</span> Process closed. Proceeding...`;
        this.sectionResults.set("WZ-007", { pass: true, code: "OK" });
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else {
        statusEl.innerHTML = `<span class="section-badge error">WZ-007</span> ${result.error || "Failed to close process"}`;
      }
    } catch (err) {
      statusEl.innerHTML = `<span class="section-badge error">WZ-007 ERROR</span> ${err.message}`;
    }
  },

  // ========================================================================
  // Step 5: Verification (WZ-006 MCP Selftest)
  // ========================================================================
  async runVerification() {
    const sectionBadge = document.getElementById("verifySectionBadge");
    if (sectionBadge) sectionBadge.textContent = "WZ-006";

    const tests = [
      { id: "binary", sectionId: "WZ-005", name: "Binary Version", fn: () => window.gres.verify.binaryVersion() },
      { id: "path", sectionId: null, name: "PATH Check", fn: () => window.gres.verify.pathWhere() },
      { id: "mcp", sectionId: "WZ-006", name: "MCP Selftest", fn: () => window.gres.wizard.runSectionById("WZ-006") },
    ];

    let allPassed = true;

    for (const test of tests) {
      const stepEl = document.getElementById(`verify-${test.id}`);
      const detailEl = document.getElementById(`verify-${test.id}-detail`);
      const statusEl = stepEl.querySelector(".step-status");

      stepEl.className = "progress-step active";
      statusEl.innerHTML = "";
      detailEl.innerHTML = test.sectionId
        ? `<span class="section-badge">${test.sectionId}</span> Testing...`
        : "Testing...";

      try {
        const result = await test.fn();

        // Handle both old API and new section results
        const passed = result.success || result.pass;

        if (passed) {
          stepEl.className = "progress-step success";
          statusEl.innerHTML = "&#10003;";
          detailEl.innerHTML = test.sectionId
            ? `<span class="section-badge success">${test.sectionId} PASSED</span> ${result.message || result.version || "Passed"}`
            : (result.message || result.version || "Passed");

          if (test.sectionId) {
            this.sectionResults.set(test.sectionId, { pass: true, ...result });
          }
        } else {
          stepEl.className = "progress-step error";
          statusEl.innerHTML = "&#10007;";
          detailEl.innerHTML = test.sectionId
            ? `<span class="section-badge error">${test.sectionId} FAILED</span> ${result.error || result.message || "Failed"}`
            : (result.error || result.message || "Failed");
          allPassed = false;

          if (test.sectionId) {
            this.sectionResults.set(test.sectionId, { pass: false, ...result });
          }
        }
      } catch (err) {
        stepEl.className = "progress-step error";
        statusEl.innerHTML = "&#10007;";
        detailEl.innerHTML = test.sectionId
          ? `<span class="section-badge error">${test.sectionId} ERROR</span> ${err.message}`
          : err.message;
        allPassed = false;
      }

      await this.sleep(400);
    }

    const resultEl = document.getElementById("verifyResult");

    if (allPassed) {
      resultEl.innerHTML = `
        <div class="note text-success" style="margin-top: 20px;">
          <span class="section-badge success">ALL GATES PASSED</span>
          All verification tests passed!
        </div>
      `;
      await this.sleep(1500);
      this.showSuccess();
    } else {
      resultEl.innerHTML = `
        <div class="note text-error" style="margin-top: 20px;">
          <span class="section-badge error">GATE FAILED</span>
          Some tests failed. Please resolve before proceeding.
        </div>
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

    // Add success message about full path
    const successNote = document.getElementById("successNote");
    if (successNote) {
      successNote.innerHTML = `
        <div class="note text-success" style="margin-top: 12px;">
          Configured using full path. PATH not required.
        </div>
        <div class="note" style="margin-top: 8px;">
          Restart your AI agent to load MCP config.
        </div>
      `;
    }

    this.showStep("success");

    // Create desktop shortcut
    try {
      const result = await window.gres.install.createShortcut();
      if (result.success) {
        console.log("Desktop shortcut created:", result.path);
      }
    } catch (err) {
      console.warn("Shortcut creation failed:", err);
    }

    // Log wizard completion
    const summary = await window.gres.wizard.summary();
    console.log("Wizard Summary:", summary);
  },

  // ========================================================================
  // Actions
  // ========================================================================
  async launchDashboard() {
    try {
      await window.gres.install.openDashboard();
    } catch (err) {
      window.gres.util.openUrl("https://ajranjith.github.io/b2b-governance-action/onboarding/?status=ready");
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
    window.gres.install.openDocs();
  },

  async viewLogs() {
    const logInfo = await window.gres.wizard.logPath();
    window.gres.util.openPath(logInfo.path);
  },

  sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  },
};

document.addEventListener("DOMContentLoaded", () => {
  wizard.init();
});
