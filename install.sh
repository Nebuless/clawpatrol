#!/bin/sh
# nebprx installer. Downloads a prebuilt binary from GitHub Releases.
# Builds from source when NEBPRX_FROM_SOURCE=1 (requires Go, Node.js and pnpm).
#
# Usage:
#   curl -fsSL https://prx.nebuless.cloud/install.sh | sh
#
# Options (env vars):
#   NEBPRX_VERSION       — release tag (default: latest)
#   NEBPRX_PREFIX        — install dir (default: $HOME/.local/bin)
#   NEBPRX_FROM_SOURCE   — 1 to build from source (Go, Node.js and pnpm required)
#   NEBPRX_REF           — git ref when building from source (default: main)
#   NEBPRX_NO_TELEMETRY  — 1 to skip the anonymous install-event ping
#
# POSIX sh.

set -eu

REPO="Nebuless/PRX"
VERSION="${NEBPRX_VERSION:-latest}"
PREFIX="${NEBPRX_PREFIX:-$HOME/.local/bin}"

# Anonymous install telemetry. Pings prx.nebuless.cloud with os/arch/
# version and a generated per-run id so we can count installs and
# debug failures. Opt out with NEBPRX_NO_TELEMETRY=1.
TELEMETRY_URL="https://prx.nebuless.cloud/api/telemetry/v1/install"
INSTALL_ID=""

telemetry() {
  [ "${NEBPRX_NO_TELEMETRY:-0}" = "1" ] && return 0
  command -v curl >/dev/null 2>&1 || return 0
  if [ -z "$INSTALL_ID" ]; then
    INSTALL_ID=$(
      od -An -N16 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' \
        || date +%s%N 2>/dev/null \
        || date +%s
    )
  fi
  event="$1"
  reason="${2:-}"
  body=$(printf '{"install_id":"%s","event":"%s","os":"%s","arch":"%s","version":"%s","from_source":"%s","reason":"%s"}' \
    "$INSTALL_ID" "$event" \
    "${OS:-}" "${ARCH:-}" "$VERSION" \
    "${NEBPRX_FROM_SOURCE:-0}" "$reason")
  (curl -fsS --max-time 3 -X POST \
     -H 'Content-Type: application/json' \
     -d "$body" "$TELEMETRY_URL" >/dev/null 2>&1 || true) &
}

if [ "$VERSION" = "latest" ]; then
  BASE="https://github.com/${REPO}/releases/latest/download"
else
  BASE="https://github.com/${REPO}/releases/download/${VERSION}"
fi

say()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
fail() { telemetry failed "$*"; printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) fail "unsupported arch: $ARCH" ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) fail "unsupported OS: $OS" ;;
esac

command -v curl >/dev/null 2>&1 || fail "curl required"
mkdir -p "$PREFIX"

# source-build branch
if [ "${NEBPRX_FROM_SOURCE:-0}" = "1" ]; then
  REF="${NEBPRX_REF:-main}"
  command -v go  >/dev/null 2>&1 || fail "go toolchain required for source build"
  command -v git >/dev/null 2>&1 || fail "git required"
  command -v pnpm >/dev/null 2>&1 || fail "pnpm 12.16.1 required for source build (see doc/dev-setup.md)"
  SRC=$(mktemp -d)
  trap 'rm -rf "$SRC"' EXIT INT TERM
  say "cloning ${REPO}@${REF}"
  git clone --depth 1 --branch "$REF" "https://github.com/${REPO}.git" "$SRC" >/dev/null 2>&1 \
    || git clone --depth 1 "https://github.com/${REPO}.git" "$SRC" >/dev/null 2>&1 \
    || fail "git clone failed"
  if [ -d "$SRC/dashboard" ]; then
    say "building dashboard"
    ( cd "$SRC/dashboard" && pnpm install --frozen-lockfile && pnpm exec vp run build ) \
      || fail "dashboard build failed"
  fi
  [ -s "$SRC/dashboard/dist/index.html" ] || fail "dashboard bundle missing"
  say "building nebprx"
  ( cd "$SRC" && go build -ldflags "-s -w" -trimpath -o nebprx ./cmd/nebprx ) || fail "go build failed"
  mv "$SRC/nebprx" "$PREFIX/nebprx"
  chmod +x "$PREFIX/nebprx"
  say "installed: $PREFIX/nebprx"
  telemetry completed
  exit 0
fi

# binary download (default)
URL="${BASE}/nebprx-${OS}-${ARCH}"
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT INT TERM
say "downloading ${URL}"
curl -fsSL -o "$TMP" "$URL" || fail "download failed (no release for ${OS}-${ARCH} at ${VERSION}?)"

# optional sha256 verify
SUMS=$(curl -fsSL "${BASE}/SHA256SUMS" 2>/dev/null || true)
if [ -n "$SUMS" ]; then
  EXPECTED=$(printf '%s\n' "$SUMS" | awk -v f="nebprx-${OS}-${ARCH}" '$2==f{print $1}')
  if [ -n "$EXPECTED" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL=$(sha256sum "$TMP" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL=$(shasum -a 256 "$TMP" | awk '{print $1}')
    else
      ACTUAL=""
    fi
    if [ -n "$ACTUAL" ]; then
      [ "$EXPECTED" = "$ACTUAL" ] || fail "sha256 mismatch (expected $EXPECTED, got $ACTUAL)"
      say "sha256 ok"
    fi
  fi
fi

mv "$TMP" "$PREFIX/nebprx"
chmod +x "$PREFIX/nebprx"
say "installed: $PREFIX/nebprx"

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) printf '\n  add to PATH:  export PATH="%s:$PATH"\n\n' "$PREFIX" ;;
esac

"$PREFIX/nebprx" version 2>/dev/null || true
echo

telemetry completed
echo "next: nebprx join <gateway-url>"
