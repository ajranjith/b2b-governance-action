# GRES B2B Governance

[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-GRES%20B2B%20Governance-blue?logo=github)](https://github.com/marketplace/actions/gres-b2b-governance)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Run GRES B2B governance verification in your CI pipeline with cap-aware threshold gating.

**Free to use. No license key required.**

## What It Does

This action downloads the official `gres-b2b` binary and runs governance checks on your codebase:

- **Verify mode**: One-time scan with pass/fail gating
- **Watch mode**: Continuous monitoring (for local dev)
- **Shadow mode**: Compare legacy vs new implementations

Outputs JUnit and SARIF reports for CI integration and GitHub Security tab.

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
| `repo` | Release repository | `gres-digital/b2b-governance-mcp` |
| `token` | GitHub token for downloads | _(none)_ |

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

## Free Usage

**No license key required.** Anyone can download and run this action.

The action wrapper is MIT licensed. The `gres-b2b` engine license is defined in the [release repository](https://github.com/gres-digital/b2b-governance-mcp).

---

## Security

- SHA256 checksum verification before binary execution
- SBOMs (SPDX) published with each release
- No network calls except GitHub release download

---

## License

MIT (this action wrapper)

See [gres-digital/b2b-governance-mcp](https://github.com/gres-digital/b2b-governance-mcp) for engine license.
