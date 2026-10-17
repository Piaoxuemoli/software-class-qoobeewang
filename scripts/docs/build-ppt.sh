#!/usr/bin/env bash
# build-ppt.sh — 生成 docs/ppt/答辩PPT.pptx（M9-3）。
#
# 只用 Node + pptxgenjs：依赖装在 gitignored 的 build/ppt-deps 里，不进入仓库。
# 幻灯片内容与排版在 scripts/docs/pptx-build.mjs；文字源在
# docs/deliverables/05-答辩PPT.md，两者必须同步修改。
#
# 用法：bash scripts/docs/build-ppt.sh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"

DEPS="$ROOT/build/ppt-deps"
if [ ! -d "$DEPS/node_modules/pptxgenjs" ]; then
  echo "== installing pptxgenjs into build/ppt-deps =="
  npm install --prefix "$DEPS" --no-audit --no-fund pptxgenjs@4
fi

NODE_PATH="$DEPS/node_modules" node scripts/docs/pptx-build.mjs
