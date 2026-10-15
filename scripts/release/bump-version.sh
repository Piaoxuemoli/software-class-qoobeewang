#!/usr/bin/env bash
# bump-version.sh X.Y.Z — set backend + client versions and roll the
# CHANGELOG [Unreleased] section over to the new release.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

VER=${1:-}
case "$VER" in
  ''|*[!0-9.]*) echo "usage: $0 X.Y.Z" >&2; exit 1 ;;
esac

DATE=$(date +%F)

# backend: internal/version/version.go const Version
sed -i "s/const Version = \"[^\"]*\"/const Version = \"$VER\"/" \
  "$ROOT/backend/internal/version/version.go"

# client: package.json "version" (first occurrence)
sed -i "0,/\"version\": \"[^\"]*\"/s//\"version\": \"$VER\"/" \
  "$ROOT/client/package.json"

# CHANGELOG: rename '## [Unreleased]' to '## [X.Y.Z] - <date>' and add a
# fresh empty Unreleased section above it.
CHANGELOG="$ROOT/CHANGELOG.md"
if grep -q '^## \[Unreleased\]' "$CHANGELOG"; then
  awk -v ver="$VER" -v date="$DATE" '
    !done && /^## \[Unreleased\]/ {
      print "## [Unreleased]"
      print ""
      print "## [" ver "] - " date
      done=1
      next
    }
    { print }
  ' "$CHANGELOG" > "$CHANGELOG.tmp" && mv "$CHANGELOG.tmp" "$CHANGELOG"
else
  echo "bump-version: WARNING: no '## [Unreleased]' in CHANGELOG.md" >&2
fi

echo "bumped to $VER"
