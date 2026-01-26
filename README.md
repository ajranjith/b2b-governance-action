# GRES B2B eCommerce Governance Framework + MCP Engine

[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-GRES%20B2B%20Governance-blue?logo=github)](https://github.com/marketplace/actions/gres-b2b-governance)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A governance framework + lightweight MCP tool for **B2B eCommerce portals** (dealer accounts, pricing, orders, backorders, fulfilment, ERP/WMS sync). It helps teams using AI-assisted development (Cursor / Claude / Windsurf / Codex) ship faster **without breaking architecture, permissions, or auditability** by generating a **RED / AMBER / GREEN readiness dashboard** and structured "what to fix next" guidance.

**Free to use. No license key required.**

---

## Why This Exists

B2B eCommerce changes are high-impact: a small mistake in **pricing**, **permissions**, or **order lifecycle** can cause major financial and operational damage. AI speeds delivery, but can introduce risky shortcuts (UI→DB coupling, missing policy checks, inconsistent patterns, weak audit trails).

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
| **Repeatable improvement loop** | Scan → fix → scan until GREEN |

---

## What's Included in the Framework

| Component | Description |
|-----------|-------------|
| **Repo skeleton + numbering** | Predictable structure so features/boundaries are discoverable |
| **ID-first architecture** | Stable IDs for key elements (API/SVC/DB/POL) so governance remains consistent |
| **ID registries** | Centralized catalogs (REF/API/SVC/DB/POL/ENT) for validation and traceability |
| **Identity Envelope** | Consistent identity across layers for audit and diagnostics |
| **Admin vs Dealer separation** | Reduces privilege leakage and keeps dealer-safe boundaries enforced |
| **Policy engine + wrappers** | Central permission checks—no hidden bypass routes |
| **Repository-only DB access** | Prevents direct DB use in UI/routes; keeps data access controlled |
| **DB wrapper + DB-IDs** | Every query is traceable and governed |
| **Dealer-safe DTO boundary** | Prevents leaking internal fields into dealer responses |
| **Audit writer** | Accountability for changes affecting orders, pricing, customer data |
| **Active probing worker** | Detects drift and broken invariants early |
| **Super User HUD** | Operational visibility for governance status (Admin-only) |
| **CI enforcement hooks** | Governance becomes measurable and repeatable in pipelines |

---

## What the MCP Tool Generates

- `.b2b/report.html` — HUD dashboard
- `.b2b/report.json` — Structured output
- `.b2b/certificate.json` — Verification certificate
- `.b2b/report.sarif.json` — SARIF for GitHub Security
- `.b2b/report.junit.xml` — JUnit for CI reporters

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
        uses: ajranjith/b2b-governance-action@v1
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
```

That's it! The action runs in `verify` mode by default.

---

## Modes

### Verify (Default)

One-time governance scan. Exits with code 1 if thresholds are exceeded.

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    mode: verify
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Watch

Continuous file watching (primarily for local development).

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    mode: watch
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Shadow

Compare outputs between legacy and new implementations for parity testing.

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    mode: shadow
    token: ${{ secrets.GITHUB_TOKEN }}
```

---

## Strictness

Control how violations affect your pipeline.

### Fail on Any RED

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    fail_on_red: true
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Fail on Any AMBER

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    allow_amber: false
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Strict Mode (No Violations Allowed)

```yaml
- uses: ajranjith/b2b-governance-action@v1
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
| `config` | Path to custom config file | _(none)_ |
| `fail_on_red` | Fail on any RED violation | `false` |
| `allow_amber` | Allow AMBER warnings | `true` |
| `sarif` | SARIF output path | `.b2b/report.sarif.json` |
| `junit` | JUnit output path | `.b2b/report.junit.xml` |
| `version` | Release version to use | `latest` |
| `repo` | Release repository | `ajranjith/b2b-governance-action` |
| `token` | GitHub token for downloads | _(none)_ |

---

## Outputs

After execution, reports are written to `.b2b/`:

| File | Description |
|------|-------------|
| `certificate.json` | Verification certificate with thresholds |
| `report.sarif.json` | SARIF format for GitHub Security tab |
| `report.junit.xml` | JUnit format for test reporters |
| `report.json` | Machine-readable findings |
| `report.html` | Human-readable dashboard |

---

## Examples

### Basic Verify

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Custom Config

```yaml
- uses: ajranjith/b2b-governance-action@v1
  with:
    config: .b2b/policy.yml
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Strict Gating

```yaml
- uses: ajranjith/b2b-governance-action@v1
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
        uses: ajranjith/b2b-governance-action@v1
        with:
          fail_on_red: true
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: .b2b/report.sarif.json

      - name: Upload JUnit
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: governance-junit
          path: .b2b/report.junit.xml
```

### Pinned Version

```yaml
- uses: ajranjith/b2b-governance-action@v1
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

## Installation Options (Local / MCP)

For local development with AI assistants, you have several options:

| Option | Best For | Effort |
|--------|----------|--------|
| **Setup Wizard** | Non-coders / Managers | ⭐ (1 min) |
| **Copy/Paste Setup** | Standard Developers | ⭐⭐ (2 mins) |
| **CLI Native** | Power Users / CI/CD | ⭐⭐⭐ (5 mins) |
| **Dev Container** | Enterprise Teams | ✨ (Zero-step) |

### Option 1 — Setup Wizard (⭐ 1 minute)

1. Download the release for your OS
2. Move it to a stable folder (e.g., `C:\Tools\...` or `~/tools/...`)
3. Open your project folder
4. Run the wizard (`--setup` or double-click on Windows)

✅ You should see: `.b2b/report.html` + `.b2b/report.json`

### Option 2 — Copy/Paste Setup (⭐⭐ 2 minutes)

1. Place the tool in a stable folder
2. Open your AI client settings → **Tools / MCP Servers**
3. Add **gres-b2b** and point it to the tool path
4. Save + restart client if needed

✅ Run a scan/doctor → confirm `.b2b/report.html`

### Option 3 — CLI Native (⭐⭐⭐ 5 minutes)

1. Build from source (or use the binary)
2. Run scans in CI on PRs
3. Upload `.b2b/report.html` as an artifact for reviewers

✅ Consistent enforcement across the whole team

### Option 4 — Dev Container (✨ Zero-step)

1. Add a `.devcontainer/` that includes the tool + env + defaults
2. User clicks "Reopen in Container"

✅ Ready-to-use governance with minimal setup effort

---

## What Success Looks Like

- Your repo contains `.b2b/report.html` and `.b2b/report.json`
- Stakeholders can open the HUD and see **RED / AMBER / GREEN**
- Teams fix issues and trend toward **GREEN** before release/merge

---

## Free Usage

**No license key required.** Anyone can download and run this action.

The action wrapper is MIT licensed. The `gres-b2b` engine license is defined in the [release repository](https://github.com/ajranjith/b2b-governance-action).

---

## Security

- SHA256 checksum verification before binary execution
- SBOMs (SPDX) published with each release
- No network calls except GitHub release download

---

## License

MIT (this action wrapper)

See [ajranjith/b2b-governance-action](https://github.com/ajranjith/b2b-governance-action) for engine license.
