#!/usr/bin/env bash
# Forbidden-import guard for the Go backend.
#
# Course rule: extension features (packing, compression, crypto, hashing)
# must be hand-written. This script fails if any NON-TEST package in
# backend/ directly imports a forbidden stdlib package or any third-party
# module. Test files are exempt: they may use these packages as
# differential-testing oracles.
set -euo pipefail

cd "$(dirname "$0")/../backend"

FORBIDDEN_REGEX='^(archive/|compress/|crypto/aes$|crypto/cipher$|crypto/des$|crypto/sha256$|crypto/sha512$|crypto/sha1$|crypto/md5$|crypto/hmac$|crypto/pbkdf2$|golang\.org/x/)'

check_dir() {
  local dir=$1
  local bad=0
  local listing
  # -buildvcs=false: import listing needs no VCS stamping, and the checkout may
  # be owned by a different user (mounted workspaces, CI edge cases).
  if ! listing=$(cd "$dir" && go list -buildvcs=false -f '{{.ImportPath}} {{join .Imports " "}}' ./... 2>&1); then
    echo "check-imports: go list failed in $dir:" >&2
    echo "$listing" >&2
    return 1
  fi
  # "{{join .Imports " "}}" covers only direct non-test imports.
  while read -r pkg imports; do
    for imp in $imports; do
      if [[ $imp =~ $FORBIDDEN_REGEX ]]; then
        echo "check-imports: $pkg imports forbidden package $imp" >&2
        bad=1
      elif [[ $imp == *.* && $imp != github.com/Piaoxuemoli/software-class-qoobeewang/* ]]; then
        echo "check-imports: $pkg imports third-party module $imp" >&2
        bad=1
      fi
    done
  done <<< "$listing"
  return $bad
}

echo "check-imports: scanning backend production imports..."
if ! check_dir .; then
  echo "check-imports: FAILED" >&2
  exit 1
fi

# Self-test: a scratch package importing compress/flate must be rejected.
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
cat > "$tmpdir/go.mod" <<'EOF'
module checkimports-selftest

go 1.27.1
EOF
cat > "$tmpdir/bad.go" <<'EOF'
package bad

import _ "compress/flate"
EOF
if check_dir "$tmpdir" 2>/dev/null; then
  echo "check-imports: self-test FAILED (compress/flate import was accepted)" >&2
  exit 1
fi

echo "check-imports: OK"
