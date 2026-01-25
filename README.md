# GRES B2B Governance (GitHub Action)

[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-GRES%20B2B%20Governance-blue?logo=github)](https://github.com/marketplace/actions/gres-b2b-governance)

Runs the GRES B2B governance engine in CI by downloading official release binaries from `gres-digital/b2b-governance-mcp`.

## Features

- **Cross-platform**: Linux, macOS, Windows (amd64/arm64)
- **Secure**: SHA256 checksum verification before execution
- **Modes**: `--verify`, `--watch`, `--shadow`
- **CI-ready**: JUnit/SARIF output, cap-aware threshold gating

## Quickstart

```yaml
name: Governance

on:
  pull_request:
  push:
    branches: [main]

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run GRES B2B Governance
        uses: gres-digital/b2b-governance-action@v1
        with:
          version: latest
          args: "--verify --fail-on-red"
          token: ${{ secrets.GITHUB_TOKEN }}
```

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `version` | Release tag from MCP repo (e.g. `v4.0.0`) | `latest` |
| `args` | Arguments passed to gres-b2b | `--verify` |
| `repo` | Repo hosting releases | `gres-digital/b2b-governance-mcp` |
| `token` | GitHub token for release downloads | `` |

## Examples

### Basic Verify

```yaml
- uses: gres-digital/b2b-governance-action@v1
  with:
    args: "--verify"
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Strict Mode (Fail on Any RED)

```yaml
- uses: gres-digital/b2b-governance-action@v1
  with:
    args: "--verify --fail-on-red"
    token: ${{ secrets.GITHUB_TOKEN }}
```

### With Custom Config

```yaml
- uses: gres-digital/b2b-governance-action@v1
  with:
    args: "--verify --config .b2b/policy.yml"
    token: ${{ secrets.GITHUB_TOKEN }}
```

### Upload SARIF to GitHub Security

```yaml
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run GRES B2B Governance
        uses: gres-digital/b2b-governance-action@v1
        with:
          args: "--verify --fail-on-red"
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

## Threshold Gating

The action uses cap-aware threshold gating:

| Condition | Result |
|-----------|--------|
| `--fail-on-red` flag | Fail on any RED violation |
| `max_red` in config | Fail if `RedCount > max_red` |
| `--allow-amber=false` flag | Fail on any AMBER warning |
| `max_amber` in config | Fail if `AmberCount > max_amber` |

Exit code `1` on failure (gates CI pipeline).

## Outputs

After execution, find reports in `.b2b/`:

- `certificate.json` - Verification certificate with thresholds
- `report.sarif.json` - SARIF format for GitHub Security tab
- `report.junit.xml` - JUnit format for test reporters
- `report.json` / `report.html` - HUD dashboard

## Security

- Verifies `checksums.txt` before binary execution
- SBOMs (SPDX) published with each MCP release
- No network calls except GitHub release download

## License

MIT
