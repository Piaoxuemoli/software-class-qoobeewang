#!/usr/bin/env bash
# install-deps.sh — single source of truth for the dev/CI toolchain.
#
# Targets Ubuntu 22.04 and 24.04, both:
#   - Docker image builds (root, no sudo)
#   - WSL/Ubuntu hosts (non-root; apt commands prefixed with sudo)
#
# Installs: git/make/gcc (needed for `go test -race`), Go (pinned tarball,
# SHA256-verified), staticcheck (pinned), Node 22 LTS (pinned tarball,
# SHA256-verified), Electron runtime libs + xvfb for e2e tests.
# Idempotent and non-interactive.
set -euo pipefail

GO_VERSION=1.27.1
GO_SHA256_AMD64=63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445
NODE_VERSION=22.23.3
NODE_SHA256_X64=1084aa36196bba4c3a5e69a1ee388a6e4ff729dad09445fbcd434b28fe3c24af
STATICCHECK_VERSION=2026.2.1

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  command -v sudo >/dev/null || { echo "need root or sudo" >&2; exit 1; }
  SUDO="sudo"
fi
export DEBIAN_FRONTEND=noninteractive

# --- apt packages ------------------------------------------------------------
$SUDO apt-get update

# Pick whichever package name exists (22.04 vs 24.04 t64 renames).
pick() { for p in "$@"; do apt-cache show "$p" >/dev/null 2>&1 && { echo "$p"; return; }; done; }

PKGS=(
  git make gcc g++ curl ca-certificates xz-utils
  xvfb xauth fonts-noto-cjk
  libnss3 libxss1 libxtst6 libgbm1 libdrm2 libxkbcommon0
  libxcomposite1 libxdamage1 libxrandr2 libxfixes3 libxcb-dri3-0
  libnotify4 xdg-utils
)
for alt in \
  "libasound2 libasound2t64" \
  "libgtk-3-0 libgtk-3-0t64" \
  "libcups2 libcups2t64" \
  "libatk1.0-0 libatk1.0-0t64" \
  "libatk-bridge2.0-0 libatk-bridge2.0-0t64" \
  "libatspi2.0-0 libatspi2.0-0t64"; do
  p=$(pick $alt) && PKGS+=("$p") || echo "warning: none of [$alt] found" >&2
done
PKGS+=(libpango-1.0-0 libcairo2)
$SUDO apt-get install -y --no-install-recommends "${PKGS[@]}"

arch=$(uname -m)
case "$arch" in
  x86_64)  GOARCH=amd64; GO_SHA=$GO_SHA256_AMD64; NODEARCH=x64; NODE_SHA=$NODE_SHA256_X64 ;;
  aarch64|arm64)
    GOARCH=arm64
    GO_SHA=$(curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-arm64.tar.gz.sha256")
    NODEARCH=arm64
    NODE_SHA=$(curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/SHASUMS256.txt" \
      | awk '/linux-arm64.tar.gz$/{print $1}')
    ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

# --- Go ----------------------------------------------------------------------
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
  echo "installing Go ${GO_VERSION} (${GOARCH})"
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  curl -fsSL -o "$tmp/go.tgz" "https://dl.google.com/go/go${GO_VERSION}.linux-${GOARCH}.tar.gz"
  echo "${GO_SHA}  $tmp/go.tgz" | sha256sum -c -
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf "$tmp/go.tgz"
else
  echo "Go ${GO_VERSION} already installed"
fi
command -v go >/dev/null || $SUDO ln -sf /usr/local/go/bin/go /usr/local/bin/go
$SUDO ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# --- staticcheck -------------------------------------------------------------
if ! staticcheck -version 2>/dev/null | grep -q "${STATICCHECK_VERSION}"; then
  echo "installing staticcheck ${STATICCHECK_VERSION}"
  # GOPROXY env (e.g. set by scripts/docker.sh) overrides the default proxy;
  # nothing mirror-specific is baked in here.
  $SUDO env "GOBIN=/usr/local/bin" GOPATH=/tmp/gopath-staticcheck \
    go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}"
else
  echo "staticcheck ${STATICCHECK_VERSION} already installed"
fi

# --- Node.js -----------------------------------------------------------------
if ! node --version 2>/dev/null | grep -q "v${NODE_VERSION}"; then
  echo "installing Node.js ${NODE_VERSION} (${NODEARCH})"
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  curl -fsSL -o "$tmp/node.tgz" "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODEARCH}.tar.gz"
  echo "${NODE_SHA}  $tmp/node.tgz" | sha256sum -c -
  $SUDO tar -C /usr/local -xzf "$tmp/node.tgz" --strip-components=1
else
  echo "Node.js ${NODE_VERSION} already installed"
fi

echo "install-deps: done"
go version
staticcheck -version || true
node --version
