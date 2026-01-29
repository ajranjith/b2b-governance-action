/**
 * Wizard State Machine
 *
 * Enforces gating: UI cannot advance until current section's test passes.
 * Manages section flow, retry logic, and repair actions.
 */

const { SECTIONS, getSection, getSectionIndex } = require("./sections");
const logger = require("./wizard-logger");

/**
 * Wizard state
 */
const state = {
  currentSectionIndex: 0,
  sectionResults: new Map(), // sectionId -> result
  context: {}, // Shared context between sections
  services: null, // Injected service references
  isComplete: false,
  isFailed: false,
  failedSection: null,
};

/**
 * Initialize state machine with services
 */
function initialize(services) {
  state.services = services;
  state.context = {};
  state.sectionResults.clear();
  state.currentSectionIndex = 0;
  state.isComplete = false;
  state.isFailed = false;
  state.failedSection = null;

  logger.logWizardStart();
  logger.cleanupOldLogs(10);

  return getStatus();
}

/**
 * Get current status
 */
function getStatus() {
  const currentSection = SECTIONS[state.currentSectionIndex];
  const currentResult = state.sectionResults.get(currentSection?.id);

  return {
    currentSectionIndex: state.currentSectionIndex,
    currentSection: currentSection
      ? {
          id: currentSection.id,
          name: currentSection.name,
          description: currentSection.description,
          retryPolicy: currentSection.retryPolicy,
          repairActions: currentSection.repairActions,
        }
      : null,
    currentResult,
    totalSections: SECTIONS.length,
    completedSections: Array.from(state.sectionResults.entries())
      .filter(([_, r]) => r.pass)
      .map(([id]) => id),
    failedSections: Array.from(state.sectionResults.entries())
      .filter(([_, r]) => !r.pass)
      .map(([id]) => id),
    isComplete: state.isComplete,
    isFailed: state.isFailed,
    failedSection: state.failedSection,
    canAdvance: currentResult?.pass === true,
    canRetry: currentResult?.pass === false && (currentSection?.retryPolicy?.maxRetries || 0) > 0,
    canSkip: currentResult?.pass === false && currentSection?.retryPolicy?.canSkip === true,
  };
}

/**
 * Run preflight check (WZ-001) - must pass before wizard proceeds
 */
async function runPreflight() {
  const section = getSection("WZ-001");
  if (!section) {
    throw new Error("Preflight section not found");
  }

  logger.logSectionStart(section.id, section.name);

  try {
    await section.run(state.context);
    const result = await section.test(state.context);

    state.sectionResults.set(section.id, result);
    logger.logPreflightResult(result);

    if (!result.pass) {
      state.isFailed = true;
      state.failedSection = section.id;
    }

    return result;
  } catch (error) {
    logger.logError(section.id, error);

    const result = {
      pass: false,
      code: "ERR_EXCEPTION",
      message: error.message,
      evidence: { stack: error.stack },
    };

    state.sectionResults.set(section.id, result);
    state.isFailed = true;
    state.failedSection = section.id;

    return result;
  }
}

/**
 * Run current section
 */
async function runCurrentSection(opts = {}) {
  const section = SECTIONS[state.currentSectionIndex];
  if (!section) {
    return { pass: false, code: "ERR_NO_SECTION", message: "No section to run" };
  }

  logger.logSectionStart(section.id, section.name);

  try {
    await section.run(state.context, state.services, opts);
    const result = await section.test(state.context, state.services);

    state.sectionResults.set(section.id, result);
    logger.logSectionResult(section.id, section.name, result);

    return result;
  } catch (error) {
    logger.logError(section.id, error);

    const result = {
      pass: false,
      code: "ERR_EXCEPTION",
      message: error.message,
      evidence: { stack: error.stack },
    };

    state.sectionResults.set(section.id, result);
    return result;
  }
}

/**
 * Run a specific section by ID
 */
async function runSection(sectionId, opts = {}) {
  const section = getSection(sectionId);
  if (!section) {
    return { pass: false, code: "ERR_SECTION_NOT_FOUND", message: `Section ${sectionId} not found` };
  }

  logger.logSectionStart(section.id, section.name);

  try {
    await section.run(state.context, state.services, opts);
    const result = await section.test(state.context, state.services);

    state.sectionResults.set(section.id, result);
    logger.logSectionResult(section.id, section.name, result);

    return result;
  } catch (error) {
    logger.logError(section.id, error);

    const result = {
      pass: false,
      code: "ERR_EXCEPTION",
      message: error.message,
      evidence: { stack: error.stack },
    };

    state.sectionResults.set(section.id, result);
    return result;
  }
}

/**
 * Advance to next section (only if current passed)
 */
function advance() {
  const currentSection = SECTIONS[state.currentSectionIndex];
  const currentResult = state.sectionResults.get(currentSection?.id);

  if (!currentResult?.pass) {
    return {
      success: false,
      error: "Cannot advance - current section has not passed",
      status: getStatus(),
    };
  }

  if (state.currentSectionIndex >= SECTIONS.length - 1) {
    state.isComplete = true;
    logger.logWizardComplete(true, getSummary());
    return {
      success: true,
      complete: true,
      status: getStatus(),
    };
  }

  state.currentSectionIndex++;
  return {
    success: true,
    status: getStatus(),
  };
}

/**
 * Skip current section (only if allowed)
 */
function skip() {
  const currentSection = SECTIONS[state.currentSectionIndex];

  if (!currentSection?.retryPolicy?.canSkip) {
    return {
      success: false,
      error: "Cannot skip - section does not allow skipping",
      status: getStatus(),
    };
  }

  const skipResult = {
    pass: true,
    code: "OK_SKIPPED",
    message: "Section skipped by user",
    evidence: {},
  };

  state.sectionResults.set(currentSection.id, skipResult);
  logger.logSectionSkip(currentSection.id, currentSection.name);

  return advance();
}

/**
 * Get summary of all section results
 */
function getSummary() {
  const summary = {
    totalSections: SECTIONS.length,
    passed: 0,
    failed: 0,
    skipped: 0,
    sections: [],
  };

  for (const section of SECTIONS) {
    const result = state.sectionResults.get(section.id);
    const status = result ? (result.pass ? (result.code === "OK_SKIPPED" ? "SKIPPED" : "PASSED") : "FAILED") : "NOT_RUN";

    summary.sections.push({
      id: section.id,
      name: section.name,
      status,
      code: result?.code,
      message: result?.message,
    });

    if (status === "PASSED") summary.passed++;
    else if (status === "SKIPPED") summary.skipped++;
    else if (status === "FAILED") summary.failed++;
  }

  return summary;
}

/**
 * Get context value
 */
function getContext(key) {
  return key ? state.context[key] : state.context;
}

/**
 * Set context value
 */
function setContext(key, value) {
  state.context[key] = value;
}

/**
 * Get section result
 */
function getSectionResult(sectionId) {
  return state.sectionResults.get(sectionId);
}

/**
 * Reset to a specific section (for repair/retry)
 */
function resetToSection(sectionId) {
  const index = getSectionIndex(sectionId);
  if (index < 0) {
    return { success: false, error: "Section not found" };
  }

  state.currentSectionIndex = index;
  state.sectionResults.delete(sectionId);
  state.isFailed = false;
  state.failedSection = null;

  return { success: true, status: getStatus() };
}

module.exports = {
  initialize,
  getStatus,
  runPreflight,
  runCurrentSection,
  runSection,
  advance,
  skip,
  getSummary,
  getContext,
  setContext,
  getSectionResult,
  resetToSection,
};
