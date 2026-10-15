#!/usr/bin/env bash
# Backend formatter. Default: gofmt -w in place. `--check`: fail if any
# file is not gofmt-clean (no changes made).
set -euo pipefail
cd "$(dirname "$0")/../backend"

if [ "${1:-}" = "--check" ]; then
  out=$(gofmt -l .)
  if [ -n "$out" ]; then
    echo "$out" >&2
    echo "gofmt check failed" >&2
    exit 1
  fi
  echo "gofmt: clean"
else
  gofmt -w .
fi
