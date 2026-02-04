/**
 * GRES B2B Setup Wizard - ID-based State Machine Controller
 *
 * Section-based architecture with gate tests:
 * - Each section has a stable ID (S1 to WZ-010)
 * - "Next" is disabled until section test passes
 * - Preflight (S1) runs immediately on start
 * - All transitions logged to ndjson
 *
 * Sections:
 * S1: Preflight (OS/arch, permissions)
 * S2: Agent Detection
 * S3: Config Merge
 * S3: Install Binary
 * S4: Binary Proof
 * S5: MCP Selftest
 * S4: Restart Check
 * WZ-008-010: Scan (optional)
 */

const wizard = {
  currentStep: "preflight",
  steps: ["preflight", "welcome", "detect", "install", "zombie", "verify", "mode", "target", "scanfix", "success"],
  selectedAgents: [],
  detectedAgents: [],
  sectionResults: new Map(),
  data: {
    version: "",
    binaryPath: "",
    installDir: "",
    scanTarget: "",
    mode: "brownfield",
    client: "",
    connected: false,
  },

  // ========================================================================
  // Initialization with Preflight-First Boot
  // ========================================================================
  async init() {
    this.showStep("preflight");
    this.updateVersion();
    this.bindModeEvents();

    try {
      // Initialize wizard state machine
      await window.gres.wizard.initialize();

      // Run preflight (S1) - must pass before proceeding
      const preflightResult = await window.gres.wizard.preflight();
      this.sectionResults.set("S1", preflightResult);

      if (!preflightResult.pass) {
        this.showPreflightError(preflightResult);
        return;
      }

      // Preflight passed - resume if possible, otherwise show welcome
      await this.sleep(500);
      const resumed = await this.resumeFromSetupState();
      if (!resumed) {
        this.showStep("welcome");
        await this.loadRecentTarget();
      }

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
      <div class="section-badge error">S1 FAILED</div>
      <div class="error-message">${result.message}</div>
    `;

    if (result.evidence) {
      detailEl.innerHTML = `<pre>${JSON.stringify(result.evidence, null, 2)}</pre>`;
    }

    retryBtn.style.display = "inline-block";
  },

  async retryPreflight() {
    document.getElementById("preflightStatus").innerHTML = `
      <div class="section-badge">S1</div>
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

  async getHomeSetupPath() {
    const home = await window.gres.util.getHomeDir();
    if (!home) return "";
    return home + "\\.b2b\\setup.json";
  },

  normalizeClientName(name) {
    if (!name) return "generic";
    const lower = name.toLowerCase();
    if (lower.includes("codex")) return "codex";
    if (lower.includes("cursor")) return "cursor";
    if (lower.includes("claude")) return "claude";
    if (lower.includes("windsurf")) return "windsurf";
    return "generic";
  },

  async writeSetupState(step, extra = {}) {
    const setupPath = await this.getHomeSetupPath();
    if (!setupPath) return;
    const payload = {
      step,
      mode: this.data.mode || "",
      target: this.data.target || {},
      client: this.data.client || "",
      connected: !!this.data.connected,
      updatedAtUtc: new Date().toISOString(),
      ...extra,
    };
    await window.gres.util.writeJSON(setupPath, payload);
  },

  async resumeFromSetupState() {
    try {
      const setupPath = await this.getHomeSetupPath();
      if (!setupPath) return false;
      const res = await window.gres.cli.readJSON(setupPath);
      if (!res.success || !res.data) return false;
      const step = res.data.step;

      if (res.data.mode) {
        this.data.mode = res.data.mode;
      }
      if (res.data.target) {
        this.data.target = res.data.target;
        if (res.data.target.workspaceRoot) {
          this.data.scanTarget = res.data.target.workspaceRoot;
        } else if (res.data.target.path) {
          this.data.scanTarget = res.data.target.path;
        }
      }

      if (step === "mode_select") {
        this.showStep("mode");
        this.syncModeUI();
        return true;
      }
      if (step === "target_select") {
        this.showStep("target");
        await this.loadRecentTarget();
        this.updateTargetNext();
        return true;
      }
      if (step === "scan_run") {
        this.showStep("scanfix");
        await this.loadRecentTarget();
        return true;
      }
    } catch (_) {
      return false;
    }
    return false;
  },

  
  async runScan() {
    const target = this.data.scanTarget || "";
    if (!target) {
      this.showStep("target");
      this.updateTargetStatus("Select a target before running scan.");
      return;
    }
    const args = ["scan"];
    if (target) {
      args.push("--target", target);
    }
    await window.gres.cli.run(args, target || undefined);
    await this.refreshReport();
  },

  async runFixLoop() {
    const target = this.data.scanTarget || "";
    if (!target) {
      this.showStep("target");
      this.updateTargetStatus("Select a target before running fix loop.");
      return;
    }
    const args = ["fix-loop", "--max-fix-attempts", "3"];
    if (target) {
      args.push("--target", target);
    }
    await window.gres.cli.run(args, target || undefined);
    await this.refreshReport();
  },

  async runRescan() {
    const target = this.data.scanTarget || "";
    if (!target) {
      this.showStep("target");
      this.updateTargetStatus("Select a target before rescan.");
      return;
    }
    const args = ["scan"];
    if (target) {
      args.push("--target", target);
    }
    await window.gres.cli.run(args, target || undefined);
    await this.refreshReport();
  },

  async openHUD() {
    const target = this.data.scanTarget || "";
    if (!target) {
      this.showStep("target");
      this.updateTargetStatus("Select a target before opening HUD.");
      return;
    }
    const hudPath = target + "\\.b2b\\report.html";
    window.gres.util.openPath(hudPath);
  },

  async selfCheckSetup() {
    const statusEl = document.getElementById("targetStatus");
    const badge = document.getElementById("setupCheckBadge");
    const target = this.data.scanTarget || document.getElementById("targetPath").value.trim();
    if (!target) {
      if (statusEl) statusEl.textContent = "Set a target before self-check.";
      if (badge) badge.style.display = "none";
      return;
    }
    const setupPath = target + "\\.b2b\\setup.json";
    const res = await window.gres.cli.readJSON(setupPath);
    if (res.success && res.data && res.data.target) {
      if (badge) {
        badge.style.display = "inline-block";
        badge.className = "section-badge success";
        badge.textContent = "SAVED";
      }
      if (statusEl) statusEl.textContent = "setup.json verified";
    } else {
      if (badge) {
        badge.style.display = "inline-block";
        badge.className = "section-badge error";
        badge.textContent = "MISSING";
      }
      if (statusEl) statusEl.textContent = res.error || "setup.json not found";
    }
  },

  updateTargetStatus(message) {
    const statusEl = document.getElementById("targetStatus");
    if (statusEl && message) statusEl.textContent = message;
  },


  async setTarget() {
    const targetPath = document.getElementById("targetPath").value.trim();
    const repoUrl = document.getElementById("repoUrl").value.trim();
    let ref = document.getElementById("repoRef").value.trim();
    const subdir = document.getElementById("repoSubdir").value.trim();
    const statusEl = document.getElementById("targetStatus");

    let args = ["target"];
    if (repoUrl) {
      if (!ref) ref = "main";
      args.push("--repo", repoUrl);
      if (ref) args.push("--ref", ref);
      if (subdir) args.push("--subdir", subdir);
    } else if (targetPath) {
      args.push("--target", targetPath);
    } else {
      statusEl.textContent = "Please provide a local path or repo URL.";
      return;
    }

    const home = await window.gres.util.getHomeDir();
    const res = await window.gres.cli.run(args, home || targetPath || undefined);
    if (!res.success) {
      statusEl.textContent = res.stderr || res.error || "Target selection failed";
      return;
    }

    if (repoUrl) {
      const wsPath = (await window.gres.cli.readJSON(`\\.b2b\\workspaces.json`));
      if (wsPath.success && wsPath.data && wsPath.data.length) {
        const latest = wsPath.data[wsPath.data.length - 1];
        this.data.scanTarget = latest.path || "";
      }
    } else {
      this.data.scanTarget = targetPath;
    }

    statusEl.textContent = this.data.scanTarget ? `Target set: ${this.data.scanTarget}` : "Target set.";
    statusEl.classList.add("text-success");
    if (this.data.scanTarget) {
      const targetInfo = {
        type: repoUrl ? "git" : "local",
        path: targetPath || "",
        repoUrl: repoUrl || "",
        ref: ref || "",
        subdir: subdir || "",
        workspaceRoot: this.data.scanTarget,
      };
      this.data.target = targetInfo;
      await this.writeSetupToRepo(this.data.scanTarget, targetInfo);
      await this.writeSetupState("scan_run", { target: targetInfo });
      statusEl.textContent += " (saved)";
    }
    this.updateTargetNext();
  },

  async classifyProject() {
    const mode = this.data.mode || "brownfield";
    const statusEl = document.getElementById("targetStatus");
    if (!this.data.scanTarget) {
      statusEl.textContent = "Set a target before classify.";
      return;
    }
    const args = ["classify", "--mode", mode, "--target", this.data.scanTarget];
    const home = await window.gres.util.getHomeDir();
    const res = await window.gres.cli.run(args, home || this.data.scanTarget);
    if (!res.success) {
      statusEl.textContent = res.stderr || res.error || "Classify failed";
      return;
    }
    statusEl.textContent = `Classified: ${mode} (saved)`;
    statusEl.classList.add("text-success");
    await this.writeSetupToRepo(this.data.scanTarget, {
      type: "local",
      path: this.data.scanTarget,
      repoUrl: "",
      ref: "",
      subdir: "",
      workspaceRoot: this.data.scanTarget,
    });
    await this.writeSetupState("target_select", { mode });
  },

  async refreshReport() {
    if (!this.data.scanTarget) return;
    const reportPath = this.data.scanTarget + "\\.b2b\\report.json";
    const res = await window.gres.cli.readJSON(reportPath);
    if (!res.success) {
      const el = document.getElementById("scanFindings");
      if (el) el.textContent = res.error || "No report available";
      return;
    }

    const rep = res.data || {};
    document.getElementById("phase1Status").textContent = rep.phase1Status || "-";
    document.getElementById("phase2Status").textContent = rep.phase2Status || "-";
    document.getElementById("phase3Status").textContent = rep.phase3Status || "-";
    document.getElementById("phase4Status").textContent = rep.phase4Status || "-";

    const counts = { red: 0, amber: 0, green: 0 };
    (rep.rules || []).forEach((r) => {
      if (r.status === "FAIL") counts.red += 1;
      if (r.status === "WARN") counts.amber += 1;
      if (r.status === "PASS") counts.green += 1;
    });
    document.getElementById("redCount").textContent = counts.red;
    document.getElementById("amberCount").textContent = counts.amber;
    document.getElementById("greenCount").textContent = counts.green;

    const findingsEl = document.getElementById("scanFindings");
    const topRulesEl = document.getElementById("topRules");
    const fails = (rep.rules || []).filter((r) => r.status === "FAIL");
    if (findingsEl) {
      if (fails.length === 0) {
        findingsEl.textContent = "All checks passing.";
      } else {
        findingsEl.innerHTML = fails.slice(0, 5).map((f) => `? ${f.ruleId}: ${f.fixHint || "Fix required"}`).join("<br/>");
      }
    }
    if (topRulesEl) {
      const top = fails.slice(0, 10);
      if (top.length === 0) {
        topRulesEl.textContent = "";
      } else {
        topRulesEl.innerHTML = `<strong>Top Failing Rules</strong><br/>` + top.map((f) => `${f.ruleId} (${f.severity || ""})`).join("<br/>");
      }
    }
  },


  async loadRecentTarget() {
    try {
      const home = await window.gres.util.getHomeDir();
      if (!home) return;
      const setupPath = home + "\\.b2b\\setup.json";
      const workspacesPath = home + "\\.b2b\\workspaces.json";

      const setupRes = await window.gres.cli.readJSON(setupPath);
      if (setupRes.success && setupRes.data && setupRes.data.target) {
        const t = setupRes.data.target;
        this.data.target = t;
        if (t.path) document.getElementById("targetPath").value = t.path;
        if (t.repoUrl) document.getElementById("repoUrl").value = t.repoUrl;
        if (t.ref) document.getElementById("repoRef").value = t.ref;
        if (t.subdir) document.getElementById("repoSubdir").value = t.subdir;
        if (setupRes.data.mode) this.data.mode = setupRes.data.mode;
        if (t.workspaceRoot) this.data.scanTarget = t.workspaceRoot;
      }

      const wsRes = await window.gres.cli.readJSON(workspacesPath);
      if (!this.data.scanTarget && wsRes.success && Array.isArray(wsRes.data) && wsRes.data.length) {
        const latest = wsRes.data[wsRes.data.length - 1];
        if (latest.path) {
          this.data.scanTarget = latest.path;
          document.getElementById("targetPath").value = latest.path;
        }
      }

      if (this.data.scanTarget) {
        const statusEl = document.getElementById("targetStatus");
        if (statusEl) statusEl.textContent = `Target set: ${this.data.scanTarget}`;
      }
      this.syncModeUI();
      this.updateTargetNext();
    } catch (_) {
      // ignore
    }
  },
  async writeSetupToRepo(repoRoot, targetInfo) {
    if (!repoRoot) return;
    const setupPath = repoRoot + "\\.b2b\\setup.json";
    const payload = {
      version: "1.0",
      status: "IN_PROGRESS",
      step: "scan_run",
      target: targetInfo,
      mode: this.data.mode || "brownfield",
      client: this.data.client || "",
      connected: !!this.data.connected,
      updatedAtUtc: new Date().toISOString(),
      resumeAvailable: true,
    };
    const res = await window.gres.util.writeJSON(setupPath, payload);
    if (!res?.success) {
      const statusEl = document.getElementById("targetStatus");
      if (statusEl) statusEl.textContent = res?.error || "Failed to persist setup.json";
    }
  },

  syncModeUI() {
    const mode = this.data.mode || "";
    document.querySelectorAll('input[name="projectMode"]').forEach((input) => {
      input.checked = input.value === mode;
    });
    const btn = document.getElementById("btnModeNext");
    if (btn) btn.disabled = !mode;
  },

  bindModeEvents() {
    document.querySelectorAll('input[name="projectMode"]').forEach((input) => {
      input.addEventListener("change", () => {
        this.data.mode = input.value;
        const btn = document.getElementById("btnModeNext");
        if (btn) btn.disabled = false;
      });
    });
  },

  updateTargetNext() {
    const btn = document.getElementById("btnTargetNext");
    if (btn) btn.disabled = !this.data.scanTarget;
  },

  confirmMode() {
    const selected = document.querySelector('input[name="projectMode"]:checked');
    if (!selected) {
      const statusEl = document.getElementById("modeStatus");
      if (statusEl) statusEl.textContent = "Select a mode to continue.";
      return;
    }
    this.data.mode = selected.value;
    const statusEl = document.getElementById("modeStatus");
    if (statusEl) statusEl.textContent = `Mode selected: ${this.data.mode}`;
    this.writeSetupState("target_select", { mode: this.data.mode });
    this.showStep("target");
    this.updateTargetNext();
  },

  async confirmTarget() {
    await this.setTarget();
    if (!this.data.scanTarget) return;
    await this.classifyProject();
    this.showStep("scanfix");
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

  
  next() {
    const currentIndex = this.steps.indexOf(this.currentStep);
    if (currentIndex >= 0 && currentIndex < this.steps.length - 1) {
      this.showStep(this.steps[currentIndex + 1]);
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
  // Step 2: Agent Detection (S2)
  // ========================================================================
  async runDetection() {
    const agentList = document.getElementById("agentList");
    const detectStatus = document.getElementById("detectStatus");
    const btnNext = document.getElementById("btnDetectNext");
    const btnSyncAll = document.getElementById("btnSyncAll");
    const sectionBadge = document.getElementById("detectSectionBadge");

    if (sectionBadge) sectionBadge.textContent = "S2";

    agentList.innerHTML = '<div class="note">Scanning for config files...</div>';
    detectStatus.textContent = "Checking Codex TOML, Claude JSON, Cursor JSON, Windsurf JSON...";
    btnNext.disabled = true;
    if (btnSyncAll) btnSyncAll.style.display = "none";

    try {
      // Run S2 through state machine
      const result = await window.gres.wizard.runSectionById("S2");
      this.sectionResults.set("S2", result);

      // Get context to access detected agents
      const ctx = await window.gres.wizard.getContext();
      this.detectedAgents = ctx.agents || [];
      await this.writeSetupState("agent_detect");

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

    // Gate test: can only advance if at least one agent selected AND S2 passed
    const btnNext = document.getElementById("btnDetectNext");
    const wz002Result = this.sectionResults.get("S2");
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
    this.data.client = this.normalizeClientName(this.selectedAgents[0]?.name);
    this.data.connected = false;

    this.showStep("install");
    this.runInstallation();
  },

  // ========================================================================
  // Step 3: Installation (S3, S3, S4)
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

      // S3: Install Binary
      if (sectionBadge) sectionBadge.textContent = "S3";
      setStepStatus("checksum", "active", "Installing binary...");
      setProgress(5, "Installing gres-b2b...", "S3");

      window.gres.wizard.onProgress((percent) => {
        setProgress(5 + Math.floor(percent * 0.35), `Installing... ${percent}%`, "S3");
      });

      const wz004Result = await window.gres.wizard.runSectionById("S3");
      window.gres.wizard.offProgress();
      this.sectionResults.set("S3", wz004Result);

      if (!wz004Result.pass) {
        throw new Error(wz004Result.message || "Installation failed");
      }

      const ctx = await window.gres.wizard.getContext();
      this.data.version = ctx.downloadResult?.version || "unknown";
      setStepStatus("checksum", "success", `v${this.data.version}`);

      // S4: Binary Proof
      if (sectionBadge) sectionBadge.textContent = "S4";
      setStepStatus("path", "active", "Verifying binary...");
      setProgress(50, "Running binary verification...", "S4");

      const wz005Result = await window.gres.wizard.runSectionById("S4");
      this.sectionResults.set("S4", wz005Result);

      if (!wz005Result.pass) {
        throw new Error(wz005Result.message || "Binary verification failed");
      }
      setStepStatus("path", "success", "Verified");

      // S3: Config Merge
      if (sectionBadge) sectionBadge.textContent = "S3";
      setStepStatus("config", "active", "Configuring agents...");
      setProgress(70, "Writing agent configurations...", "S3");

      let successCount = 0;
      let errorCount = 0;

      for (const agent of this.selectedAgents) {
        await window.gres.wizard.setContext("selectedAgent", agent);
        const configResult = await window.gres.wizard.runSectionById("S3");

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

      this.sectionResults.set("S3", { pass: successCount > 0, successCount, errorCount });

      if (errorCount > 0 && successCount === 0) {
        throw new Error(`Failed to configure all ${errorCount} agent(s)`);
      }

      setStepStatus("config", "success", `${successCount} agent(s)`);
      setProgress(100, "Installation complete!", null);

      this.data.connected = true;
      await this.writeSetupState("mode_select", {
        client: this.data.client,
        connected: true,
      });

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
          <h3>S3 Config Parse Error</h3>
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
  // Step 4: Zombie Guard (S4)
  // ========================================================================
  async checkForRunningAgents() {
    // Run S4 through state machine
    const wz007Result = await window.gres.wizard.runSectionById("S4");
    this.sectionResults.set("S4", wz007Result);

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
      if (sectionBadge) sectionBadge.textContent = "S4";

      const restartEl = document.getElementById("zombieRestart");
      if (restartEl) {
        restartEl.textContent = running.restartMessage || "Please close the application.";
      }

      const statusEl = document.getElementById("zombieStatus");
      const forceBtn = document.getElementById("btnForceKill");
      const skipBtn = document.getElementById("btnSkipZombie");

      if (running.isCodex) {
        statusEl.innerHTML = `<span class="section-badge">S4</span> Restart your terminal/CLI session to apply changes.`;
        if (forceBtn) forceBtn.style.display = "none";
        if (skipBtn) skipBtn.style.display = "inline-block";
      } else {
        statusEl.innerHTML = `<span class="section-badge">S4</span> Waiting for process to exit...`;
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
      statusEl.innerHTML = `<span class="section-badge">S4</span> ${status.message || "Checking..."}`;
    });

    try {
      const result = await window.gres.zombie.waitForExit({
        agentName: this.currentZombieAgent.agentName,
        timeout: 120000,
        pollInterval: 3000,
      });

      window.gres.zombie.offStatus();

      if (result.exited) {
        statusEl.innerHTML = `<span class="section-badge success">S4 PASSED</span> Process exited. Proceeding...`;
        this.sectionResults.set("S4", { pass: true, code: "OK" });
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else if (result.timeout) {
        statusEl.innerHTML = `<span class="section-badge error">S4</span> Timeout. Use Force Close or close manually.`;
      }
    } catch (err) {
      window.gres.zombie.offStatus();
      statusEl.innerHTML = `<span class="section-badge error">S4 ERROR</span> ${err.message}`;
    }
  },

  skipZombie() {
    this.sectionResults.set("S4", { pass: true, code: "OK_SKIPPED" });
    this.showStep("verify");
    this.runVerification();
  },

  async forceKill() {
    const statusEl = document.getElementById("zombieStatus");
    statusEl.innerHTML = `<span class="section-badge">S4</span> Force closing...`;

    try {
      const result = await window.gres.zombie.forceKill(this.currentZombieAgent.agentName);

      if (result.success) {
        statusEl.innerHTML = `<span class="section-badge success">S4 PASSED</span> Process closed. Proceeding...`;
        this.sectionResults.set("S4", { pass: true, code: "OK" });
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else {
        statusEl.innerHTML = `<span class="section-badge error">S4</span> ${result.error || "Failed to close process"}`;
      }
    } catch (err) {
      statusEl.innerHTML = `<span class="section-badge error">S4 ERROR</span> ${err.message}`;
    }
  },

  // ========================================================================
  // Step 5: Verification (S5 MCP Selftest)
  // ========================================================================
  async runVerification() {
    const sectionBadge = document.getElementById("verifySectionBadge");
    if (sectionBadge) sectionBadge.textContent = "S5";

    const tests = [
      { id: "binary", sectionId: "S4", name: "Binary Version", fn: () => window.gres.verify.binaryVersion() },
      { id: "path", sectionId: null, name: "PATH Check", fn: () => window.gres.verify.pathWhere() },
      { id: "mcp", sectionId: "S5", name: "MCP Selftest", fn: () => window.gres.wizard.runSectionById("S5") },
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
      await this.sleep(800);
      this.showStep("mode");
      this.syncModeUI();
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
