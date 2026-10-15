#!/usr/bin/env bash
# Backend lint: gofmt, go vet, staticcheck, forbidden-import guard.
set -euo pipefail
cd "$(dirname "$0")/../backend"

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

cd ..
bash scripts/check-imports.sh
