#!/usr/bin/env bash
# check-version.sh [tag] — assert backend (internal/version) and client
# (package.json) versions agree, optionally also with a release tag
# (leading "v" stripped). Exit 1 on mismatch.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

backend=$(sed -n 's/.*const Version = "\([^"]*\)".*/\1/p' \
  "$ROOT/backend/internal/version/version.go" | head -1)
client=$(sed -n 's/^  "version": "\([^"]*\)".*/\1/p' \
  "$ROOT/client/package.json" | head -1)

[ -n "$backend" ] || { echo "check-version: cannot read backend version" >&2; exit 1; }
[ -n "$client" ] || { echo "check-version: cannot read client version" >&2; exit 1; }

echo "backend: $backend"
echo "client:  $client"
if [ "$backend" != "$client" ]; then
  echo "check-version: backend/client mismatch" >&2
  exit 1
fi

if [ -n "${1:-}" ]; then
  tag=${1#v}
  echo "tag:     $tag"
  if [ "$tag" != "$backend" ]; then
    echo "check-version: tag '$tag' != version '$backend'" >&2
    exit 1
  fi
fi
echo "check-version: OK"
