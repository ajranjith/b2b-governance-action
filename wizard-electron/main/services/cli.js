const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");
const { BINARY_PATH } = require("./config");

function runCLI(args, cwd) {
  if (!fs.existsSync(BINARY_PATH)) {
    return { success: false, error: "Binary not found at " + BINARY_PATH };
  }
  const res = spawnSync(BINARY_PATH, args, {
    cwd: cwd || process.cwd(),
    windowsHide: true,
  });
  return {
    success: res.status === 0,
    code: res.status,
    stdout: (res.stdout || "").toString(),
    stderr: (res.stderr || "").toString(),
  };
}

function readJSON(filePath) {
  try {
    const data = fs.readFileSync(filePath, "utf8");
    return { success: true, data: JSON.parse(data) };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

module.exports = { runCLI, readJSON };
