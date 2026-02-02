# gres-b2b CLI Commands Reference

This document lists the supported commands for the `gres-b2b` binary as shipped in v4.0.0.

## Overview

```
gres-b2b <command> [flags]
```

## Setup + Connect Flow

### `setup` - Interactive setup

```bash
gres-b2b setup
```

### `setup --non-interactive` - Non-interactive setup

```bash
gres-b2b setup --non-interactive --target <path> --mode brownfield --client cursor --action scan
```

### `target` - Select scan target

```bash
gres-b2b target --target <path>
gres-b2b target --repo <url> --ref <ref> --subdir <path>
```

### `classify` - Greenfield or brownfield

```bash
gres-b2b classify --mode greenfield
gres-b2b classify --mode brownfield
```

### `detect-agents` - Detect installed AI agents

```bash
gres-b2b detect-agents
```

### `connect-agent` - Write MCP config for one or all agents

```bash
gres-b2b connect-agent --client cursor
gres-b2b connect-agent --all
```

### `validate-agent` - Validate MCP config + binary

```bash
gres-b2b validate-agent --client cursor
gres-b2b validate-agent --all
```

## Core Actions

### `scan` - Governance scan

```bash
gres-b2b scan
```

**Outputs:**
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/results.json`

---

### `verify` - Governance verify with gating

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

### `watch` - Watch and rescan

```bash
gres-b2b watch
```

---

### `shadow` - Shadow parity

```bash
gres-b2b shadow --vectors <file.yml>
```

---

### `fix` - Auto-heal structural issues

```bash
gres-b2b fix --dry-run
gres-b2b fix --apply
```

---

### `fix-loop` - Scan -> Fix -> Rescan loop

```bash
gres-b2b fix-loop --max-fix-attempts 3
```

---

### `support-bundle` - Create support bundle zip

```bash
gres-b2b support-bundle <repoRoot>
```

---

### `rollback` - Restore last GREEN snapshot

```bash
gres-b2b rollback --latest-green
gres-b2b rollback --to <UTC_YYYYMMDD_HHMMSS>
```

---

### `doctor` - Prerequisite check

```bash
gres-b2b doctor
```

**Outputs:**
- `.b2b/doctor.json`

## MCP Server

### `mcp serve` - MCP server (stdio JSON-RPC)

```bash
gres-b2b mcp serve
```

### `mcp selftest` - MCP self-test

```bash
gres-b2b mcp selftest
```
