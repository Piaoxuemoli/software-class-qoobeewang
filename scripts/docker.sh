#!/usr/bin/env bash
# docker.sh — run repo tasks inside the Docker toolchain images.
# Works on Linux and on Windows Git Bash (handles MSYS_NO_PATHCONV and
# Windows-style bind-mount paths). `make` is NOT needed on the host.
#
# Usage:
#   scripts/docker.sh check            dev image: npm ci + make check
#   scripts/docker.sh check-all        dev image: npm ci + make check-all
#   scripts/docker.sh shell            interactive bash in dev image
#   scripts/docker.sh docs             docs image: build-docs.sh -> docs/pdf/
#   scripts/docker.sh make <targets>   dev image: arbitrary make targets
#   scripts/docker.sh images [dev|docs]  build image(s) (default: both)
#
# Environment (all optional):
#   DOCS_PROFILE / DOCS_OUT    passed to the docs build (DOCS_OUT is resolved
#                              inside the container — repo-relative works best)
#   GOPROXY                    container Go proxy + dev image build-arg
#                              (default: https://goproxy.cn,direct; "" = default Go behavior)
#   ELECTRON_MIRROR            npm electron binary mirror
#                              (default: https://npmmirror.com/mirrors/electron/)
#   ELECTRON_BUILDER_BINARIES_MIRROR
#                              electron-builder binary mirror
#                              (default: https://npmmirror.com/mirrors/electron-builder-binaries/)
#   GH_MIRROR                  image build-arg: URL prefix mirror for
#                              github.com assets (default: https://gh-proxy.com/; "" disables)
#   GH_PROXY                   image build-arg: http(s) proxy for github.com
#                              downloads (default: empty = none)
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
DEV_IMAGE=databackup-dev
DOCS_IMAGE=databackup-docs
NODE_VOL=databackup-client-node-modules
GOCACHE_VOL=databackup-go-build-cache

# Git Bash on Windows: docker -v wants a Windows path; stop MSYS rewriting.
if [ "${OS:-}" = "Windows_NT" ]; then
  export MSYS_NO_PATHCONV=1
  if command -v cygpath >/dev/null 2>&1; then
    MOUNT_ROOT=$(cygpath -w "$ROOT")
  else
    MOUNT_ROOT=$(cd "$ROOT" && pwd -W)
  fi
else
  MOUNT_ROOT=$ROOT
fi

export GOPROXY=${GOPROXY-https://goproxy.cn,direct}
export ELECTRON_MIRROR=${ELECTRON_MIRROR-https://npmmirror.com/mirrors/electron/}
export ELECTRON_BUILDER_BINARIES_MIRROR=${ELECTRON_BUILDER_BINARIES_MIRROR-https://npmmirror.com/mirrors/electron-builder-binaries/}

build_args() { # docker build args shared by all image builds
  printf '%s\n' \
    --build-arg "GH_MIRROR=${GH_MIRROR:-https://gh-proxy.com/}" \
    --build-arg "GH_PROXY=${GH_PROXY:-}" \
    --build-arg "GOPROXY=${GOPROXY}"
}

ensure_image() { # <image> <dockerfile>
  docker image inspect "$1" >/dev/null 2>&1 && return 0
  echo "docker.sh: image $1 missing — building..."
  # shellcheck disable=SC2046
  docker build -f "$2" -t "$1" $(build_args) "$ROOT"
}

dev_run() { # <cmd...>
  docker run --rm \
    -v "$MOUNT_ROOT:/workspace" -w /workspace \
    -v "$NODE_VOL:/workspace/client/node_modules" \
    -v "$GOCACHE_VOL:/root/.cache/go-build" \
    -e DISPLAY= -e DOCS_PROFILE -e DOCS_OUT \
    -e ELECTRON_MIRROR -e ELECTRON_BUILDER_BINARIES_MIRROR -e GOPROXY \
    "$DEV_IMAGE" "$@"
}

usage() { sed -n '2,27p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

cmd=${1:-}
[ $# -gt 0 ] && shift || true
case "$cmd" in
  check)
    ensure_image "$DEV_IMAGE" docker/dev.Dockerfile
    dev_run make client-install check
    ;;
  check-all)
    ensure_image "$DEV_IMAGE" docker/dev.Dockerfile
    dev_run make client-install check-all
    ;;
  shell)
    ensure_image "$DEV_IMAGE" docker/dev.Dockerfile
    docker run --rm -it \
      -v "$MOUNT_ROOT:/workspace" -w /workspace \
      -v "$NODE_VOL:/workspace/client/node_modules" \
      -v "$GOCACHE_VOL:/root/.cache/go-build" \
      -e DISPLAY= -e ELECTRON_MIRROR -e ELECTRON_BUILDER_BINARIES_MIRROR -e GOPROXY \
      "$DEV_IMAGE" bash
    ;;
  docs)
    ensure_image "$DOCS_IMAGE" docker/docs.Dockerfile
    docker run --rm \
      -v "$MOUNT_ROOT:/workspace" -w /workspace \
      -e DOCS_PROFILE -e DOCS_OUT \
      "$DOCS_IMAGE" bash scripts/docs/build-docs.sh
    ;;
  make)
    ensure_image "$DEV_IMAGE" docker/dev.Dockerfile
    dev_run make "$@"
    ;;
  images)
    targets=("$@")
    [ ${#targets[@]} -eq 0 ] && targets=(dev docs)
    for which in "${targets[@]}"; do
      # shellcheck disable=SC2046
      case "$which" in
        dev)  docker build -f docker/dev.Dockerfile -t "$DEV_IMAGE" \
                $(build_args) "$ROOT" ;;
        docs) docker build -f docker/docs.Dockerfile -t "$DOCS_IMAGE" \
                $(build_args) "$ROOT" ;;
        *) echo "docker.sh: unknown image '$which' (dev|docs)" >&2; exit 1 ;;
      esac
    done
    ;;
  -h|--help|help|'')
    usage 0
    ;;
  *)
    echo "docker.sh: unknown command '$cmd'" >&2
    usage 1
    ;;
esac
