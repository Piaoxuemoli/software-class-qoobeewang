#!/usr/bin/env bash
# changelog-section.sh X.Y.Z — print the body of that version's CHANGELOG
# section (used for GitHub release notes). Exits 1 if not found.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
VER=${1:?usage: $0 X.Y.Z}

out=$(awk -v ver="$VER" '
  $0 ~ "^## \\[" ver "\\]" { found=1; next }
  found && /^## \[/ { exit }
  found { print }
' "$ROOT/CHANGELOG.md")

# trim leading/trailing blank lines
out=$(printf '%s\n' "$out" | sed -e '1{/^[[:space:]]*$/d}' | sed -e :a -e '/^\n*$/{$d;N;ba}')

if [ -z "${out//[[:space:]]/}" ]; then
  echo "changelog-section: no '## [$VER]' section in CHANGELOG.md" >&2
  exit 1
fi
printf '%s\n' "$out"
