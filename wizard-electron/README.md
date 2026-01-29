# GRES B2B Setup Wizard (Electron)

> **This is "Option 1 — Download & Install Wizard"** from the [main documentation](../README.md#get-started).

Electron-based Windows installer with debuggable UI and reliable state validation.

## Download

[**Download gres-b2b-setup.exe →**](https://github.com/ajranjith/b2b-governance-action/releases/latest/download/gres-b2b-setup.exe)

---

## Architecture

```
wizard-electron/
├── main/
│   ├── main.js          # Electron main process
│   ├── preload.js       # IPC bridge (contextBridge)
│   └── services/
│       ├── mcp.js       # MCP selftest
│       ├── download.js  # GitHub Releases download + checksum
│       ├── path.js      # Registry PATH update + WM_SETTINGCHANGE
│       ├── verify.js    # Binary + PATH verification
│       └── scan.js      # Governance scans with streaming
├── renderer/
│   ├── index.html       # Wizard UI
│   ├── styles.css       # Styles
│   └── wizard.js        # State machine controller
└── package.json         # Electron + electron-builder config
```

## State-Validation Loop

The wizard runs 3 mandatory tests before completion:

### Test 1: Binary Integrity Test
- Downloads `gres-b2b.exe` from GitHub Releases
- Calculates SHA256 checksum
- Runs `gres-b2b --version` to verify executable

### Test 2: PATH Environment Test
- Updates User PATH via `HKCU\Environment` registry
- Broadcasts `WM_SETTINGCHANGE`
- Runs `where gres-b2b` in a fresh process to verify

### Test 3: MCP Connection Test
- Runs `gres-b2b mcp selftest`
- Protocol-level handshake verification
- Agent-agnostic (works with Claude, Cursor, VS Code, etc.)

All 3 tests must pass to reach the success screen.

## Development

### Prerequisites
- Node.js 18+
- npm or yarn

### Setup
```bash
cd wizard-electron
npm install
```

### Run in Development
```bash
npm start
```

Press `Ctrl+Shift+I` to open DevTools for debugging.

### Build Installer
```bash
npm run build:win
```

Output: `dist/gres-b2b-setup.exe`

## IPC API

The renderer communicates with main process via `window.gres.*`:

```javascript
// MCP operations
await window.gres.mcp.selftest()

// Installation
await window.gres.install.downloadBinary()
await window.gres.install.verifyChecksum()
await window.gres.install.applyPath()

// Verification
await window.gres.verify.binaryVersion()
await window.gres.verify.pathWhere()
await window.gres.verify.doctor()

// Config
await window.gres.config.write({ agent, projectPath, version })

// Utilities
await window.gres.util.openUrl(url)
await window.gres.util.getInstallPath()
```

## File Locations

| File | Location |
|------|----------|
| Binary | `%LOCALAPPDATA%\Programs\gres-b2b\gres-b2b.exe` |
| Config | `%LOCALAPPDATA%\Programs\gres-b2b\gres-b2b.config.json` |
| User Config | `%LOCALAPPDATA%\gres-b2b\config.toml` |

## Bundled Resources

The wizard bundles the CLI binary and configuration file in `extraResources`:

```
resources/
├── gres-b2b.exe           # CLI binary
└── gres-b2b.config.json   # Default configuration
```

Both files are installed to the install directory during setup.

## Why Electron?

The previous Go-based installer using `dlgs` had issues with:
- List selection dialog freezing on some Windows versions
- Limited debugging capabilities
- No DevTools for troubleshooting

Electron provides:
- Reliable native dialogs via Chromium
- Full DevTools for debugging stuck states
- Consistent UI across Windows versions
- Easy to update and iterate
