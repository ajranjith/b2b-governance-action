#!/usr/bin/env bash
# GRES B2B Governance Action - Install and Run Script
# Downloads release assets, verifies checksum, and runs gres-b2b
set -euo pipefail

# ============================================================================
# Configuration from environment (set by action.yml)
# ============================================================================
REPO="${GRES_B2B_REPO}"
VERSION="${GRES_B2B_VERSION}"
MODE="${GRES_B2B_MODE:-verify}"
CONFIG="${GRES_B2B_CONFIG:-}"
FAIL_ON_RED="${GRES_B2B_FAIL_ON_RED:-}"
ALLOW_AMBER="${GRES_B2B_ALLOW_AMBER:-}"
VECTORS="${GRES_B2B_VECTORS:-}"
SARIF="${GRES_B2B_SARIF:-.b2b/results.sarif}"
JUNIT="${GRES_B2B_JUNIT:-.b2b/junit.xml}"

# ============================================================================
# Platform detection
# ============================================================================
case "${RUNNER_OS}" in
  Linux)   OS="linux" ;;
  Windows) OS="windows" ;;
  macOS)   OS="darwin" ;;
  *) echo "::error::Unsupported OS: ${RUNNER_OS}"; exit 2 ;;
esac

case "${RUNNER_ARCH}" in
  X64)   ARCH="amd64" ;;
  ARM64) ARCH="arm64" ;;
  *) echo "::error::Unsupported arch: ${RUNNER_ARCH}"; exit 2 ;;
esac

ASSET="gres-b2b.zip"

# ============================================================================
# Download release
# ============================================================================
WORKDIR="$(mktemp -d)"
cd "$WORKDIR"

echo "::group::Download ${ASSET} from ${REPO} (${VERSION})"

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  export GH_TOKEN="${GITHUB_TOKEN}"
fi

if [[ "${VERSION}" == "latest" ]]; then
  gh release download --repo "${REPO}" --pattern "${ASSET}" --pattern "checksums.txt"
else
  gh release download "${VERSION}" --repo "${REPO}" --pattern "${ASSET}" --pattern "checksums.txt"
fi

# Verify files exist
[[ -f "${ASSET}" ]] || { echo "::error::Release asset not found: ${ASSET}"; ls -la; exit 4; }
[[ -f "checksums.txt" ]] || { echo "::error::checksums.txt not found in release"; ls -la; exit 4; }

echo "::endgroup::"

# ============================================================================
# Verify checksum
# ============================================================================
echo "::group::Verify SHA256 checksum"

expected="$(grep " ${ASSET}$" checksums.txt | awk '{print $1}' || true)"
if [[ -z "${expected}" ]]; then
  echo "::error::No checksum entry found for ${ASSET} in checksums.txt"
  cat checksums.txt
  exit 3
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${ASSET}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${ASSET}" | awk '{print $1}')"
elif command -v pwsh >/dev/null 2>&1; then
  actual="$(pwsh -NoProfile -Command "(Get-FileHash '${ASSET}' -Algorithm SHA256).Hash" | tr '[:upper:]' '[:lower:]')"
else
  echo "::error::No sha256sum, shasum, or pwsh available to verify checksum"
  exit 3
fi

if [[ "${expected}" != "${actual}" ]]; then
  echo "::error::Checksum mismatch for ${ASSET}"
  echo "Expected: ${expected}"
  echo "Actual:   ${actual}"
  exit 3
fi

echo "Checksum verified: ${expected}"
echo "::endgroup::"

# ============================================================================
# Extract binary
# ============================================================================
echo "::group::Extract binary"

if command -v unzip >/dev/null 2>&1; then
  unzip -q "${ASSET}" -d .
elif command -v bsdtar >/dev/null 2>&1; then
  bsdtar -xf "${ASSET}" -C .
elif command -v tar >/dev/null 2>&1; then
  tar -xf "${ASSET}" -C .
else
  echo "::error::No unzip/bsdtar/tar available to extract ${ASSET}"
  exit 5
fi

BIN="./bin/${OS}-${ARCH}/gres-b2b"
if [[ "${OS}" == "windows" ]]; then
  BIN="./bin/${OS}-${ARCH}/gres-b2b.exe"
fi

chmod +x "${BIN}" 2>/dev/null || true

if [[ ! -f "${BIN}" ]]; then
  echo "::error::Binary not found after extraction: ${BIN}"
  echo "Contents of ./bin:"
  ls -la ./bin
  exit 6
fi

echo "Binary ready: ${BIN}"
echo "::endgroup::"

# ============================================================================
# Compose command arguments
# ============================================================================
echo "::group::Compose command"

ARGS=()

# Config file (JSON overrides)
if [[ -n "${CONFIG}" ]]; then
  ARGS+=("--config" "${CONFIG}")
fi

# Mode
case "${MODE}" in
  verify)
    # run scan + verify for gating
    ;;
  watch)
    ARGS+=("--watch" "${GITHUB_WORKSPACE:-.}")
    ;;
  shadow)
    if [[ -z "${VECTORS}" ]]; then
      echo "::error::mode=shadow requires inputs.vectors"
      exit 7
    fi
    ARGS+=("--shadow" "--vectors" "${VECTORS}" "${GITHUB_WORKSPACE:-.}")
    ;;
  *)
    echo "::error::Invalid mode: ${MODE}. Must be verify, watch, or shadow."
    exit 7
    ;;
esac

# Print resolved command (safe for logs - no secrets)
echo "Mode:         ${MODE}"
echo "Config:       ${CONFIG:-<default>}"
echo "Fail on RED:  ${FAIL_ON_RED:-<default>}"
echo "Allow AMBER:  ${ALLOW_AMBER:-<default>}"
echo "SARIF output: ${SARIF}"
echo "JUnit output: ${JUNIT}"

echo "::endgroup::"

# ============================================================================
# Run gres-b2b
# ============================================================================
echo "::group::Run gres-b2b"

# Return to workspace for file operations
cd "${GITHUB_WORKSPACE:-$(pwd)}"

# Optional strictness overrides for verify gating
if [[ -n "${FAIL_ON_RED}" || -n "${ALLOW_AMBER}" ]]; then
  mkdir -p .b2b
  cat > .b2b/config.yml <<EOF
fail_on_red: ${FAIL_ON_RED:-true}
allow_amber: ${ALLOW_AMBER:-false}
EOF
fi

if [[ "${MODE}" == "verify" ]]; then
  "${WORKDIR}/${BIN}" ${ARGS[@]} scan
  "${WORKDIR}/${BIN}" ${ARGS[@]} verify
else
  "${WORKDIR}/${BIN}" ${ARGS[@]}
fi

echo "::endgroup::"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "Outputs:"
echo "  Certificate: .b2b/certificate.json"
echo "  SARIF:       ${SARIF}"
echo "  JUnit:       ${JUNIT}"
echo "  Report:      .b2b/report.json"
echo "  Dashboard:   .b2b/report.html"

# Copy outputs to custom paths if requested
if [[ "${SARIF}" != ".b2b/results.sarif" ]]; then
  mkdir -p "$(dirname "${SARIF}")"
  cp ".b2b/results.sarif" "${SARIF}"
fi
if [[ "${JUNIT}" != ".b2b/junit.xml" ]]; then
  mkdir -p "$(dirname "${JUNIT}")"
  cp ".b2b/junit.xml" "${JUNIT}"
fi
