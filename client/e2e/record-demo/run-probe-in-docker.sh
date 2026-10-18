#!/usr/bin/env bash
# 媒体基建预检 · 在 databackup-dev 容器里跑录制链路探针（不是正式录制脚本）。
#
# 宿主机（Git Bash）调用示例：
#   MSYS_NO_PATHCONV=1 docker run --rm \
#     -v "C:/Users/Qoobeewang/Desktop/软件工程实验:/workspace" -w /workspace \
#     -v "databackup-node-69d49d18:/workspace/client/node_modules" \
#     -e DISPLAY= \
#     databackup-dev bash /workspace/client/e2e/record-demo/run-probe-in-docker.sh
#
# 说明：node_modules 是本 worktree 的 Linux 依赖卷（宿主侧 electron/dist 为空），
# 卷名用 `bash scripts/docker.sh volumes` 查。
set -uo pipefail

REPO=/workspace
CLIENT="$REPO/client"
PROBE_OUT="$REPO/course/04-视频/work/probe"
DISPLAY_NUM=:98
RC=0

echo '==================== 环境预检 ===================='
echo "date:        $(date -Is)"
echo "node:        $(node --version)"
echo "uid:         $(id -u) ($(id -un))"
echo "DISPLAY:     ${DISPLAY_NUM}"
echo "electron:    $(node -e "console.log(require('$CLIENT/node_modules/electron/package.json').version)" 2>&1)"
echo "playwright:  $(node -e "console.log(require('$CLIENT/node_modules/@playwright/test/package.json').version)" 2>&1)"
echo "backend:     $REPO/backend/bin/databackup -> $(command -v file >/dev/null && file -b "$REPO/backend/bin/databackup" 2>&1 || echo '(file 命令缺失)')"
echo "zh fonts:    $(fc-list :lang=zh | wc -l) entries"
echo "pw browsers: ${PLAYWRIGHT_BROWSERS_PATH:-$REPO/course/04-视频/tools/ms-playwright}"
echo "probe out:   $PROBE_OUT"
mkdir -p "$PROBE_OUT"

[ -x "$CLIENT/node_modules/electron/dist/electron" ] || { echo 'FATAL: 缺少 electron 二进制（node_modules 卷没挂对？）'; exit 1; }
[ -x "$REPO/backend/bin/databackup" ] || { echo 'FATAL: 缺少 backend/bin/databackup（先 make backend）'; exit 1; }

echo '==================== Xvfb ===================='
# 2560x1600：1120x720 窗口 × dsf 2 = 2240x1440 物理像素，必须放得下。
Xvfb "$DISPLAY_NUM" -screen 0 2560x1600x24 -nolisten tcp >/tmp/xvfb-probe.log 2>&1 &
XVFB_PID=$!
trap 'kill "$XVFB_PID" 2>/dev/null' EXIT
for _ in $(seq 1 100); do
  [ -e "/tmp/.X11-unix/X${DISPLAY_NUM#:}" ] && break
  sleep 0.1
done
if [ -e "/tmp/.X11-unix/X${DISPLAY_NUM#:}" ]; then
  echo "Xvfb ready on $DISPLAY_NUM (socket /tmp/.X11-unix/X${DISPLAY_NUM#:})"
else
  echo 'FATAL: Xvfb 没起来'; cat /tmp/xvfb-probe.log; exit 1
fi

export DISPLAY="$DISPLAY_NUM"
export HOME=/home/demo
export XDG_CONFIG_HOME=/home/demo/.config
export DATABACKUP_BIN="$REPO/backend/bin/databackup"
# Playwright recordVideo 依赖自带的 ffmpeg（registry 里的 "ffmpeg" 浏览器），
# 开发镜像里没有 → 预装到宿主可持久化的 tools/ms-playwright（不改镜像）：
#   docker run --rm -v ... -w /workspace/client -e PLAYWRIGHT_BROWSERS_PATH=... \
#     databackup-dev node node_modules/playwright-core/cli.js install ffmpeg
export PLAYWRIGHT_BROWSERS_PATH="$REPO/course/04-视频/tools/ms-playwright"
if [ ! -x "$PLAYWRIGHT_BROWSERS_PATH"/ffmpeg-*/ffmpeg-linux ]; then
  echo "FATAL: 缺少 Playwright ffmpeg（recordVideo 编码用）；先执行："
  echo "  node node_modules/playwright-core/cli.js install ffmpeg"
  echo "  （PLAYWRIGHT_BROWSERS_PATH=$PLAYWRIGHT_BROWSERS_PATH）"
  exit 1
fi
mkdir -p "$HOME" "$XDG_CONFIG_HOME"

cd "$CLIENT" || exit 1

run_probe() { # <w> <h> <label> [fps]
  echo
  echo "==================== 录屏探针 $1x$2 ($3) ===================="
  PROBE_VIDEO_W="$1" PROBE_VIDEO_H="$2" PROBE_VIDEO_FPS="${4:-}" \
    PROBE_VIDEO_DIR="$PROBE_OUT/raw-$3" \
    node e2e/record-demo/probe-record.mjs
  local code=$?
  [ $code -ne 0 ] && { echo "probe $3 FAILED (exit $code)"; RC=$code; }
}

run_probe 1120 720 1120x720
run_probe 2240 1440 2240x1440
# 可选：验证 recordVideo.fps 显式 30 时容器帧率是否真的变 30（PROBE_FPS30=1 时跑）。
[ "${PROBE_FPS30:-0}" = "1" ] && run_probe 1120 720 1120x720-fps30 30

echo
echo '==================== 反证：root 不加 --no-sandbox ===================='
timeout 40 node e2e/record-demo/probe-no-sandbox-check.mjs

echo
echo '==================== 产物 ===================='
ls -l "$PROBE_OUT"/raw-*/ 2>/dev/null
echo '==================== probe 完成 rc='"$RC"' ===================='
exit "$RC"
