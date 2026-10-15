#!/usr/bin/env bash
# gh-download <out-file> <sha256> <url> [fallback-url ...]
#
# Download a release asset, trying each URL in turn. For github.com URLs two
# optional fallbacks apply (both empty/unset = disabled):
#   GH_MIRROR  URL prefix mirror, e.g. https://gh-proxy.com/
#   GH_PROXY   http(s) proxy, e.g. http://proxy.example.com:7890
# Integrity is enforced by the SHA256 check, not by the transport.
set -euo pipefail

out=$1; sha=$2; shift 2
urls=("$@")

GH_MIRROR=${GH_MIRROR:-}
GH_PROXY=${GH_PROXY:-}

try_curl() {
  curl -fsSL --connect-timeout 8 --max-time 300 -o "$out" "$@"
}

ok=0
for url in "${urls[@]}"; do
  echo "gh-download: trying $url"
  if try_curl "$url"; then ok=1; break; fi
  if [[ $url == https://github.com/* ]]; then
    if [ -n "$GH_MIRROR" ]; then
      echo "gh-download: trying mirror ${GH_MIRROR}${url}"
      if try_curl "${GH_MIRROR}${url}"; then ok=1; break; fi
    fi
    if [ -n "$GH_PROXY" ]; then
      echo "gh-download: trying via proxy $GH_PROXY"
      if https_proxy="$GH_PROXY" http_proxy="$GH_PROXY" try_curl "$url"; then
        ok=1; break
      fi
    fi
  fi
done

if [ "$ok" != 1 ]; then
  echo "gh-download: all sources failed" >&2
  exit 1
fi
echo "${sha}  ${out}" | sha256sum -c -
