#!/usr/bin/env bash
# 客户端 4 段录制 · 容器内入口（Xvfb + 环境变量 + 逐段执行）。
#
# 由 run-in-docker.sh（宿主 Git Bash）调用；也可在容器里手动跑：
#   SEG_MODE=rehearsal SEG_PASS=1 bash client/e2e/record-demo/record-all.sh
#   SEG_MODE=formal                  bash client/e2e/record-demo/record-all.sh
#
# 环境变量：
#   SEG_IDS          要跑的段（空格分隔），默认 4 段全跑
#   SEG_MODE         rehearsal | formal（默认 rehearsal）
#   SEG_PASS         彩排轮次（1/2；正式录制固定 1）
#   SEG_K_MS         覆盖时间轴偏移 kMs（默认自动读彩排 JSON）
#   SEG_CLOSE_MARGIN_MS  目标时长之外的录制余量（默认 1500）
#   SEG_DISPLAY      X display，默认 :98
#
# 产物：
#   rehearsal → course/04-视频/work/rehearsal/<id>-rehearsal-pass<N>.{webm,json}
#   formal    → course/04-视频/work/seg/<id>.webm|json + seg/annotation-boxes.json
#   seed 日志  → course/04-视频/work/seed/*.json
set -uo pipefail

REPO=/workspace
CLIENT="$REPO/client"
WORK="$REPO/course/04-视频/work"
IDS=${SEG_IDS:-"seg-02-setup seg-03-backup seg-04-manage seg-05-restore"}
MODE=${SEG_MODE:-rehearsal}
PASS=${SEG_PASS:-1}
DISPLAY_NUM=${SEG_DISPLAY:-:98}
DISP_NUM="${DISPLAY_NUM#:}"
LOGDIR="$WORK/logs"
RC=0

echo '==================== 客户端段录制 ===================='
echo "date:       $(date -Is)"
echo "node:       $(node --version)"
echo "uid:        $(id -u) ($(id -un))"
echo "mode:       $MODE (pass $PASS)"
echo "segments:   $IDS"
echo "display:    $DISPLAY_NUM"
echo "electron:   $(node -e "console.log(require('$CLIENT/node_modules/electron/package.json').version)" 2>&1)"
echo "playwright: $(node -e "console.log(require('$CLIENT/node_modules/@playwright/test/package.json').version)" 2>&1)"
echo "backend:    $($REPO/backend/bin/databackup --version 2>&1)"
echo "pw browsers:${PLAYWRIGHT_BROWSERS_PATH:-<unset>}"
echo "nproc:      $(nproc)"

[ -x "$CLIENT/node_modules/electron/dist/electron" ] || { echo 'FATAL: 缺少 electron 二进制（node_modules 卷没挂对？）'; exit 1; }
[ -x "$REPO/backend/bin/databackup" ] || { echo 'FATAL: 缺少 backend/bin/databackup（先 make backend）'; exit 1; }

export DISPLAY="$DISPLAY_NUM"
export HOME=/home/demo
export XDG_CONFIG_HOME=/home/demo/.config
export DATABACKUP_BIN="$REPO/backend/bin/databackup"
# recordVideo 依赖 Playwright 自带 ffmpeg；不设这项会让页面初始化挂死（PROBE-infra §B0）
export PLAYWRIGHT_BROWSERS_PATH="$REPO/course/04-视频/tools/ms-playwright"
if ! ls "$PLAYWRIGHT_BROWSERS_PATH"/ffmpeg-*/ffmpeg-linux >/dev/null 2>&1; then
  echo "FATAL: 缺少 Playwright ffmpeg（recordVideo 编码用）：$PLAYWRIGHT_BROWSERS_PATH/ffmpeg-*/ffmpeg-linux"
  exit 1
fi
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$WORK" "$LOGDIR" \
  "$WORK/seg" "$WORK/rehearsal" "$WORK/seed"

# ---------- Xvfb（2560x1600：1120x720 窗口 × dsf2 = 2240x1440 物理像素必须放得下） ----------
XVFB_PID=''
if [ -e "/tmp/.X11-unix/X${DISP_NUM}" ]; then
  echo "Xvfb $DISPLAY_NUM already running（复用）"
else
  Xvfb "$DISPLAY_NUM" -screen 0 2560x1600x24 -nolisten tcp >/tmp/xvfb-record.log 2>&1 &
  XVFB_PID=$!
  trap '[ -n "$XVFB_PID" ] && kill "$XVFB_PID" 2>/dev/null' EXIT
  for _ in $(seq 1 100); do
    [ -e "/tmp/.X11-unix/X${DISP_NUM}" ] && break
    sleep 0.1
  done
  if [ -e "/tmp/.X11-unix/X${DISP_NUM}" ]; then
    echo "Xvfb ready on $DISPLAY_NUM"
  else
    echo 'FATAL: Xvfb 没起来'; cat /tmp/xvfb-record.log; exit 1
  fi
fi

cd "$CLIENT" || exit 1

RESULTS=()
for id in $IDS; do
  SEGLOG="$LOGDIR/${id}-${MODE}-pass${PASS}-$(date +%Y%m%d-%H%M%S).log"
  echo
  echo "==================== $id ($MODE pass$PASS) ===================="
  SEG_MODE="$MODE" SEG_PASS="$PASS" node "e2e/record-demo/segments/${id}.mjs" >"$SEGLOG" 2>&1
  code=$?
  tail -n 30 "$SEGLOG"
  if [ $code -ne 0 ]; then
    echo "!! $id FAILED (exit $code)；完整日志：$SEGLOG"
    RC=$code
  fi
  RESULTS+=("$id exit=$code log=$SEGLOG")
done

echo
echo '==================== 产物 ===================='
if [ "$MODE" = 'formal' ]; then
  ls -l "$WORK/seg"/*.webm "$WORK/seg"/*.json 2>/dev/null
else
  ls -l "$WORK/rehearsal"/*pass"$PASS".webm "$WORK/rehearsal"/*pass"$PASS".json 2>/dev/null
fi
echo '==================== 汇总 ===================='
for r in "${RESULTS[@]}"; do echo "$r"; done
echo "record-all rc=$RC"
exit "$RC"
