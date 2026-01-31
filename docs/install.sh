#!/bin/bash
#
# GRES B2B Governance - Installer for Linux/macOS
#
# Usage:
#   curl -fsSL https://ajranjith.github.io/b2b-governance-action/install.sh | bash
#
# Or with specific version:
#   curl -fsSL https://ajranjith.github.io/b2b-governance-action/install.sh | bash -s -- --version 4.0.0
#
# Repository: https://github.com/ajranjith/b2b-governance-action
#

set -euo pipefail

REPO="ajranjith/b2b-governance-action"
BINARY_NAME="gres-b2b"
VERSION="${VERSION:-latest}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

step() { echo -e "  ${CYAN}->${NC} $1"; }
success() { echo -e "  ${GREEN}OK${NC} $1"; }
fail() { echo -e "  ${RED}FAIL${NC} $1"; }
warn() { echo -e "  ${YELLOW}WARN${NC} $1"; }

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --version)
      VERSION="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  GRES B2B Governance - Installer${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Step 1: Detect OS and architecture
step "Detecting system..."
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    fail "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    fail "Unsupported OS: $OS"
    exit 1
    ;;
esac

success "Detected: $OS/$ARCH"

# Step 2: Determine install directory
if [[ -z "${INSTALL_DIR:-}" ]]; then
  if [[ -d "$HOME/.local/bin" ]] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    INSTALL_DIR="$HOME/.local/bin"
  elif [[ -w "/usr/local/bin" ]]; then
    INSTALL_DIR="/usr/local/bin"
  elif command -v sudo &>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
    USE_SUDO=1
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi
fi

step "Install directory: $INSTALL_DIR"

# Step 3: Get latest version if needed
if [[ "$VERSION" == "latest" ]]; then
  step "Fetching latest release version..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
  if [[ -z "$VERSION" ]]; then
    fail "Could not determine latest version"
    exit 1
  fi
  success "Latest version: $VERSION"
else
  VERSION="${VERSION#v}"
  success "Using specified version: $VERSION"
fi

# Step 4: Build download URL
ARCHIVE_NAME="gres-b2b.zip"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$VERSION/$ARCHIVE_NAME"
step "Download URL: $DOWNLOAD_URL"

# Step 5: Download and extract
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

step "Downloading $ARCHIVE_NAME..."
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/$ARCHIVE_NAME"; then
  fail "Download failed"
  echo ""
  echo -e "  ${YELLOW}Possible causes:${NC}"
  echo "    - Release v$VERSION does not exist"
  echo "    - Network connectivity issue"
  echo "    - GitHub rate limiting"
  echo ""
  echo -e "  ${YELLOW}Check releases at:${NC}"
  echo "    https://github.com/$REPO/releases"
  exit 1
fi
success "Downloaded"

step "Extracting archive..."
if command -v unzip >/dev/null 2>&1; then
  unzip -q "$TEMP_DIR/$ARCHIVE_NAME" -d "$TEMP_DIR"
elif command -v bsdtar >/dev/null 2>&1; then
  bsdtar -xf "$TEMP_DIR/$ARCHIVE_NAME" -C "$TEMP_DIR"
else
  fail "unzip/bsdtar not found"
  exit 1
fi

EXTRACTED_BINARY="$TEMP_DIR/bin/${OS}-${ARCH}/$BINARY_NAME"
if [[ ! -f "$EXTRACTED_BINARY" ]]; then
  fail "Binary $BINARY_NAME not found in archive"
  exit 1
fi
success "Extracted"

# Step 6: Install binary
step "Installing to $INSTALL_DIR..."
if [[ "${USE_SUDO:-}" == "1" ]]; then
  sudo install -m 755 "$EXTRACTED_BINARY" "$INSTALL_DIR/$BINARY_NAME"
else
  install -m 755 "$EXTRACTED_BINARY" "$INSTALL_DIR/$BINARY_NAME"
fi
success "Installed to $INSTALL_DIR/$BINARY_NAME"

# Step 7: Check PATH
step "Checking PATH..."
if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  success "$INSTALL_DIR is in PATH"
else
  warn "$INSTALL_DIR is NOT in PATH"
  echo ""
  echo -e "  ${YELLOW}Add to your shell profile:${NC}"

  SHELL_NAME=$(basename "$SHELL")
  case "$SHELL_NAME" in
    bash) PROFILE_FILE="$HOME/.bashrc" ;;
    zsh)  PROFILE_FILE="$HOME/.zshrc" ;;
    *)    PROFILE_FILE="$HOME/.profile" ;;
  esac

  echo ""
  echo "    echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> $PROFILE_FILE"
  echo "    source $PROFILE_FILE"
  echo ""

  export PATH="$INSTALL_DIR:$PATH"
fi

# Step 8: Verify installation
step "Verifying installation..."
if VERSION_OUTPUT=$("$INSTALL_DIR/$BINARY_NAME" --version 2>&1); then
  success "Verified: $VERSION_OUTPUT"
else
  success "Binary installed (version output: $VERSION_OUTPUT)"
fi

# Done!
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "  Binary:  ${CYAN}$INSTALL_DIR/$BINARY_NAME${NC}"
echo -e "  Version: ${CYAN}v$VERSION${NC}"
echo ""
echo -e "  ${YELLOW}Next Steps:${NC}"
echo ""
echo "  1. Ensure $INSTALL_DIR is in your PATH"
echo ""
echo "  2. Verify installation:"
echo -e "     ${CYAN}gres-b2b --version${NC}"
echo ""
echo "  3. Check prerequisites:"
echo -e "     ${CYAN}gres-b2b doctor${NC}"
echo ""
echo "  4. Run your first scan:"
echo -e "     ${CYAN}gres-b2b scan${NC}"
echo ""
echo "  5. Start MCP server:"
echo -e "     ${CYAN}gres-b2b mcp serve${NC}"
echo ""
echo -e "  ${YELLOW}Documentation:${NC}"
echo "    https://github.com/$REPO/blob/main/docs/INSTALL.md"
echo ""
