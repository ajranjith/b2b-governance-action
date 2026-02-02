# Installation Guide (ZIP + CLI) - MCP Engine v4.0.0

This guide validates the MCP server setup for Marketplace release v4.0.0.

## A) ZIP Install (No-coder / Simple)

1) Download the release ZIP:
- https://github.com/ajranjith/b2b-governance-action/releases/download/v4.0.0/gres-b2b.zip

2) Unzip to a folder:

Windows example:
```
%LOCALAPPDATA%\Programs\gres-b2b\
```

3) Run a quick health check:
```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

4) Configure MCP client (see Section D)

5) Verify MCP tools appear and respond (see Section E)

## A.1) Unified Setup Flow (Wizard + CLI)

Both the Wizard UI and the CLI run the same steps in the same order.

Interactive (recommended):
```
bin\windows-amd64\gres-b2b.exe setup
```

Non-interactive:
```
bin\windows-amd64\gres-b2b.exe setup --non-interactive --target <path> --mode brownfield --client cursor --action verify
```

Step order: S1 Select Target ? S2 Detect Agents ? S3 Connect ? S4 Validate ? S5 Run Action ? S6 Update HUD ? S7 Resume.


## B) CLI Install (Developer / Advanced)

### Option 1: Build from source

```
cd cli
go test ./...
go build ./...
```

### Option 2: Use release ZIP

Use the ZIP steps above and run the binary directly from `bin/<os-arch>/`.

## C) MCP Connection - Required Snippets

Use these JSON snippets in your MCP client configuration. Replace the command path with your local binary location.

### Claude Desktop\n\nConfig paths:\n- Windows: %APPDATA%\\Claude\\claude_desktop_config.json\n- macOS: ~/Library/Application Support/Claude/claude_desktop_config.json\n- Linux: ~/.config/Claude/claude_desktop_config.json"
# Installation Guide (ZIP + CLI) - MCP Engine v4.0.0

This guide validates the MCP server setup for Marketplace release v4.0.0.

## A) ZIP Install (No-coder / Simple)

1) Download the release ZIP:
- https://github.com/ajranjith/b2b-governance-action/releases/download/v4.0.0/gres-b2b.zip

2) Unzip to a folder:

Windows example:
```
%LOCALAPPDATA%\Programs\gres-b2b\
```

3) Run a quick health check:
```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

4) Configure MCP client (see Section D)

5) Verify MCP tools appear and respond (see Section E)

## A.1) Unified Setup Flow (Wizard + CLI)

Both the Wizard UI and the CLI run the same steps in the same order.

Interactive (recommended):
```
bin\windows-amd64\gres-b2b.exe setup
```

Non-interactive:
```
bin\windows-amd64\gres-b2b.exe setup --non-interactive --target <path> --mode brownfield --client cursor --action verify
```

Step order: S1 Select Target ? S2 Detect Agents ? S3 Connect ? S4 Validate ? S5 Run Action ? S6 Update HUD ? S7 Resume.


## B) CLI Install (Developer / Advanced)

### Option 1: Build from source

```
cd cli
go test ./...
go build ./...
```

### Option 2: Use release ZIP

Use the ZIP steps above and run the binary directly from `bin/<os-arch>/`.

## C) MCP Connection - Required Snippets

Use these JSON snippets in your MCP client configuration. Replace the command path with your local binary location.

### Claude Desktop (Windows example)

`%APPDATA%\Claude\claude_desktop_config.json`
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor (JSON example)

`%APPDATA%\Cursor\mcp.json` (Windows) or `~/.cursor/mcp.json` (macOS/Linux)
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Windsurf / VS Code MCP (JSON example)

`.vscode/mcp.json`
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Codex CLI (TOML example)

```toml
[mcp_servers.gresB2B]
command = "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe"
args = ["mcp", "serve"]
enabled = true
```

## D) MCP Connection Validation

### D1. Binary viability

```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

Expected output (short):
```
GRES B2B CLI - MCP Bridge for AI Agent Governance
Usage:
  gres-b2b --version              Show version information
  ...
  gres-b2b doctor                 Run prerequisite checks
```

```
Doctor status: DEGRADED
```

### D2. MCP self-test

```
bin\windows-amd64\gres-b2b.exe mcp selftest
```

Expected output (short):
```
GRES B2B MCP Self-Test
======================

[OK] Version: 1.0.0
[INFO] Using default config
[OK] JSON-RPC parsing
[OK] Response generation

