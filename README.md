# GRES B2B eCommerce Governance Framework + MCP Engine

[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-GRES%20B2B%20Governance-blue?logo=github)](https://github.com/marketplace/actions/gres-b2b-governance)
[![GitHub Release](https://img.shields.io/github/v/release/ajranjith/b2b-governance-action?logo=github&label=Release)](https://github.com/ajranjith/b2b-governance-action/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A governance framework + lightweight MCP tool for **B2B eCommerce portals** (dealer accounts, pricing, orders, backorders, fulfilment, ERP/WMS sync). It helps teams using AI-assisted development (Cursor / Claude / Windsurf / Codex) ship faster **without breaking architecture, permissions, or auditability** by generating a **RED / AMBER / GREEN readiness dashboard** and structured "what to fix next" guidance.

**Free to use. No license key required.**

---

## Get Started

Choose your setup method:

### Option 1 - Download CLI Package (Recommended)

Single ZIP with all supported binaries and documentation.

[**Download gres-b2b.zip ->**](https://github.com/ajranjith/b2b-governance-action/releases/download/v4.0.0/gres-b2b.zip)

**ZIP contents:**
- `bin/windows-amd64/gres-b2b.exe`
- `bin/linux-amd64/gres-b2b`
- `bin/darwin-arm64/gres-b2b`
- `docs/` (INSTALL/AGENT/ARCHITECTURE + CLI refs)
- `RELEASE_NOTES.md`
- `checksums.txt`
- `sbom.spdx.json`
**Installation (Windows example):**
1. Extract to `%LOCALAPPDATA%\\Programs\\gres-b2b\\`
2. (Optional) Add `%LOCALAPPDATA%\\Programs\\gres-b2b\\bin\\windows-amd64` to PATH
3. Run `gres-b2b.exe doctor` (optional) or the unified setup flow: `gres-b2b.exe setup`
   - Non-interactive: `gres-b2b.exe setup --non-interactive --target <path> --mode brownfield --client cursor --action verify`
   - Greenfield/Brownfield options appear in the `gres-b2b setup` wizard
   - MCP snippets and details: [docs/INSTALL.md](docs/INSTALL.md)

### Option 2 - CLI Setup Guide (Manual)

**For macOS/Linux, or if you prefer manual steps.**

- Full install and MCP setup guide: [docs/INSTALL.md](docs/INSTALL.md)
- CLI reference: [docs/CLI-COMMANDS.md](docs/CLI-COMMANDS.md)

---

## Why This Exists

B2B eCommerce changes are high-impact: a small mistake in **pricing**, **permissions**, or **order lifecycle** can cause major financial and operational damage. AI speeds delivery, but can introduce risky shortcuts (UI->DB coupling, missing policy checks, inconsistent patterns, weak audit trails).

**This framework exists to keep delivery fast and enterprise-safe** using repeatable guardrails and a simple readiness signal.

---

## Business Benefits

### For Managers / Owners

| Benefit | Description |
|---------|-------------|
| **Fewer incidents & refunds** | Catches structural risks early (permissions, boundary bypass, unsafe writes) |
| **Faster releases with confidence** | Reduces rework and PR churn by keeping builds aligned from day one |
| **Executive visibility** | Clear RED / AMBER / GREEN readiness without reading code |
| **Safer ERP/WMS integrations** | Encourages clean boundaries and controlled mutation paths |
| **Enterprise-friendly posture** | Supports support bundles, version stamping, and optional compliance artefacts |

### For Engineers / Delivery Teams

| Benefit | Description |
|---------|-------------|
| **AI guardrails built-in** | Agent receives structured instructions to build "the right way" |
| **Consistency across teams** | Fewer style wars, faster onboarding, fewer architectural forks |
| **Repeatable improvement loop** | Scan -> fix -> scan until GREEN |

---

## What's Included in the Framework

| Component | Description |
|-----------|-------------|
| **Repo skeleton + numbering** | Predictable structure so features/boundaries are discoverable |
| **ID-first architecture** | Stable IDs for key elements (API/SVC/DB/POL) so governance remains consistent |
| **ID registries** | Centralized catalogs (REF/API/SVC/DB/POL/ENT) for validation and traceability |
| **Identity Envelope** | Consistent identity across layers for audit and diagnostics |
| **Admin vs Dealer separation** | Reduces privilege leakage and keeps dealer-safe boundaries enforced |
| **Policy engine + wrappers** | Central permission checks - no hidden bypass routes |
| **Repository-only DB access** | Prevents direct DB use in UI/routes; keeps data access controlled |
| **DB wrapper + DB-IDs** | Every query is traceable and governed |
| **Dealer-safe DTO boundary** | Prevents leaking internal fields into dealer responses |
| **Audit writer** | Accountability for changes affecting orders, pricing, customer data |
| **Active probing worker** | Detects drift and broken invariants early |
| **Super User HUD** | Operational visibility for governance status (Admin-only) |
| **CI enforcement hooks** | Governance becomes measurable and repeatable in pipelines |

---


**HUD enforcement:** HUD will turn RED if the template performs a DB mutation without the mandatory audit wrapper/marker (Naked Mutation).

## What the MCP Tool Generates

- `.b2b/report.html` - HUD dashboard
- `.b2b/report.json` - Structured output
- `.b2b/certificate.json` - Verification certificate
- `.b2b/results.sarif` - SARIF for GitHub Security
- `.b2b/junit.xml` - JUnit for CI reporters

- `.b2b/api-routes.json` - API route inventory
- `.b2b/hints.json` - Live hints list
- `.b2b/audit.log` - Append-only audit log
- `.b2b/backups/<timestamp>/` - GREEN/PASS snapshots
- `.b2b/rollback.log` - Rollback history

**Produces RED / AMBER / GREEN readiness** with specific "fix next" guidance.

---

## Quickstart

```yaml
name: Governance

on: [push, pull_request]

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Governance Check
        uses: ajranjith/b2b-governance-action@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
```

That's it! The action runs in `verify` mode by default.

---

## Modes

### Verify (Default)

Runs scan then verify gating. Exits with code 1 if thresholds are exceeded.

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    mode: verify
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Watch

Continuous file watching (primarily for local development).

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    mode: watch
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Shadow

Compare outputs between legacy and new implementations for parity testing.

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    mode: shadow
    vectors: .b2b/vectors.yml
    token: ${{ secrets.GITHUB_TOKEN }}
```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    mode: shadow
    token: ${{ secrets.GITHUB_TOKEN }}
```

---

## Strictness

Control how violations affect your pipeline.

### Fail on Any RED

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    fail_on_red: true
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Fail on Any AMBER

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    allow_amber: false
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Strict Mode (No Violations Allowed)

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    fail_on_red: true
    allow_amber: false
    token: ${{ secrets.GITHUB_TOKEN }}
```

---

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `mode` | Operation mode: `verify` \| `watch` \| `shadow` | `verify` |
| `config` | Path to custom JSON config file | _(none)_ |
| `fail_on_red` | If set (true/false), writes fail_on_red in .b2b/config.yml | (empty) |
| `allow_amber` | If set (true/false), writes allow_amber in .b2b/config.yml | (empty) |
| `sarif` | SARIF output path | `.b2b/results.sarif` |
| `junit` | JUnit output path | `.b2b/junit.xml` |
| `version` | Release version to use | `latest` |
| `repo` | Release repository | `ajranjith/b2b-governance-action` |
| `token` | GitHub token for downloads | _(none)_ |
| `vectors` | Shadow vectors.yml (required when mode=shadow) | _(none)_ |

---

## Outputs

After execution, reports are written to `.b2b/`:

| File | Description |
|------|-------------|
| `certificate.json` | Verification certificate with thresholds |
| `results.sarif` | SARIF format for GitHub Security tab |
| `junit.xml` | JUnit format for test reporters |
| `report.json` | Machine-readable findings |
| `report.html` | Human-readable dashboard |
| `api-routes.json` | BFF API route inventory + API-ID mapping |
| `hints.json` | Live hints list (watch mode) |
| `audit.log` | Append-only audit log |
| `rollback.log` | Rollback history |

---

## Examples

### Basic Verify

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Custom Config

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    config: .b2b/config.json
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Strict Gating

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    fail_on_red: true
    allow_amber: false
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Upload SARIF to GitHub Security

```yaml
name: Governance

on: [push, pull_request]

permissions:
  contents: read
  security-events: write

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Governance Check
        uses: ajranjith/b2b-governance-action@v4
        with:
          fail_on_red: true
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: .b2b/results.sarif

      - name: Upload JUnit
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: governance-junit
          path: .b2b/junit.xml
```

### Pinned Version

```yaml
- uses: ajranjith/b2b-governance-action@v4
  with:
    version: v4.0.0
    token: ${{ secrets.GITHUB_TOKEN }}
```

---

## Threshold Gating

The action uses cap-aware threshold gating:

| Condition | Result |
|-----------|--------|
| `fail_on_red: true` | Fail on any RED violation (max_red=0) |
| `max_red` in config | Fail if RED count exceeds cap |
| `allow_amber: false` | Fail on any AMBER warning (max_amber=0) |
| `max_amber` in config | Fail if AMBER count exceeds cap |

**Precedence**: CLI flags > config values > defaults

Exit code `1` gates the CI pipeline on failure.

---

## Local Installation

See [**Get Started**](#get-started) at the top of this README for installation options.

For CI/CD integration, use the GitHub Action as shown in [Quickstart](#quickstart).

---

## CLI Commands

After installation, the `gres-b2b` CLI provides these commands:

```bash
# Show version
gres-b2b --version

# Launch interactive setup (agent → mode → target → scan/verify)
gres-b2b setup

# Start MCP server (used by AI agents)
gres-b2b mcp serve

# Run MCP self-test
gres-b2b mcp selftest

# Check prerequisites
gres-b2b doctor
```

Notes:
- `scan`, `verify`, `watch`, `shadow`, `fix`, and `fix-loop` require an explicit target (`--target` or `--repo`).
- From MCP inside Codex, use the `setup_status` tool to see the current setup step and next action.

The CLI implements the Model Context Protocol (MCP) over stdio, enabling AI agents to interact with governance tools through a standardized JSON-RPC 2.0 interface.

---

## What Success Looks Like

- Your repo contains `.b2b/report.html` and `.b2b/report.json`
- Stakeholders can open the HUD and see **RED / AMBER / GREEN**
- Teams fix issues and trend toward **GREEN** before release/merge

---

## Free Usage

**No license key required.** Anyone can download and run this action.

The action wrapper is MIT licensed. The `gres-b2b` engine license is defined in the [engine repository](https://github.com/ajranjith/B2B-Updated).

---

## Security

- CLI binary bundled inside the wizard - single trusted download
- SHA256 checksum verification before binary execution
- Windows Defender-aware installation (auto-unblocks Mark of the Web)
- SBOMs (SPDX) published with each release
- No network calls during installation (fully offline after download)

---

## License

MIT (this action wrapper)

See [LICENSE](https://github.com/ajranjith/b2b-governance-action/blob/main/LICENSE) for this action wrapper. Engine license is in [B2B-Updated](https://github.com/ajranjith/B2B-Updated).














