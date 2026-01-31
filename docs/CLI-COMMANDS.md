# gres-b2b CLI Commands Reference

This document lists the supported commands for the `gres-b2b` binary as shipped in v4.0.0.

## Overview

```
gres-b2b <command> [flags]
```

## Core Commands

### `verify` - Governance verify with gating

Runs the verification pipeline and applies gating rules.

Note: erify requires .b2b/results.json from a prior scan.

```bash
gres-b2b verify
```

**Outputs:**
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/certificate.json`
- `.b2b/results.sarif`
- `.b2b/junit.xml`

**Exit codes:**
- `0` - PASS
- `1` - FAIL (violations or caps)

---

### `scan` - Governance scan

Runs a scan and produces reports (no gating).

```bash
gres-b2b scan
```

**Outputs:**
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/results.sarif`
- `.b2b/junit.xml`

---

### `doctor` - Prerequisite check

Checks prerequisites and writes a doctor report.

```bash
gres-b2b doctor
```

**Outputs:**
- `.b2b/doctor.json`

---

## Modes and Utilities

### `mcp serve` - MCP server (stdio JSON-RPC)

```bash
gres-b2b mcp serve
```

### `mcp selftest` - MCP self-test

```bash
gres-b2b mcp selftest
```

---

### `--watch <path>` - Watch mode

Watches a workspace and re-runs scans on changes.

```bash
gres-b2b --watch <repoRoot>
```

Outputs on each run:
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/hints.json`
- `.b2b/history/*.report.json`

---

### `--shadow --vectors <file.yml> <repoRoot>` - Shadow parity

```bash
gres-b2b --shadow --vectors vectors.yml <repoRoot>
```

**Outputs:**
- `.b2b/parity-report.json`

---

### `--fix [--dry-run]` - Auto-heal (structural only)

```bash
gres-b2b --fix --dry-run
```

**Outputs:**
- `.b2b/fix-plan.json`
- `.b2b/fix.patch`

---

### `--support-bundle <repoRoot>` - Support bundle

```bash
gres-b2b --support-bundle <repoRoot>
```

**Outputs:**
- `.b2b/support-bundle_<timestamp>.zip`

---

### `--rollback --latest-green` / `--rollback --to <UTC_YYYYMMDD_HHMMSS>`

```bash
gres-b2b --rollback --latest-green
gres-b2b --rollback --to 20260131_004439
```

---

### `--verify-cert <path>` - Verify signed certificate

```bash
gres-b2b --verify-cert .b2b/certificate.json
```

---

### `--ingest-admin [incoming] [locked] [--resume]`

```bash
gres-b2b --ingest-admin ./incoming ./locked --resume
```

---

### `--setup` - Onboarding setup/resume

```bash
gres-b2b --setup
```

---

## Configuration

The CLI uses built-in defaults and supports optional overrides via `--config <path>`.

- `--config` expects a JSON file matching the internal config schema.
- Verification gating reads `.b2b/config.yml` (YAML) for `fail_on_red`, `allow_amber`, `max_red`, and `max_amber`.

## MCP Server Mode

The MCP server operates over stdio using JSON-RPC with Content-Length framing.

**JSON config example (Claude/Cursor/Windsurf):**
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "gres-b2b",
      "args": ["mcp", "serve"]
    }
  }
}
```

**TOML example (Codex):**
```toml
[mcp_servers.gresB2B]
command = "gres-b2b"
args = ["mcp", "serve"]
enabled = true
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `governance_check` | Validate a governance action (input: `action`) |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error or threshold exceeded |
| `2` | Invalid arguments |
| `3` | Configuration error |

