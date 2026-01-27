@echo off
REM GRES B2B Bootstrapper - Windows Build Script
REM Builds a native Windows GUI installer (no console window)

echo Building GRES B2B Bootstrapper...

REM Install rsrc tool if needed (embeds manifest)
where rsrc >nul 2>&1
if %errorlevel% neq 0 (
    echo Installing rsrc tool...
    go install github.com/akavel/rsrc@latest
)

REM Generate resource file from manifest
echo Generating Windows resources...
rsrc -manifest rsrc.syso.manifest -o rsrc.syso

REM Build with no console window (-H windowsgui)
echo Compiling...
go build -ldflags="-H windowsgui -s -w" -o gres-b2b-setup.exe .

if exist gres-b2b-setup.exe (
    echo.
    echo Build successful: gres-b2b-setup.exe
    echo.
) else (
    echo.
    echo Build failed!
    exit /b 1
)
