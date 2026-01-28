/**
 * Prebuild Script - Build gres-b2b CLI for bundling
 *
 * Compiles the Go CLI from ../cli and places it in resources/
 * for bundling with the Electron app.
 */

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const CLI_DIR = path.join(__dirname, "..", "..", "cli");
const OUTPUT_DIR = path.join(__dirname, "..", "resources");
const OUTPUT_PATH = path.join(OUTPUT_DIR, "gres-b2b.exe");

// Version to embed in binary
const VERSION = process.env.GRES_VERSION || "1.0.0";
const BUILD_DATE = new Date().toISOString().split("T")[0];

async function main() {
  console.log("Building gres-b2b CLI...");
  console.log(`  Version: ${VERSION}`);
  console.log(`  Build date: ${BUILD_DATE}`);

  // Ensure output directory exists
  if (!fs.existsSync(OUTPUT_DIR)) {
    fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  }

  // Check if CLI source exists
  const mainGo = path.join(CLI_DIR, "main.go");
  if (!fs.existsSync(mainGo)) {
    console.error(`CLI source not found at: ${mainGo}`);
    process.exit(1);
  }

  // Check if binary already exists and is recent (skip if < 1 hour old and source unchanged)
  if (fs.existsSync(OUTPUT_PATH)) {
    const binaryStats = fs.statSync(OUTPUT_PATH);
    const sourceStats = fs.statSync(mainGo);

    // If binary is newer than source, skip
    if (binaryStats.mtimeMs > sourceStats.mtimeMs) {
      const ageMs = Date.now() - binaryStats.mtimeMs;
      const oneHour = 60 * 60 * 1000;

      if (ageMs < oneHour) {
        console.log(`Binary is up to date (${Math.round(ageMs / 60000)} min old). Skipping build.`);
        return;
      }
    }
  }

  try {
    // Build for Windows amd64
    const ldflags = `-s -w -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}`;
    const cmd = `go build -ldflags "${ldflags}" -o "${OUTPUT_PATH}" .`;

    console.log(`Running: ${cmd}`);
    console.log(`Working directory: ${CLI_DIR}`);

    execSync(cmd, {
      cwd: CLI_DIR,
      env: {
        ...process.env,
        GOOS: "windows",
        GOARCH: "amd64",
        CGO_ENABLED: "0",
      },
      stdio: "inherit",
    });

    // Verify binary was created
    if (!fs.existsSync(OUTPUT_PATH)) {
      console.error("Build completed but binary not found!");
      process.exit(1);
    }

    const stats = fs.statSync(OUTPUT_PATH);
    console.log(`\nBuild successful!`);
    console.log(`  Output: ${OUTPUT_PATH}`);
    console.log(`  Size: ${(stats.size / 1024 / 1024).toFixed(2)} MB`);
  } catch (err) {
    console.error("Build failed:", err.message);

    // Check if Go is installed
    try {
      execSync("go version", { stdio: "pipe" });
    } catch {
      console.error("\nGo is not installed or not in PATH.");
      console.error("Please install Go from https://go.dev/dl/");
    }

    // Check if we have an existing binary we can use
    if (fs.existsSync(OUTPUT_PATH)) {
      console.log("\nUsing existing binary from previous build.");
    } else {
      process.exit(1);
    }
  }
}

main();
