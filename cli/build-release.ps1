# ============================================================================
# GRES B2B CLI Release Build Script (PowerShell)
#
# Builds the CLI and creates a release ZIP containing:
#   - gres-b2b.exe (standalone Windows binary)
#   - gres-b2b.config.json (default configuration)
#   - README.txt (quick start guide)
#   - checksums.txt (SHA256 checksums)
#
# Usage: .\build-release.ps1 [-Version "1.0.0"]
# ============================================================================

param(
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"

# Configuration
$BuildDate = Get-Date -Format "yyyy-MM-dd"
$OutputDir = "dist"
$StagingDir = "$OutputDir\staging"
$ZipName = "gres-b2b-v$Version-windows-amd64.zip"

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "GRES B2B CLI Release Builder" -ForegroundColor Cyan
Write-Host "Version: $Version" -ForegroundColor Cyan
Write-Host "Build Date: $BuildDate" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# Create output directories
if (!(Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir | Out-Null }
if (Test-Path $StagingDir) { Remove-Item -Recurse -Force $StagingDir }
New-Item -ItemType Directory -Path $StagingDir | Out-Null

# Step 1: Build the binary
Write-Host "[1/5] Building gres-b2b.exe..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
$ldflags = "-s -w -X main.Version=$Version -X main.BuildDate=$BuildDate"

$buildResult = & go build -ldflags $ldflags -o "$StagingDir\gres-b2b.exe" . 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Build failed!" -ForegroundColor Red
    Write-Host $buildResult
    exit 1
}
Write-Host "   Built: $StagingDir\gres-b2b.exe" -ForegroundColor Green

# Step 2: Copy config file
Write-Host "[2/5] Copying configuration..." -ForegroundColor Yellow
if (Test-Path "gres-b2b.config.json") {
    Copy-Item "gres-b2b.config.json" "$StagingDir\"
    Write-Host "   Copied: gres-b2b.config.json" -ForegroundColor Green
} else {
    Write-Host "   WARNING: gres-b2b.config.json not found" -ForegroundColor Yellow
}

# Step 3: Create README.txt
Write-Host "[3/5] Creating README.txt..." -ForegroundColor Yellow
$readme = @"
GRES B2B Governance Engine v$Version
=====================================

Quick Start:

1. Extract all files to: %LOCALAPPDATA%\Programs\gres-b2b\
   Or: C:\Program Files\GRES\B2B\

2. Run a health check:
   gres-b2b.exe doctor

3. Configure your AI agent's MCP settings to use:
   {
     "mcpServers": {
       "gres-b2b": {
         "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\gres-b2b.exe",
         "args": ["mcp", "serve"]
       }
     }
   }

Configuration:

  The gres-b2b.config.json file contains default settings.
  Place it in the same directory as the executable.

  Config search order:
  1. --config flag (explicit path)
  2. Same directory as executable
  3. %ProgramData%\GRES\B2B\gres-b2b.config.json
  4. Built-in defaults

Commands:

  gres-b2b --help       Show all commands
  gres-b2b --version    Show version info
  gres-b2b doctor       Run health check
  gres-b2b mcp          Start MCP server mode
  gres-b2b scan         Scan workspace for issues

Documentation: https://ajranjith.github.io/b2b-governance-action/

Build: v$Version ($BuildDate)
"@
$readme | Out-File -FilePath "$StagingDir\README.txt" -Encoding UTF8
Write-Host "   Created: README.txt" -ForegroundColor Green

# Step 4: Generate checksums
Write-Host "[4/5] Generating checksums..." -ForegroundColor Yellow
$checksums = @"
SHA256 Checksums for GRES B2B v$Version
Generated: $BuildDate

"@

$files = @("gres-b2b.exe", "gres-b2b.config.json")
foreach ($file in $files) {
    $filePath = "$StagingDir\$file"
    if (Test-Path $filePath) {
        $hash = (Get-FileHash -Path $filePath -Algorithm SHA256).Hash.ToLower()
        $checksums += "$hash  $file`n"
    }
}
$checksums | Out-File -FilePath "$StagingDir\checksums.txt" -Encoding UTF8
Write-Host "   Created: checksums.txt" -ForegroundColor Green

# Step 5: Create ZIP archive
Write-Host "[5/5] Creating ZIP archive..." -ForegroundColor Yellow
$zipPath = "$OutputDir\$ZipName"
if (Test-Path $zipPath) { Remove-Item $zipPath }

Compress-Archive -Path "$StagingDir\*" -DestinationPath $zipPath -Force
Write-Host "   Created: $ZipName" -ForegroundColor Green

# Show results
Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Build Complete!" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Output: $zipPath" -ForegroundColor White
Write-Host ""
Write-Host "Contents:" -ForegroundColor White
Get-ChildItem $StagingDir | ForEach-Object { Write-Host "  - $($_.Name)" }
Write-Host ""

$zipSize = (Get-Item $zipPath).Length
$zipSizeKB = [math]::Round($zipSize / 1024, 2)
Write-Host "ZIP Size: $zipSizeKB KB" -ForegroundColor White
Write-Host ""

# Cleanup staging
Remove-Item -Recurse -Force $StagingDir

Write-Host "Done!" -ForegroundColor Green

