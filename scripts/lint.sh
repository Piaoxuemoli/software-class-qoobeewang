#!/usr/bin/env bash
# Backend lint: gofmt, go vet, staticcheck, forbidden-import guard.
set -euo pipefail
cd "$(dirname "$0")/../backend"

# Diagnosability on CI runners that ship their own Go earlier on PATH.
echo "== toolchain =="
go version
command -v go gofmt staticcheck || true
staticcheck -version || true

echo "== gofmt =="
out=$(gofmt -l .)
if [ -n "$out" ]; then
  echo "$out" >&2
  echo "gofmt: files need formatting" >&2
  exit 1
fi

echo "== go vet =="
go vet ./...

echo "== staticcheck =="
staticcheck ./...

echo "== windows 交叉编译 =="
# C4b：保证 GOOS=windows 始终可编译可 vet（含测试文件）。
GOOS=windows go build ./...
GOOS=windows go vet ./...

cd ..
bash scripts/check-imports.sh
