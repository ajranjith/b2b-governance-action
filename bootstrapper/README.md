# GRES B2B Bootstrapper

Native Windows GUI installer for gres-b2b. No PowerShell or admin rights required.

## Features

- **Wizard Flow**: Welcome → MCP Config → Target Selection → Installation → Finish
- **MCP Verification**: Tests MCP connection before proceeding (agent-agnostic)
- **Native Folder Picker**: Windows BrowseFolderDialog
- **Git URL Validation**: Validates GitHub repository URLs
- **Automatic Download**: Fetches latest release from GitHub
- **User PATH Update**: Adds to HKCU\Environment\Path (no admin needed)
- **Verified Scan**: Runs governance scan to confirm installation

## Build

### Prerequisites

- Go 1.22+
- Windows (Walk is Windows-only)

### Build Steps

```batch
cd bootstrapper
build.bat
```

This produces `gres-b2b-setup.exe` - a native Windows GUI application.

### Manual Build

```batch
REM Install rsrc tool
go install github.com/akavel/rsrc@latest

REM Generate Windows resources
rsrc -manifest rsrc.syso.manifest -o rsrc.syso

REM Build GUI executable (no console window)
go build -ldflags="-H windowsgui -s -w" -o gres-b2b-setup.exe .
```

## How It Works

1. **Welcome Screen**: Explains what the installer will do
2. **MCP Config**: User enters MCP host:port, clicks "Test MCP Connection"
   - Gates Next button until connection verified
   - Agent-agnostic TCP connect test
3. **Target Selection**: User picks local folder OR enters GitHub URL
   - Native Windows folder picker
   - Validates folder exists and is readable
   - Validates GitHub URL format
4. **Installation**: Background workflow runs:
   - Writes config to `%LOCALAPPDATA%\gres-b2b\config.json`
   - Downloads latest release from GitHub
   - Extracts to `%LOCALAPPDATA%\Programs\gres-b2b\gres-b2b.exe`
   - Adds install dir to User PATH
   - Broadcasts WM_SETTINGCHANGE for running terminals
   - Runs `gres-b2b doctor` to verify prerequisites
   - Runs `gres-b2b scan --workspace <path>` for verified scan
5. **Finish Screen**: Shows success, offers to open report

## File Locations

| File | Location |
|------|----------|
| Binary | `%LOCALAPPDATA%\Programs\gres-b2b\gres-b2b.exe` |
| Config | `%LOCALAPPDATA%\gres-b2b\config.json` |
| Reports | `<target>\.b2b\report.html` |

## Dependencies

- [lxn/walk](https://github.com/lxn/walk) - Windows GUI library
- [golang.org/x/sys/windows/registry](https://pkg.go.dev/golang.org/x/sys/windows/registry) - Registry access

## Notes

- **No Admin Required**: Uses user-writable paths only
- **No PowerShell**: Pure Go implementation
- **Agent-Agnostic**: MCP verification doesn't depend on specific AI host
- **Broadcast PATH Change**: New terminals pick up PATH without restart
