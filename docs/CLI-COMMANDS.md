# gres-b2b CLI Commands Reference

This document specifies all CLI commands for the `gres-b2b` Go binary.

## Overview

```
gres-b2b [command] [flags]
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

### `--verify` - Run Governance Scan

Runs a one-time governance scan and outputs results.

```bash
gres-b2b --verify [--workspace=/path] [--config=/path]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--workspace` | `.` | Path to workspace to scan |
| `--config` | `.b2b/policy.yml` | Path to policy config |
| `--json` | `false` | Output JSON to stdout |

**Output files:**
- `.b2b/report.html` - Visual dashboard
- `.b2b/report.json` - Machine-readable results
- `.b2b/report.sarif.json` - GitHub Security format
- `.b2b/report.junit.xml` - CI/CD test format

**Exit codes:**
- `0` - Scan completed (may have warnings)
- `1` - Scan failed or threshold exceeded

---

### `--setup` - Interactive Live Scan

Opens a browser with live scan progress via SSE.

```bash
gres-b2b --setup [--port=8765] [--workspace=/path] [--no-browser]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8765` | SSE server port |
| `--workspace` | `.` | Path to workspace to scan |
| `--no-browser` | `false` | Don't auto-open browser |

**Behavior:**
1. Starts HTTP server with SSE endpoint
2. Opens `http://localhost:{port}/progress.html` in browser
3. Streams scan progress events
4. Shows final RED/AMBER/GREEN results

See [SETUP-MODE.md](SETUP-MODE.md) for SSE event specifications.

---

### `--fix` - Auto-Fix Loop

Runs scan → fix → re-scan loop until all checks pass or max iterations reached.

```bash
gres-b2b --fix [--max-iterations=10] [--workspace=/path]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--max-iterations` | `10` | Maximum fix iterations |
| `--workspace` | `.` | Path to workspace |
| `--dry-run` | `false` | Show fixes without applying |

**Stop conditions:**
- All checks are GREEN
- Max iterations reached
- No improvement after 3 consecutive attempts on same issue

**Output files:**
- `.b2b/report.html` - Final dashboard
- `.b2b/comparison.html` - Before/after diff
- `.b2b/rebuild.log` - Timing and iteration log

---

### `--watch` - Continuous Watch Mode

Watches for file changes and re-scans automatically.

```bash
gres-b2b --watch [--workspace=/path] [--debounce=1000]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--workspace` | `.` | Path to workspace |
| `--debounce` | `1000` | Debounce delay in ms |

---

### `--shadow` - Shadow Mode

Compares outputs between legacy and new implementations.

```bash
gres-b2b --shadow [--legacy=/path] [--new=/path]
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
| `scan(workspace?)` | Run governance scan |
| `fix(workspace?, maxIterations?)` | Run auto-fix loop |
| `report(workspace?)` | Get current report summary |
| `config(key?, value?)` | Get or set configuration |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GRES_B2B_WORKSPACE` | Default workspace path |
| `GRES_B2B_CONFIG` | Default policy config path |
| `GRES_B2B_PORT` | Default SSE server port |
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
gres-b2b --setup

# Run scan (CLI only)
gres-b2b --verify

# Auto-fix until GREEN (max 10 attempts)
gres-b2b --fix --max-iterations=10

# Watch mode for development
gres-b2b --watch

# Run scan on specific workspace
gres-b2b --verify --workspace=/path/to/project
```
