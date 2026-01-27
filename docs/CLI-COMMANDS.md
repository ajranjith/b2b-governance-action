# gres-b2b CLI Commands Reference

This document specifies all CLI commands for the `gres-b2b` Go binary.

## Overview

```
gres-b2b <command> [flags]
```

## Commands

### `doctor` - Prerequisite Check

Checks prerequisites, paths, permissions, and prints actionable fixes.

```bash
gres-b2b doctor
```

**Output:**
```
GRES B2B Governance - Doctor

Prerequisites:
  ✅ Git available (git version 2.40.0)
  ✅ Go runtime (go1.21.0)
  ❌ Node.js not found (optional, for JS linting)

Paths:
  ✅ Binary path: /usr/local/bin/gres-b2b
  ✅ Config detected: ~/.codex/config.toml
  ✅ Workspace: /home/user/my-project

Permissions:
  ✅ Binary executable
  ✅ Workspace readable
  ✅ Output directory writable

Status: HEALTHY (2 optional items missing)
```

**Exit codes:**
- `0` - All required checks pass
- `1` - Required check failed

---

### `mcp selftest` - MCP Server Health Check

Starts the MCP server in test mode and verifies connectivity independently of any AI agent.

```bash
gres-b2b mcp selftest [--port=8765] [--verbose]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8765` | Port for test server |
| `--verbose` | `false` | Show detailed output |

**What it tests:**
1. Server starts successfully
2. MCP handshake completes
3. At least one tool is listed
4. A no-op tool call (e.g., `doctor()`) succeeds

**Output:**
```
GRES B2B Governance - MCP Self-Test

Testing MCP server health...

  ✅ Server started on port 8765
  ✅ MCP handshake completed
  ✅ 5 tools available: doctor, scan, fix, report, config
  ✅ Test call: doctor() returned OK

MCP Server: HEALTHY

Your AI agent should now be able to connect to this server.
```

**Exit codes:**
- `0` - All tests pass
- `1` - Server failed to start or test failed

---

### `scan` - Run Governance Scan

Runs a governance scan on the workspace.

```bash
gres-b2b scan [--live] [--workspace=/path] [--config=/path]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--live` | `false` | Open browser with live progress |
| `--workspace` | `.` | Path to workspace to scan |
| `--config` | `.b2b/policy.yml` | Path to policy config |
| `--json` | `false` | Output JSON to stdout |

**With `--live` flag:**
1. Starts local server on `127.0.0.1` (random free port)
2. Opens browser automatically to live progress page
3. Streams scan progress via SSE
4. Prints URL to terminal: `Live progress: http://127.0.0.1:<port>/progress`
5. When done, page shows "Open Full Report" button

**Output files:**
- `.b2b/report.html` - Visual dashboard
- `.b2b/report.json` - Machine-readable results
- `.b2b/report.sarif.json` - GitHub Security format
- `.b2b/report.junit.xml` - CI/CD test format

**Exit codes:**
- `0` - Scan completed (may have warnings)
- `1` - Scan failed or threshold exceeded

---

### `fix` - Auto-Fix Loop

Runs scan → fix → re-scan loop until all checks pass or max iterations reached.

```bash
gres-b2b fix [--live] [--max-iterations=10] [--workspace=/path]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--live` | `false` | Open browser with live progress |
| `--max-iterations` | `10` | Maximum fix iterations |
| `--workspace` | `.` | Path to workspace |
| `--dry-run` | `false` | Show fixes without applying |

**With `--live` flag:**
- Same as `scan --live` but shows iteration timeline
- Each iteration displays RED/AMBER/GREEN counts
- Progress updates in real-time

**Stop conditions:**
- All checks are GREEN
- Max iterations reached
- No improvement after 3 consecutive attempts on same issue

**Output files:**
- `.b2b/report.html` - Final dashboard
- `.b2b/comparison.html` - Before/after diff
- `.b2b/rebuild.log` - Timing and iteration log

---

### `watch` - Continuous Watch Mode

Watches for file changes and re-scans automatically.

```bash
gres-b2b watch [--workspace=/path] [--debounce=1000]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--workspace` | `.` | Path to workspace |
| `--debounce` | `1000` | Debounce delay in ms |

---

### `shadow` - Shadow Mode

Compares outputs between legacy and new implementations.

```bash
gres-b2b shadow [--legacy=/path] [--new=/path]
```

---

## MCP Server Mode

When invoked as an MCP server (by AI agents), the binary operates in stdio mode:

```bash
gres-b2b mcp
```

This is typically configured in agent config files:

**TOML (Codex):**
```toml
[mcp_servers.gresB2BGovernance]
command = "gres-b2b"
args = ["mcp"]
enabled = true
```

**JSON (Claude/Cursor):**
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "gres-b2b",
      "args": ["mcp"]
    }
  }
}
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `doctor()` | Check prerequisites and paths |
| `scan(workspace?, live?)` | Run governance scan |
| `fix(workspace?, maxIterations?, live?)` | Run auto-fix loop |
| `report(workspace?)` | Get current report summary |
| `config(key?, value?)` | Get or set configuration |

---

## The `--live` Flag

When you pass `--live` to `scan` or `fix`:

1. **Starts local HTTP server** on `127.0.0.1` with a random available port
2. **Opens browser automatically** to the progress page
3. **Streams events via SSE** at `/events`
4. **Prints URL to terminal** so you can copy it if browser doesn't open:
   ```
   Live progress: http://127.0.0.1:52341/progress
   ```
5. **Serves progress page locally** (not from GitHub Pages) so SSE works
6. **Shows "Open Full Report"** button when complete

The progress page is embedded in the binary and served locally - it does NOT load from GitHub Pages during scanning.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GRES_B2B_WORKSPACE` | Default workspace path |
| `GRES_B2B_CONFIG` | Default policy config path |
| `GRES_B2B_PORT` | Default server port for --live |
| `GRES_B2B_VERSION` | Override version for dev containers |

---

## Exit Codes Summary

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error or threshold exceeded |
| `2` | Invalid arguments |
| `3` | Configuration error |

---

## Examples

```bash
# Check prerequisites
gres-b2b doctor

# Verify MCP server works (agent-agnostic)
gres-b2b mcp selftest

# Run scan with live browser progress
gres-b2b scan --live

# Run scan (CLI only, no browser)
gres-b2b scan

# Auto-fix until GREEN (max 10 attempts) with live progress
gres-b2b fix --live --max-iterations=10

# Auto-fix (CLI only)
gres-b2b fix --max-iterations=5

# Watch mode for development
gres-b2b watch

# Run scan on specific workspace
gres-b2b scan --workspace=/path/to/project

# Run scan and output JSON
gres-b2b scan --json
```
