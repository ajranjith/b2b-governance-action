/**
 * GRES B2B Setup Wizard - State Machine Controller
 *
 * State-Validation Loop:
 * 1. Binary Integrity Test (download + checksum + --version)
 * 2. PATH Environment Test (update + where gres-b2b)
 * 3. MCP Connection Test (gres-b2b mcp selftest)
 *
 * Flow: Welcome → Detect → Install → Zombie → Verify → Success
 */

const wizard = {
  currentStep: "welcome",
  steps: ["welcome", "detect", "install", "zombie", "verify", "success"],
  selectedAgents: [],
  data: {
    version: "",
    binaryPath: "",
    configPath: "",
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
      versionEl.textContent = "v1.0.0";
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
  // Step 2: Agent Detection
  // ========================================================================
  async runDetection() {
    const agentList = document.getElementById("agentList");
    const detectStatus = document.getElementById("detectStatus");
    const btnNext = document.getElementById("btnDetectNext");

    agentList.innerHTML = '<div class="note">Scanning...</div>';
    detectStatus.textContent = "Checking Registry and disk locations...";
    btnNext.disabled = true;

    try {
      const result = await window.gres.detect.agents();

      if (!result.success) {
        throw new Error(result.error || "Detection failed");
      }

      const agents = result.agents || [];

      if (agents.length === 0) {
        agentList.innerHTML = `
          <div class="note">No compatible AI agents found.</div>
          <p class="note" style="margin-top: 12px;">
            Supported: Claude Desktop, Cursor, VS Code (Windsurf), Codex CLI
          </p>
        `;
        detectStatus.textContent = "Install a supported AI agent and try again.";
        return;
      }

      // Render agent list with checkboxes
      agentList.innerHTML = agents
        .map(
          (agent, index) => `
          <label class="agent-item" data-agent="${agent.name}">
            <input type="checkbox" name="agent" value="${agent.name}" data-index="${index}">
            <div class="agent-info">
              <div class="agent-name">${agent.name}</div>
              <div class="agent-source">${agent.path || agent.configPath || "Detected"}</div>
            </div>
            <span class="agent-badge ${agent.source === "registry" ? "registry" : "disk"}">
              ${agent.source === "registry" ? "Registry" : "Disk"}
            </span>
          </label>
        `
        )
        .join("");

      // Store agents data for later use
      this.detectedAgents = agents;

      // Add checkbox listeners
      agentList.querySelectorAll('input[type="checkbox"]').forEach((checkbox) => {
        checkbox.addEventListener("change", () => this.updateAgentSelection());
      });

      // Add click handler to label for better UX
      agentList.querySelectorAll(".agent-item").forEach((item) => {
        item.addEventListener("click", (e) => {
          if (e.target.tagName !== "INPUT") {
            const checkbox = item.querySelector('input[type="checkbox"]');
            checkbox.checked = !checkbox.checked;
            this.updateAgentSelection();
          }
        });
      });

      detectStatus.textContent = `Found ${agents.length} agent(s). Select the ones to configure.`;

      // Auto-select first agent
      const firstCheckbox = agentList.querySelector('input[type="checkbox"]');
      if (firstCheckbox) {
        firstCheckbox.checked = true;
        this.updateAgentSelection();
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

    // Update visual selection
    document.querySelectorAll(".agent-item").forEach((item) => {
      const checkbox = item.querySelector('input[type="checkbox"]');
      item.classList.toggle("selected", checkbox.checked);
    });

    // Enable/disable next button
    const btnNext = document.getElementById("btnDetectNext");
    btnNext.disabled = this.selectedAgents.length === 0;
  },

  selectAgents() {
    if (this.selectedAgents.length === 0) {
      return;
    }

    this.showStep("install");
    this.runInstallation();
  },

  // ========================================================================
  // Step 3: Installation
  // ========================================================================
  async runInstallation() {
    const steps = ["download", "checksum", "path", "config"];
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

      stepEl.className = "progress-step " + status;
      if (detail) detailEl.textContent = detail;

      // Update status icon
      const statusEl = stepEl.querySelector(".step-status");
      if (status === "active") {
        statusEl.innerHTML = "";
      } else if (status === "success") {
        statusEl.innerHTML = "&#10003;";
      } else if (status === "error") {
        statusEl.innerHTML = "&#10007;";
      }
    };

    // Reset all steps
    steps.forEach((s) => setStepStatus(s, "", "Waiting..."));
    setProgress(0, "Starting...");

    try {
      // Step 1: Download binary
      setStepStatus("download", "active", "Downloading...");
      setProgress(5, "Downloading binary from GitHub...");

      // Set up progress listener
      window.gres.install.onProgress((percent) => {
        setProgress(5 + Math.floor(percent * 0.2), `Downloading... ${percent}%`);
      });

      const downloadResult = await window.gres.install.downloadBinary();
      window.gres.install.offProgress();

      if (!downloadResult.success) {
        throw new Error(downloadResult.error || "Download failed");
      }

      this.data.version = downloadResult.version;
      this.data.binaryPath = downloadResult.path;
      setStepStatus("download", "success", `v${downloadResult.version}`);

      // Step 2: Verify checksum
      setStepStatus("checksum", "active", "Calculating...");
      setProgress(30, "Verifying binary integrity...");

      const checksumResult = await window.gres.install.verifyChecksum({
        path: downloadResult.path,
      });

      if (!checksumResult.success) {
        throw new Error(checksumResult.error || "Checksum verification failed");
      }

      setStepStatus("checksum", "success", checksumResult.checksum.substring(0, 12) + "...");

      // Step 3: Configure PATH
      setStepStatus("path", "active", "Updating registry...");
      setProgress(50, "Configuring system PATH...");

      const pathResult = await window.gres.install.applyPath();

      if (!pathResult.success) {
        throw new Error(pathResult.error || "PATH configuration failed");
      }

      setStepStatus(
        "path",
        "success",
        pathResult.alreadyInPath ? "Already configured" : "Updated"
      );

      // Step 4: Write agent configs (non-destructive merge)
      setStepStatus("config", "active", "Merging configs...");
      setProgress(70, "Writing agent configurations...");

      let configPath = "";
      for (const agent of this.selectedAgents) {
        const configResult = await window.gres.config.write({
          agent: agent.name,
          configPath: agent.configPath,
          binaryPath: this.data.binaryPath,
        });

        if (!configResult.success) {
          throw new Error(`Config write failed for ${agent.name}: ${configResult.error}`);
        }

        configPath = configResult.path;

        // If backup was created, note it
        if (configResult.backup) {
          console.log(`Backup created: ${configResult.backup}`);
        }
      }

      this.data.configPath = configPath;
      setStepStatus("config", "success", `${this.selectedAgents.length} agent(s)`);
      setProgress(100, "Installation complete!");

      // Check if any agents are running (need zombie guard)
      await this.sleep(500);
      await this.checkForRunningAgents();
    } catch (err) {
      const failedStep = steps.find((s) => {
        const el = document.getElementById(`step-${s}`);
        return el.classList.contains("active");
      });

      if (failedStep) {
        setStepStatus(failedStep, "error", err.message);
      }

      this.showError("Installation Failed", err.message);
    }
  },

  // ========================================================================
  // Step 4: Zombie Guard
  // ========================================================================
  async checkForRunningAgents() {
    // Check if any selected agents are running
    let runningAgent = null;

    for (const agent of this.selectedAgents) {
      const result = await window.gres.zombie.check(agent.name);
      if (result.running) {
        runningAgent = { ...agent, processes: result.processes };
        break;
      }
    }

    if (runningAgent) {
      // Show zombie guard step
      this.currentZombieAgent = runningAgent;
      document.getElementById("zombieAgent").textContent = runningAgent.name;
      document.getElementById("zombieProcess").textContent = runningAgent.processes?.[0] || runningAgent.name;
      document.getElementById("zombieStatus").textContent = "Waiting for process to exit...";

      this.showStep("zombie");
      this.startZombieWatch();
    } else {
      // No running agents, proceed to verification
      this.showStep("verify");
      this.runVerification();
    }
  },

  async startZombieWatch() {
    const statusEl = document.getElementById("zombieStatus");

    // Set up status listener
    window.gres.zombie.onStatus((status) => {
      statusEl.textContent = status.message || "Checking...";
    });

    try {
      const result = await window.gres.zombie.waitForExit({
        agentName: this.currentZombieAgent.name,
        timeout: 120000, // 2 minutes
        pollInterval: 3000, // Check every 3 seconds
      });

      window.gres.zombie.offStatus();

      if (result.exited) {
        statusEl.textContent = "Agent closed. Proceeding...";
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else if (result.timeout) {
        statusEl.textContent = "Timeout waiting for agent. Use Force Close or close manually.";
      }
    } catch (err) {
      window.gres.zombie.offStatus();
      statusEl.textContent = `Error: ${err.message}`;
    }
  },

  async forceKill() {
    const statusEl = document.getElementById("zombieStatus");
    statusEl.textContent = "Force closing agent...";

    try {
      const result = await window.gres.zombie.forceKill(this.currentZombieAgent.name);

      if (result.success) {
        statusEl.textContent = "Agent closed. Proceeding...";
        await this.sleep(1000);
        this.showStep("verify");
        this.runVerification();
      } else {
        statusEl.textContent = result.error || "Failed to close agent";
      }
    } catch (err) {
      statusEl.textContent = `Error: ${err.message}`;
    }
  },

  // ========================================================================
  // Step 5: Verification - State Validation Loop
  // ========================================================================
  async runVerification() {
    const tests = [
      {
        id: "binary",
        name: "Binary Test",
        fn: () => window.gres.verify.binaryVersion(),
      },
      {
        id: "path",
        name: "PATH Test",
        fn: () => window.gres.verify.pathWhere(),
      },
      {
        id: "mcp",
        name: "MCP Handshake",
        fn: () => window.gres.mcp.selftest(),
      },
    ];

    let allPassed = true;

    for (const test of tests) {
      const stepEl = document.getElementById(`verify-${test.id}`);
      const detailEl = document.getElementById(`verify-${test.id}-detail`);
      const statusEl = stepEl.querySelector(".step-status");

      // Mark as active
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

    // Show result
    const resultEl = document.getElementById("verifyResult");

    if (allPassed) {
      resultEl.innerHTML = `
        <div class="note text-success" style="margin-top: 20px;">
          All tests passed!
        </div>
      `;

      // Proceed to success
      await this.sleep(1500);
      this.showSuccess();
    } else {
      resultEl.innerHTML = `
        <div class="note text-error" style="margin-top: 20px;">
          Some tests failed. Check the errors above.
        </div>
        <div class="step-actions">
          <button class="btn btn-secondary" onclick="wizard.retry()">Retry Tests</button>
          <button class="btn btn-primary" onclick="wizard.openDocs()">Get Help</button>
        </div>
      `;
    }
  },

  // ========================================================================
  // Step 6: Success
  // ========================================================================
  async showSuccess() {
    // Update success screen
    document.getElementById("successAgent").textContent =
      this.selectedAgents.map((a) => a.name).join(", ");
    document.getElementById("successVersion").textContent = "v" + this.data.version;
    document.getElementById("successConfig").textContent =
      this.data.configPath || this.selectedAgents[0]?.configPath || "-";

    this.showStep("success");

    // Create desktop shortcut
    try {
      await window.gres.util.createShortcut({
        name: "GRES B2B Dashboard",
        target: this.data.binaryPath,
        args: ["dashboard"],
      });
    } catch (err) {
      console.warn("Failed to create shortcut:", err);
    }
  },

  // ========================================================================
  // Error Handling
  // ========================================================================
  showError(title, message, details = "") {
    document.getElementById("errorTitle").textContent = title;
    document.getElementById("errorMessage").textContent = message;
    document.getElementById("errorDetails").textContent = details;
    this.showStep("error");
  },

  retry() {
    // Reset verification UI
    ["binary", "path", "mcp"].forEach((id) => {
      const stepEl = document.getElementById(`verify-${id}`);
      const detailEl = document.getElementById(`verify-${id}-detail`);
      const statusEl = stepEl.querySelector(".step-status");

      stepEl.className = "progress-step";
      statusEl.innerHTML = "";
      detailEl.textContent = "Waiting...";
    });

    document.getElementById("verifyResult").innerHTML = "";

    // Re-run verification
    this.runVerification();
  },

  // ========================================================================
  // External Actions
  // ========================================================================
  openDocs() {
    window.gres.util.openUrl("https://github.com/ajranjith/b2b-governance-action#readme");
  },

  async launchDashboard() {
    // Find an available port and launch dashboard
    try {
      const port = await window.gres.scan.findPort(3000);
      await window.gres.scan.start({ port });
      await window.gres.util.openUrl(`http://localhost:${port}`);
    } catch (err) {
      // Fallback: just try to open default port
      window.gres.util.openUrl("http://localhost:3000");
    }
  },

  // ========================================================================
  // Utilities
  // ========================================================================
  sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  },
};

// Initialize on load
document.addEventListener("DOMContentLoaded", () => {
  wizard.init();
});
