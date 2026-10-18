#!/usr/bin/env bash
# DataBackup 演示视频 · 卡片段容器内一键脚本（databackup-dev 镜像内执行）
#
# 宿主入口（Git Bash，注意 MSYS_NO_PATHCONV=1 与 node_modules 卷名）：
#   cd "C:/Users/Qoobeewang/Desktop/软件工程实验"
#   export MSYS_NO_PATHCONV=1
#   VOL=$(bash scripts/docker.sh volumes | awk '/databackup-node-/{print $1; exit}')
#   docker run --rm \
#     -v "C:/Users/Qoobeewang/Desktop/软件工程实验:/workspace" -w /workspace \
#     -v "$VOL:/workspace/client/node_modules" \
#     -e DISPLAY= \
#     databackup-dev bash /workspace/client/e2e/record-demo/cards/run-in-docker.sh <mode>
#
# mode:
#   capture   预置 /home/demo + 跑真实命令/真实备份还原，采集 raw/commands.txt（不录屏）
#   build     由 raw/ 生成 scenes.json（真实输出 → 场景）
#   probe     录 4s 片头卡，验证 recordVideo 链路/尺寸/帧率/中文（先 headless 后 Xvfb headed）
#   rehearse  彩排：几何 JSON + 抽帧 PNG（不录正式视频）
#   record    正式录制 3 段 webm 到 course/04-视频/work/seg/
#   all       capture + build + rehearse
set -uo pipefail

REPO=/workspace
CARDS="$REPO/client/e2e/record-demo/cards"
WORK="$REPO/course/04-视频/work"
DISPLAY_NUM=:97
MODE="${1:-rehearse}"
[ $# -gt 0 ] && shift
EXTRA_ARGS=("$@")
RC=0
XVFB_PID=

export PLAYWRIGHT_BROWSERS_PATH="$REPO/course/04-视频/tools/ms-playwright"
export HOME=/root
export NODE_OPTIONS="${NODE_OPTIONS:-}"

echo '==================== 卡片段环境 ===================='
echo "date:        $(date -Is)"
echo "node:        $(node --version)"
echo "mode:        $MODE"
echo "display:     $DISPLAY_NUM"
echo "pw browsers: $PLAYWRIGHT_BROWSERS_PATH"
echo "zh fonts:    $(fc-list :lang=zh | wc -l) entries"
echo "mono fonts:  $(fc-list :spacing=mono family 2>/dev/null | sort -u | tr '\n' ';')"
echo "card:        $CARDS/card.html"
mkdir -p "$WORK/cards/raw" "$WORK/rehearsal/cards" "$WORK/seg"

cleanup() {
  [ -n "${XVFB_PID:-}" ] && kill "$XVFB_PID" 2>/dev/null
}
trap cleanup EXIT

start_xvfb() {
  # 与客户端组错开 display（客户端组用 :98）；尺寸 = 输出分辨率 1920×1080
  if [ -e "/tmp/.X11-unix/X${DISPLAY_NUM#:}" ]; then
    echo "Xvfb socket 已存在: /tmp/.X11-unix/X${DISPLAY_NUM#:}"
    return 0
  fi
  Xvfb "$DISPLAY_NUM" -screen 0 1920x1080x24 -nolisten tcp >/tmp/xvfb-cards.log 2>&1 &
  XVFB_PID=$!
  for _ in $(seq 1 100); do
    [ -e "/tmp/.X11-unix/X${DISPLAY_NUM#:}" ] && break
    sleep 0.1
  done
  if [ -e "/tmp/.X11-unix/X${DISPLAY_NUM#:}" ]; then
    echo "Xvfb ready on $DISPLAY_NUM (1920x1080x24, pid $XVFB_PID)"
    export DISPLAY="$DISPLAY_NUM"
  else
    echo 'FATAL: Xvfb 没起来'; cat /tmp/xvfb-cards.log; exit 1
  fi
}

require_chromium() {
  if ! ls -d "$PLAYWRIGHT_BROWSERS_PATH"/chromium-* >/dev/null 2>&1; then
    echo 'FATAL: 缺少 Playwright Chromium；先执行：'
    echo "  cd /workspace/client && PLAYWRIGHT_BROWSERS_PATH=$PLAYWRIGHT_BROWSERS_PATH \\"
    echo '    node node_modules/playwright-core/cli.js install chromium'
    exit 1
  fi
  if ! ls -d "$PLAYWRIGHT_BROWSERS_PATH"/ffmpeg-* >/dev/null 2>&1; then
    echo 'FATAL: 缺少 Playwright ffmpeg（recordVideo 硬前提，见 PROBE-infra.md §B0）'
    exit 1
  fi
}

case "$MODE" in
  capture)
    bash "$CARDS/capture-real-output.sh" || RC=$?
    ;;
  build)
    node "$CARDS/build-scenes.mjs" || RC=$?
    ;;
  probe)
    require_chromium
    # 说明：当前 Chromium 156 的 headless-shell 与 Playwright 的 --remote-debugging-pipe 不兼容
    # （"Headless commands are not compatible with remote debugging"），headless 模式无法启动；
    # 正式录制统一走 headed + Xvfb :97（与任务要求一致，也与客户端段的 Xvfb 链路同源）。
    start_xvfb
    CARD_HEADED=1 node "$CARDS/render-scenes.mjs" --mode probe || RC=$?
    ;;
  rehearse)
    require_chromium
    if [ "${CARD_HEADED:-0}" = "1" ]; then
      start_xvfb
    else
      export DISPLAY=  # 彩排默认 headless（正式录制实际用哪种模式，由 probe 结果决定后用 CARD_HEADED 覆盖）
    fi
    CARD_HEADED="${CARD_HEADED:-0}" node "$CARDS/render-scenes.mjs" --mode rehearse || RC=$?
    ;;
  record)
    require_chromium
    start_xvfb
    CARD_HEADED="${CARD_HEADED:-1}" node "$CARDS/render-scenes.mjs" --mode record "${EXTRA_ARGS[@]}" || RC=$?
    ;;
  all)
    bash "$CARDS/capture-real-output.sh" || RC=$?
    [ "$RC" -eq 0 ] && { node "$CARDS/build-scenes.mjs" || RC=$?; }
    [ "$RC" -eq 0 ] && { CARD_HEADED=0 node "$CARDS/render-scenes.mjs" --mode rehearse || RC=$?; }
    ;;
  *)
    echo "FATAL: 未知 mode '$MODE'（capture|build|probe|rehearse|record|all）"
    exit 2
    ;;
esac

echo "==================== 卡片段 $MODE 完成 rc=$RC ===================="
exit "$RC"
