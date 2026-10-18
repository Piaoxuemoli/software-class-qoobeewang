#!/usr/bin/env bash
# 客户端 4 段录制 · 宿主（Windows Git Bash）一键入口。
#
# 用法（仓库根目录）：
#   SEG_MODE=rehearsal SEG_PASS=1 SEG_SYNC_FLASH=1 bash client/e2e/record-demo/run-in-docker.sh  # 彩排1：测 kMs
#   SEG_MODE=rehearsal SEG_PASS=2 bash client/e2e/record-demo/run-in-docker.sh                    # 彩排2：复跑校准
#   SEG_MODE=formal bash client/e2e/record-demo/run-in-docker.sh                                 # 正式录制
#   SEG_IDS="seg-02-setup" SEG_MODE=rehearsal SEG_PASS=1 bash client/e2e/record-demo/run-in-docker.sh
#
# 容器内实际执行 client/e2e/record-demo/record-all.sh：
#   - Xvfb :98 2560x1600、HOME=/home/demo、PLAYWRIGHT_BROWSERS_PATH=tools/ms-playwright
#   - node_modules 用本 worktree 的 Linux 依赖卷（宿主目录为空，不能直接挂宿主 node_modules）
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)

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

# 注意：scripts/docker.sh volumes 的输出要整段接住再解析。直接管道给 awk 时
# awk 提前 exit 会让上游 docker 命令收到 SIGPIPE，配合 pipefail 会把本脚本带崩。
VOL_LIST=$(bash "$ROOT/scripts/docker.sh" volumes 2>/dev/null || true)
VOL=$(printf '%s\n' "$VOL_LIST" | grep -E '^databackup-node-' | awk '{print $1; exit}' || true)
if [ -z "${VOL:-}" ]; then
  echo "FATAL: 没找到本 worktree 的 node_modules 卷（先跑 bash scripts/docker.sh make client-install）" >&2
  exit 1
fi

echo "run-in-docker: root=$MOUNT_ROOT"
echo "run-in-docker: node_modules volume=$VOL"
echo "run-in-docker: SEG_MODE=${SEG_MODE:-rehearsal} SEG_PASS=${SEG_PASS:-1} SEG_IDS=${SEG_IDS:-<all>}"

docker run --rm \
  -v "$MOUNT_ROOT:/workspace" -w /workspace \
  -v "$VOL:/workspace/client/node_modules" \
  -e DISPLAY= \
  -e SEG_MODE="${SEG_MODE:-rehearsal}" \
  -e SEG_PASS="${SEG_PASS:-1}" \
  -e SEG_IDS="${SEG_IDS:-}" \
  -e SEG_K_MS="${SEG_K_MS:-}" \
  -e SEG_CLOSE_MARGIN_MS="${SEG_CLOSE_MARGIN_MS:-}" \
  -e SEG_VIDEO_FPS="${SEG_VIDEO_FPS:-}" \
  -e SEG_SYNC_FLASH="${SEG_SYNC_FLASH:-}" \
  databackup-dev bash /workspace/client/e2e/record-demo/record-all.sh
