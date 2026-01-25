#!/usr/bin/env bash
# GRES B2B Governance Action - Install and Run Script
# Downloads release binary, verifies checksum, and runs with provided args
set -euo pipefail

REPO="${GRES_B2B_REPO}"
VERSION="${GRES_B2B_VERSION}"
ARGS="${GRES_B2B_ARGS}"

# Map GitHub runner OS/arch to GoReleaser naming
case "${RUNNER_OS}" in
  Linux)   OS="linux" ;;
  Windows) OS="windows" ;;
  macOS)   OS="darwin" ;;
  *) echo "Unsupported OS: ${RUNNER_OS}" >&2; exit 2 ;;
esac

case "${RUNNER_ARCH}" in
  X64)   ARCH="amd64" ;;
  ARM64) ARCH="arm64" ;;
  *) echo "Unsupported arch: ${RUNNER_ARCH}" >&2; exit 2 ;;
esac

ASSET="gres-b2b_${OS}_${ARCH}.zip"

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

echo "::group::Verify checksum"

expected="$(grep " ${ASSET}\$" checksums.txt | awk '{print $1}' || true)"
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

echo "::group::Extract binary"

if command -v unzip >/dev/null 2>&1; then
  unzip -q "${ASSET}" -d ./bin
elif command -v bsdtar >/dev/null 2>&1; then
  mkdir -p ./bin
  bsdtar -xf "${ASSET}" -C ./bin
elif command -v tar >/dev/null 2>&1; then
  mkdir -p ./bin
  tar -xf "${ASSET}" -C ./bin
else
  echo "::error::No unzip/bsdtar/tar available to extract ${ASSET}"
  exit 5
fi

BIN="./bin/gres-b2b"
if [[ "${OS}" == "windows" ]]; then
  BIN="./bin/gres-b2b.exe"
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

echo "::group::Run gres-b2b ${ARGS}"

# Return to original directory for file operations
cd "${GITHUB_WORKSPACE:-$(pwd)}"

# shellcheck disable=SC2086
"${WORKDIR}/${BIN}" ${ARGS}

echo "::endgroup::"
