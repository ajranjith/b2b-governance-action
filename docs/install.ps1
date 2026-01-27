#Requires -Version 5.1
<#
.SYNOPSIS
    Installs gres-b2b CLI tool for Windows.

.DESCRIPTION
    Downloads the latest gres-b2b release from GitHub and installs it to a
    user-writable folder. No admin rights required.

.PARAMETER Version
    Specific version to install (e.g., "1.0.0"). Defaults to "latest".

.PARAMETER InstallDir
    Installation directory. Defaults to "$env:LOCALAPPDATA\gres-b2b".

.EXAMPLE
    irm https://ajranjith.github.io/b2b-governance-action/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -Version "1.0.0"

.NOTES
    Repository: https://github.com/ajranjith/b2b-governance-action
#>

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\gres-b2b"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"  # Faster downloads

$Repo = "ajranjith/b2b-governance-action"
$BinaryName = "gres-b2b.exe"

function Write-Step {
    param([string]$Message)
    Write-Host "  -> " -NoNewline -ForegroundColor Cyan
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "  OK " -NoNewline -ForegroundColor Green
    Write-Host $Message
}

function Write-Fail {
    param([string]$Message)
    Write-Host "  FAIL " -NoNewline -ForegroundColor Red
    Write-Host $Message
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  GRES B2B Governance - Installer" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Step 1: Detect architecture
Write-Step "Detecting system architecture..."
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$OS = "windows"
Write-Success "Detected: $OS/$Arch"

# Step 2: Get latest release version if needed
if ($Version -eq "latest") {
    Write-Step "Fetching latest release version..."
    try {
        $ReleaseUrl = "https://api.github.com/repos/$Repo/releases/latest"
        $Release = Invoke-RestMethod -Uri $ReleaseUrl -UseBasicParsing
        $Version = $Release.tag_name -replace '^v', ''
        Write-Success "Latest version: $Version"
    } catch {
        Write-Fail "Could not fetch latest version: $_"
        exit 1
    }
} else {
    $Version = $Version -replace '^v', ''
    Write-Success "Using specified version: $Version"
}

# Step 3: Build download URL
# Expected archive name: gres-b2b_<version>_windows_amd64.zip
$ArchiveName = "gres-b2b_${Version}_${OS}_${Arch}.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/v$Version/$ArchiveName"
Write-Step "Download URL: $DownloadUrl"

# Step 4: Create install directory
Write-Step "Creating install directory..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}
Write-Success "Install directory: $InstallDir"

# Step 5: Download archive
$TempDir = Join-Path $env:TEMP "gres-b2b-install-$(Get-Random)"
$ArchivePath = Join-Path $TempDir $ArchiveName

Write-Step "Downloading $ArchiveName..."
try {
    New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing
    Write-Success "Downloaded to $ArchivePath"
} catch {
    Write-Fail "Download failed: $_"
    Write-Host ""
    Write-Host "  Possible causes:" -ForegroundColor Yellow
    Write-Host "    - Release v$Version does not exist"
    Write-Host "    - Network connectivity issue"
    Write-Host "    - GitHub rate limiting"
    Write-Host ""
    Write-Host "  Check releases at:" -ForegroundColor Yellow
    Write-Host "    https://github.com/$Repo/releases"
    exit 1
}

# Step 6: Extract archive
Write-Step "Extracting archive..."
try {
    Expand-Archive -Path $ArchivePath -DestinationPath $TempDir -Force

    # Find the binary (might be in root or subfolder)
    $ExtractedBinary = Get-ChildItem -Path $TempDir -Filter $BinaryName -Recurse | Select-Object -First 1
    if (-not $ExtractedBinary) {
        throw "Binary $BinaryName not found in archive"
    }

    # Copy to install directory
    $DestPath = Join-Path $InstallDir $BinaryName
    Copy-Item -Path $ExtractedBinary.FullName -Destination $DestPath -Force
    Write-Success "Installed to $DestPath"
} catch {
    Write-Fail "Extraction failed: $_"
    exit 1
} finally {
    # Cleanup temp files
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Step 7: Add to PATH (User scope, no admin needed)
Write-Step "Configuring PATH..."
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = "$InstallDir;$UserPath"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    Write-Success "Added $InstallDir to User PATH"
} else {
    Write-Success "Already in PATH"
}

# Also update current session PATH
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

# Step 8: Verify installation
Write-Step "Verifying installation..."
try {
    $BinaryPath = Join-Path $InstallDir $BinaryName
    $VersionOutput = & $BinaryPath --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Verified: $VersionOutput"
    } else {
        Write-Success "Binary installed (version check returned: $VersionOutput)"
    }
} catch {
    Write-Fail "Could not verify: $_"
    Write-Host "  The binary was installed but verification failed." -ForegroundColor Yellow
    Write-Host "  Try running: $BinaryPath --version" -ForegroundColor Yellow
}

# Done!
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Installation Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Binary:  " -NoNewline; Write-Host "$InstallDir\$BinaryName" -ForegroundColor Cyan
Write-Host "  Version: " -NoNewline; Write-Host "v$Version" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Next Steps:" -ForegroundColor Yellow
Write-Host ""
Write-Host "  1. Open a NEW terminal (so PATH takes effect)"
Write-Host ""
Write-Host "  2. Verify installation:"
Write-Host "     gres-b2b --version" -ForegroundColor Cyan
Write-Host ""
Write-Host "  3. Check prerequisites:"
Write-Host "     gres-b2b doctor" -ForegroundColor Cyan
Write-Host ""
Write-Host "  4. Run your first scan:"
Write-Host "     gres-b2b scan --live" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Documentation:" -ForegroundColor Yellow
Write-Host "    https://ajranjith.github.io/b2b-governance-action/onboarding/"
Write-Host ""
