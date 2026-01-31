@echo off
REM ============================================================================
REM GRES B2B CLI Release Build Script
REM
REM Builds the CLI and creates a release ZIP containing:
REM   - gres-b2b.exe (standalone Windows binary)
REM   - gres-b2b.config.json (default configuration)
REM   - README.txt (quick start guide)
REM   - checksums.txt (SHA256 checksums)
REM ============================================================================

setlocal EnableDelayedExpansion

REM Configuration
set VERSION=%1
if "%VERSION%"=="" set VERSION=1.0.0
set BUILD_DATE=%date:~10,4%-%date:~4,2%-%date:~7,2%
set OUTPUT_DIR=dist
set ZIP_NAME=gres-b2b-v%VERSION%-windows-amd64.zip

echo ============================================
echo GRES B2B CLI Release Builder
echo Version: %VERSION%
echo Build Date: %BUILD_DATE%
echo ============================================
echo.

REM Create output directory
if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%
if exist %OUTPUT_DIR%\staging rd /s /q %OUTPUT_DIR%\staging
mkdir %OUTPUT_DIR%\staging

REM Build the binary
echo [1/5] Building gres-b2b.exe...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
set LDFLAGS=-s -w -X main.Version=%VERSION% -X main.BuildDate=%BUILD_DATE%

go build -ldflags "%LDFLAGS%" -o %OUTPUT_DIR%\staging\gres-b2b.exe .
if errorlevel 1 (
    echo ERROR: Build failed!
    exit /b 1
)
echo    Built: %OUTPUT_DIR%\staging\gres-b2b.exe

REM Copy config file
echo [2/5] Copying configuration...
if exist gres-b2b.config.json (
    copy gres-b2b.config.json %OUTPUT_DIR%\staging\ > nul
    echo    Copied: gres-b2b.config.json
) else (
    echo    WARNING: gres-b2b.config.json not found
)

REM Create README.txt
echo [3/5] Creating README.txt...
(
echo GRES B2B Governance Engine v%VERSION%
echo =====================================
echo.
echo Quick Start:
echo.
echo 1. Extract all files to: %%LOCALAPPDATA%%\Programs\gres-b2b\
echo    Or: C:\Program Files\GRES\B2B\
echo.
echo 2. Run a health check:
echo    gres-b2b.exe doctor
echo.
echo 3. Configure your AI agent's MCP settings to use:
echo    {
echo      "mcpServers": {
echo        "gres-b2b": {
echo          "command": "C:\\Users\\YOUR_USER\\AppData\\Local\\Programs\\gres-b2b\\gres-b2b.exe",
echo          "args": ["mcp", "serve"]
echo        }
echo      }
echo    }
echo.
echo Configuration:
echo.
echo   The gres-b2b.config.json file contains default settings.
echo   Place it in the same directory as the executable.
echo.
echo   Config search order:
echo   1. --config flag ^(explicit path^)
echo   2. Same directory as executable
echo   3. %%ProgramData%%\GRES\B2B\gres-b2b.config.json
echo   4. Built-in defaults
echo.
echo Commands:
echo.
echo   gres-b2b --help       Show all commands
echo   gres-b2b --version    Show version info
echo   gres-b2b doctor       Run health check
echo   gres-b2b mcp          Start MCP server mode
echo   gres-b2b scan         Scan workspace for issues
echo.
echo Documentation: https://ajranjith.github.io/b2b-governance-action/
echo.
echo Build: v%VERSION% ^(%BUILD_DATE%^)
) > %OUTPUT_DIR%\staging\README.txt
echo    Created: README.txt

REM Generate checksums
echo [4/5] Generating checksums...
pushd %OUTPUT_DIR%\staging
(
echo SHA256 Checksums for GRES B2B v%VERSION%
echo Generated: %BUILD_DATE%
echo.
) > checksums.txt

for %%f in (gres-b2b.exe gres-b2b.config.json) do (
    if exist %%f (
        for /f "delims=" %%h in ('certutil -hashfile %%f SHA256 ^| findstr /v ":" ^| findstr /v "CertUtil"') do (
            echo %%h  %%f >> checksums.txt
        )
    )
)
popd
echo    Created: checksums.txt

REM Create ZIP archive
echo [5/5] Creating ZIP archive...
if exist %OUTPUT_DIR%\%ZIP_NAME% del %OUTPUT_DIR%\%ZIP_NAME%

REM Use PowerShell to create ZIP
powershell -NoProfile -Command "Compress-Archive -Path '%OUTPUT_DIR%\staging\*' -DestinationPath '%OUTPUT_DIR%\%ZIP_NAME%' -Force"
if errorlevel 1 (
    echo ERROR: Failed to create ZIP archive!
    exit /b 1
)

REM Show results
echo.
echo ============================================
echo Build Complete!
echo ============================================
echo.
echo Output: %OUTPUT_DIR%\%ZIP_NAME%
echo.
echo Contents:
dir /b %OUTPUT_DIR%\staging
echo.

REM Get file size
for %%A in (%OUTPUT_DIR%\%ZIP_NAME%) do set SIZE=%%~zA
set /a SIZE_KB=%SIZE%/1024
echo ZIP Size: %SIZE_KB% KB
echo.

REM Cleanup staging
rd /s /q %OUTPUT_DIR%\staging

echo Done!

