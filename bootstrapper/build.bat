@echo off
REM GRES B2B Bootstrapper - Windows Build Script
REM Builds a zero-terminal native Windows installer using dlgs dialogs

echo.
echo ========================================
echo   GRES B2B Bootstrapper - Build
echo ========================================
echo.

REM Check Go is installed
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Go is not installed or not in PATH
    exit /b 1
)

REM Download dependencies
echo Downloading dependencies...
go mod download
if %errorlevel% neq 0 (
    echo ERROR: Failed to download dependencies
    exit /b 1
)

REM Build with no console window (-H windowsgui)
echo.
echo Compiling (GUI mode, no console window)...
go build -ldflags="-H windowsgui -s -w" -o gres-b2b-setup.exe .

if exist gres-b2b-setup.exe (
    echo.
    echo ========================================
    echo   Build Successful!
    echo ========================================
    echo.
    echo   Output: gres-b2b-setup.exe
    echo.
    echo   Features:
    echo     - Zero-terminal installer (uses native dialogs)
    echo     - Agent-agnostic MCP verification
    echo     - Per-user install (no admin required)
    echo     - Automatic PATH update
    echo.
) else (
    echo.
    echo ERROR: Build failed!
    exit /b 1
)