All tests passed!
```

### D3. MCP tools list + call (JSON-RPC)

Expected output (short):
```
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"governance_check"}]}}
{"jsonrpc":"2.0","id":3,"result":{"content":[{"text":"Governance check passed for action: health_check"}]}}
```

## E) Command Test Matrix (Executed)

All commands below were executed against the repo fixtures.

### E1. Build & Unit Tests

```
cd cli
go test ./...
go build ./...
```

Result: PASS

### E2. Binary tests (local)

```
# scan + verify on fixture
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Verify output:
```
+--------------------------------------------------+
|           GRES B2B Governance Verify            |
+--------------------------------------------------+
|  Status: PASS                                    |
|  RED:    0                                       |
|  AMBER:  0                                       |
|  GREEN:  0                                       |
+--------------------------------------------------+
|  PASSED: RED=0, AMBER=0, GREEN=0                 |
+--------------------------------------------------+
```

Artifacts observed:
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/results.json`
- `.b2b/results.sarif`
- `.b2b/junit.xml`
- `.b2b/certificate.json`

### E3. Watch mode

```
bin\windows-amd64\gres-b2b.exe --watch <repoRoot>
```

Validated:
- `.b2b/report.json` updated on file touch
- `.b2b/hints.json` generated

### E4. Shadow mode

```
bin\windows-amd64\gres-b2b.exe --shadow --vectors vectors.yml <repoRoot>
```

Validated:
- `.b2b/parity-report.json` created

### E5. Fix (dry-run)

```
bin\windows-amd64\gres-b2b.exe --fix --dry-run
```

Validated:
- `.b2b/fix-plan.json`
- `.b2b/fix.patch`

### E6. Support bundle

```
bin\windows-amd64\gres-b2b.exe --support-bundle <repoRoot>
```

Validated:
- `.b2b/support-bundle_<timestamp>.zip`

### E7. Rollback

```
bin\windows-amd64\gres-b2b.exe --rollback --latest-green
```

Validated:
- Snapshot restore returned `rollback_restored=True`
- `.b2b/rollback.log` appended
- `.b2b/audit.log` appended

## F) ZIP Package Validation

Unzipped `gres-b2b.zip` into a clean folder and ran:

```
bin\windows-amd64\gres-b2b.exe doctor
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Outputs observed in `.b2b/`:
- `report.json`
- `report.html`
- `certificate.json`
- `results.sarif`
- `junit.xml`
- `results.json`

## G) Troubleshooting

### Tool not found / PATH issues
- Use full path: `C:\\...\\gres-b2b.exe`
- For PowerShell: `& "C:\\path\\gres-b2b.exe" --help`

### MCP protocol errors
- MCP server uses stdio JSON-RPC with Content-Length framing.
- Any logs go to stderr, not stdout.

### Missing config or artifacts
- `verify` requires `.b2b/results.json` from a prior `scan`.
- `doctor` writes `.b2b/doctor.json` and may show DEGRADED if optional items are missing.

### Windows path quoting examples

```
& "C:\\Users\\YOU\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe" mcp serve
```
=# Installation Guide (ZIP + CLI) - MCP Engine v4.0.0

This guide validates the MCP server setup for Marketplace release v4.0.0.

## A) ZIP Install (No-coder / Simple)

1) Download the release ZIP:
- https://github.com/ajranjith/b2b-governance-action/releases/download/v4.0.0/gres-b2b.zip

2) Unzip to a folder:

Windows example:
```
%LOCALAPPDATA%\Programs\gres-b2b\
```

3) Run a quick health check:
```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

4) Configure MCP client (see Section D)

5) Verify MCP tools appear and respond (see Section E)

## A.1) Unified Setup Flow (Wizard + CLI)

Both the Wizard UI and the CLI run the same steps in the same order.

Interactive (recommended):
```
bin\windows-amd64\gres-b2b.exe setup
```

Non-interactive:
```
bin\windows-amd64\gres-b2b.exe setup --non-interactive --target <path> --mode brownfield --client cursor --action verify
```

Step order: S1 Select Target ? S2 Detect Agents ? S3 Connect ? S4 Validate ? S5 Run Action ? S6 Update HUD ? S7 Resume.


## B) CLI Install (Developer / Advanced)

### Option 1: Build from source

```
cd cli
go test ./...
go build ./...
```

### Option 2: Use release ZIP

Use the ZIP steps above and run the binary directly from `bin/<os-arch>/`.

## C) MCP Connection - Required Snippets

Use these JSON snippets in your MCP client configuration. Replace the command path with your local binary location.

### Claude Desktop (Windows example)

`%APPDATA%\Claude\claude_desktop_config.json`
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor (JSON example)

`%APPDATA%\Cursor\mcp.json` (Windows) or `~/.cursor/mcp.json` (macOS/Linux)
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Windsurf / VS Code MCP (JSON example)

`.vscode/mcp.json`
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Codex CLI (TOML example)

```toml
[mcp_servers.gresB2B]
command = "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe"
args = ["mcp", "serve"]
enabled = true
```

## D) MCP Connection Validation

### D1. Binary viability

```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

