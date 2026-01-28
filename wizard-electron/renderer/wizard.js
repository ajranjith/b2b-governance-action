/**
 * GRES B2B Setup Wizard - State Machine Controller
 *
 * State-Validation Loop:
 * 1. Binary Integrity Test (download + checksum + --version)
 * 2. PATH Environment Test (update + where gres-b2b)
 * 3. MCP Connection Test (gres-b2b mcp selftest)
 */

const wizard = {
  currentStep: "welcome",
  steps: ["welcome", "agent", "project", "install", "verify", "complete"],
  data: {
    agent: "Claude Desktop",
    projectPath: "",
    version: "",
    binaryPath: "",
  },

  // Initialize wizard
  init() {
    this.showStep("welcome");

    // Set up agent selection listener
    document.querySelectorAll('input[name="agent"]').forEach((radio) => {
      radio.addEventListener("change", (e) => {
        this.data.agent = e.target.value;
      });
    });
  },

  // Show a specific step
  showStep(stepId) {
    document.querySelectorAll(".step").forEach((step) => {
      step.classList.remove("active");
    });

    const step = document.getElementById(`step-${stepId}`);
    if (step) {
      step.classList.add("active");
      this.currentStep = stepId;
    }

    // Run step-specific logic
    if (stepId === "install") {
      this.runInstallation();
    } else if (stepId === "verify") {
      this.runVerification();
    }
  },

  // Navigate to next step
  next() {
    const currentIndex = this.steps.indexOf(this.currentStep);
    if (currentIndex < this.steps.length - 1) {
      // Validate current step before proceeding
      if (this.currentStep === "project" && !this.data.projectPath) {
        alert("Please select a project folder.");
        return;
      }

      this.showStep(this.steps[currentIndex + 1]);
    }
  },

  // Navigate to previous step
  prev() {
    const currentIndex = this.steps.indexOf(this.currentStep);
    if (currentIndex > 0) {
      this.showStep(this.steps[currentIndex - 1]);
    }
  },

  // Folder selection
  async selectFolder() {
    // Use native folder dialog via Electron
    // For now, use a simple prompt (will be replaced with native dialog)
    const input = document.getElementById("projectPath");

    // Electron's dialog module needs to be called from main process
    // For now, allow manual entry
    const path = prompt("Enter project folder path:", input.value || "C:\\");
    if (path) {
      this.data.projectPath = path;
      input.value = path;
    }
  },

  // ========================================================================
  // Installation Step
  // ========================================================================
  async runInstallation() {
    const progressFill = document.getElementById("progressFill");
    const progressStatus = document.getElementById("progressStatus");
    const installLog = document.getElementById("installLog");

    const log = (msg) => {
      installLog.textContent += msg + "\n";
      installLog.scrollTop = installLog.scrollHeight;
    };

    const setProgress = (percent, status) => {
      progressFill.style.width = percent + "%";
      progressStatus.textContent = status;
    };

    try {
      // Step 1: Download binary
      setProgress(10, "Downloading gres-b2b...");
      log("Fetching latest release from GitHub...");

      const downloadResult = await window.gres.install.downloadBinary();
      if (!downloadResult.success) {
        throw new Error(downloadResult.error);
      }

      log(`Downloaded v${downloadResult.version}`);
      this.data.version = downloadResult.version;
      this.data.binaryPath = downloadResult.path;

      // Step 2: Verify checksum
      setProgress(40, "Verifying binary integrity...");
      log("Calculating checksum...");

      const checksumResult = await window.gres.install.verifyChecksum();
      if (!checksumResult.success) {
        throw new Error(checksumResult.error);
      }

      log(`Checksum: ${checksumResult.checksum.substring(0, 16)}...`);

      // Step 3: Update PATH
      setProgress(60, "Configuring system PATH...");
      log("Updating user PATH...");

      const pathResult = await window.gres.install.applyPath();
      if (!pathResult.success) {
        throw new Error(pathResult.error);
      }

      log(pathResult.alreadyInPath ? "PATH already configured" : "PATH updated successfully");

      // Step 4: Write config
      setProgress(80, "Writing configuration...");
      log("Creating config file...");

      const configResult = await window.gres.config.write({
        agent: this.data.agent,
        projectPath: this.data.projectPath,
        version: this.data.version,
      });

      if (!configResult.success) {
        throw new Error(configResult.error);
      }

      log(`Config: ${configResult.path}`);

      // Complete
      setProgress(100, "Installation complete!");
      log("\nReady for verification...");

      // Wait a moment then proceed to verification
      setTimeout(() => {
        this.showStep("verify");
      }, 1000);
    } catch (err) {
      log(`\nERROR: ${err.message}`);
      this.showError("Installation Failed", err.message);
    }
  },

  // ========================================================================
  // Verification Step - State Validation Loop
  // ========================================================================
  async runVerification() {
    const tests = [
      { id: "binary", name: "Binary Integrity Test", fn: this.testBinary.bind(this) },
      { id: "path", name: "PATH Environment Test", fn: this.testPath.bind(this) },
      { id: "mcp", name: "MCP Connection Test", fn: this.testMcp.bind(this) },
    ];

    let allPassed = true;

    for (const test of tests) {
      const testEl = document.getElementById(`test-${test.id}`);
      const statusEl = testEl.querySelector(".test-status");
      const detailEl = document.getElementById(`test-${test.id}-detail`);

      // Mark as running
      statusEl.className = "test-status running";
      statusEl.textContent = "●";
      detailEl.textContent = "Testing...";

      try {
        const result = await test.fn();

        if (result.success) {
          statusEl.className = "test-status success";
          statusEl.textContent = "✓";
          detailEl.textContent = result.message || "Passed";
        } else {
          statusEl.className = "test-status error";
          statusEl.textContent = "✗";
          detailEl.textContent = result.error || "Failed";
          allPassed = false;
        }
      } catch (err) {
        statusEl.className = "test-status error";
        statusEl.textContent = "✗";
        detailEl.textContent = err.message;
        allPassed = false;
      }

      // Small delay between tests
      await this.sleep(300);
    }

    // Show result
    const resultEl = document.getElementById("verifyResult");

    if (allPassed) {
      resultEl.className = "verify-result success";
      resultEl.innerHTML = "All tests passed! ✓";

      // Proceed to complete
      setTimeout(() => {
        this.showComplete();
      }, 1500);
    } else {
      resultEl.className = "verify-result error";
      resultEl.innerHTML = `
        Some tests failed. Please check the errors above.<br>
        <button class="btn btn-secondary" style="margin-top: 12px" onclick="wizard.retry()">Retry</button>
      `;
    }
  },

  // Test 1: Binary Integrity (--version)
  async testBinary() {
    const result = await window.gres.verify.binaryVersion();
    return result;
  },

  // Test 2: PATH Environment (where gres-b2b)
  async testPath() {
    const result = await window.gres.verify.pathWhere();
    return result;
  },

  // Test 3: MCP Connection (selftest)
  async testMcp() {
    const result = await window.gres.mcp.selftest();
    return result;
  },

  // ========================================================================
  // Complete Step
  // ========================================================================
  showComplete() {
    // Update summary
    document.getElementById("summaryBinary").textContent = this.data.binaryPath;
    document.getElementById("summaryVersion").textContent = "v" + this.data.version;
    document.getElementById("summaryAgent").textContent = this.data.agent;
    document.getElementById("summaryProject").textContent = this.data.projectPath || "(not set)";

    this.showStep("complete");
  },

  // ========================================================================
  // Error Handling
  // ========================================================================
  showError(title, message, details = "") {
    document.getElementById("errorMessage").textContent = message;
    document.getElementById("errorDetails").textContent = details;
    this.showStep("error");
  },

  retry() {
    // Reset verification tests UI
    ["binary", "path", "mcp"].forEach((id) => {
      const testEl = document.getElementById(`test-${id}`);
      const statusEl = testEl.querySelector(".test-status");
      const detailEl = document.getElementById(`test-${id}-detail`);
      statusEl.className = "test-status pending";
      statusEl.textContent = "●";
      detailEl.textContent = "";
    });

    document.getElementById("verifyResult").textContent = "";

    // Go back to install step
    this.showStep("install");
  },

  // ========================================================================
  // Utilities
  // ========================================================================
  sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  },

  openDocs() {
    window.gres.util.openUrl("https://ajranjith.github.io/b2b-governance-action/cli/");
  },

  finish() {
    window.gres.util.openUrl(
      "https://ajranjith.github.io/b2b-governance-action/onboarding/?status=ready"
    );
  },
};

// Initialize on load
document.addEventListener("DOMContentLoaded", () => {
  wizard.init();
});