Expected output (short):
```
GRES B2B CLI - MCP Bridge for AI Agent Governance
Usage:
  gres-b2b --version              Show version information
  ...
  gres-b2b doctor                 Run prerequisite checks
```

```
Doctor status: DEGRADED
```

### D2. MCP self-test

```
bin\windows-amd64\gres-b2b.exe mcp selftest
```

Expected output (short):
```
GRES B2B MCP Self-Test
======================

[OK] Version: 1.0.0
[INFO] Using default config
[OK] JSON-RPC parsing
[OK] Response generation

All tests passed!
```

### D3. MCP tools list + call (JSON-RPC)

Expected output (short):
```
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"governance_check"}]}}
{"jsonrpc":"2.0","id":3,"result":{"content":[{"text":"Governance check passed for action: health_check"}]}}
```

## E) Command Test Matrix (Executed)

All commands below were executed against the repo fixtures.

### E1. Build & Unit Tests

```
cd cli
go test ./...
go build ./...
```

Result: PASS

### E2. Binary tests (local)

```
# scan + verify on fixture
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Verify output:
```
+--------------------------------------------------+
|           GRES B2B Governance Verify            |
+--------------------------------------------------+
|  Status: PASS                                    |
|  RED:    0                                       |
|  AMBER:  0                                       |
|  GREEN:  0                                       |
+--------------------------------------------------+
|  PASSED: RED=0, AMBER=0, GREEN=0                 |
+--------------------------------------------------+
```

Artifacts observed:
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/results.json`
- `.b2b/results.sarif`
- `.b2b/junit.xml`
- `.b2b/certificate.json`

### E3. Watch mode

```
bin\windows-amd64\gres-b2b.exe --watch <repoRoot>
```

Validated:
- `.b2b/report.json` updated on file touch
- `.b2b/hints.json` generated

### E4. Shadow mode

```
bin\windows-amd64\gres-b2b.exe --shadow --vectors vectors.yml <repoRoot>
```

Validated:
- `.b2b/parity-report.json` created

### E5. Fix (dry-run)

```
bin\windows-amd64\gres-b2b.exe --fix --dry-run
```

Validated:
- `.b2b/fix-plan.json`
- `.b2b/fix.patch`

### E6. Support bundle

```
bin\windows-amd64\gres-b2b.exe --support-bundle <repoRoot>
```

Validated:
- `.b2b/support-bundle_<timestamp>.zip`

### E7. Rollback

```
bin\windows-amd64\gres-b2b.exe --rollback --latest-green
```

Validated:
- Snapshot restore returned `rollback_restored=True`
- `.b2b/rollback.log` appended
- `.b2b/audit.log` appended

## F) ZIP Package Validation

Unzipped `gres-b2b.zip` into a clean folder and ran:

```
bin\windows-amd64\gres-b2b.exe doctor
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Outputs observed in `.b2b/`:
- `report.json`
- `report.html`
- `certificate.json`
- `results.sarif`
- `junit.xml`
- `results.json`

## G) Troubleshooting

### Tool not found / PATH issues
- Use full path: `C:\\...\\gres-b2b.exe`
- For PowerShell: `& "C:\\path\\gres-b2b.exe" --help`

### MCP protocol errors
- MCP server uses stdio JSON-RPC with Content-Length framing.
- Any logs go to stderr, not stdout.

### Missing config or artifacts
- `verify` requires `.b2b/results.json` from a prior `scan`.
- `doctor` writes `.b2b/doctor.json` and may show DEGRADED if optional items are missing.

### Windows path quoting examples

```
& "C:\\Users\\YOU\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe" mcp serve
```
 -replace '### Cursor \(JSON example\)\n\n%APPDATA%\\Cursor\\mcp.json \(Windows\) or ~/.cursor/mcp.json \(macOS/Linux\)',
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor (JSON example)

`%APPDATA%\Cursor\mcp.json` (Windows) or `~/.cursor/mcp.json` (macOS/Linux)
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Windsurf / VS Code MCP (JSON example)

`.vscode/mcp.json`
```json
{
  "mcpServers": {
    "gres-b2b": {
      "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Codex CLI (TOML example)

```toml
[mcp_servers.gresB2B]
command = "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe"
args = ["mcp", "serve"]
enabled = true
```

## D) MCP Connection Validation

### D1. Binary viability

```
bin\windows-amd64\gres-b2b.exe --help
bin\windows-amd64\gres-b2b.exe doctor
```

Expected output (short):
```
GRES B2B CLI - MCP Bridge for AI Agent Governance
Usage:
  gres-b2b --version              Show version information
  ...
  gres-b2b doctor                 Run prerequisite checks
```

```
Doctor status: DEGRADED
```

### D2. MCP self-test

```
bin\windows-amd64\gres-b2b.exe mcp selftest
```

Expected output (short):
```
GRES B2B MCP Self-Test
======================

[OK] Version: 1.0.0
[INFO] Using default config
[OK] JSON-RPC parsing
[OK] Response generation

All tests passed!
```

### D3. MCP tools list + call (JSON-RPC)

Expected output (short):
```
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"governance_check"}]}}
{"jsonrpc":"2.0","id":3,"result":{"content":[{"text":"Governance check passed for action: health_check"}]}}
```

## E) Command Test Matrix (Executed)

All commands below were executed against the repo fixtures.

### E1. Build & Unit Tests

```
cd cli
go test ./...
go build ./...
```

Result: PASS

### E2. Binary tests (local)

```
# scan + verify on fixture
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Verify output:
```
+--------------------------------------------------+
|           GRES B2B Governance Verify            |
+--------------------------------------------------+
|  Status: PASS                                    |
|  RED:    0                                       |
|  AMBER:  0                                       |
|  GREEN:  0                                       |
+--------------------------------------------------+
|  PASSED: RED=0, AMBER=0, GREEN=0                 |
+--------------------------------------------------+
```

Artifacts observed:
- `.b2b/report.json`
- `.b2b/report.html`
- `.b2b/results.json`
- `.b2b/results.sarif`
- `.b2b/junit.xml`
- `.b2b/certificate.json`

### E3. Watch mode

```
bin\windows-amd64\gres-b2b.exe --watch <repoRoot>
```

Validated:
- `.b2b/report.json` updated on file touch
- `.b2b/hints.json` generated

### E4. Shadow mode

```
bin\windows-amd64\gres-b2b.exe --shadow --vectors vectors.yml <repoRoot>
```

Validated:
- `.b2b/parity-report.json` created

### E5. Fix (dry-run)

```
bin\windows-amd64\gres-b2b.exe --fix --dry-run
```

Validated:
- `.b2b/fix-plan.json`
- `.b2b/fix.patch`

### E6. Support bundle

```
bin\windows-amd64\gres-b2b.exe --support-bundle <repoRoot>
```

Validated:
- `.b2b/support-bundle_<timestamp>.zip`

### E7. Rollback

```
bin\windows-amd64\gres-b2b.exe --rollback --latest-green
```

Validated:
- Snapshot restore returned `rollback_restored=True`
- `.b2b/rollback.log` appended
- `.b2b/audit.log` appended

## F) ZIP Package Validation

Unzipped `gres-b2b.zip` into a clean folder and ran:

```
bin\windows-amd64\gres-b2b.exe doctor
bin\windows-amd64\gres-b2b.exe scan
bin\windows-amd64\gres-b2b.exe verify
```

Outputs observed in `.b2b/`:
- `report.json`
- `report.html`
- `certificate.json`
- `results.sarif`
- `junit.xml`
- `results.json`

## G) Troubleshooting

### Tool not found / PATH issues
- Use full path: `C:\\...\\gres-b2b.exe`
- For PowerShell: `& "C:\\path\\gres-b2b.exe" --help`

### MCP protocol errors
- MCP server uses stdio JSON-RPC with Content-Length framing.
- Any logs go to stderr, not stdout.

### Missing config or artifacts
- `verify` requires `.b2b/results.json` from a prior `scan`.
- `doctor` writes `.b2b/doctor.json` and may show DEGRADED if optional items are missing.

### Windows path quoting examples

```
& "C:\\Users\\YOU\\AppData\\Local\\Programs\\gres-b2b\\bin\\windows-amd64\\gres-b2b.exe" mcp serve
```

