#!/usr/bin/env bash
# e2e-display.sh — 在无 DISPLAY 的环境（Docker、CI）为 Electron e2e 自起 Xvfb。
#
# 不用 xvfb-run：它与 X server 的就绪握手依赖 SIGUSR1，信号在 trap 生效前到达会
# 丢失，wait 永久阻塞（Debian #866636，长期已知且随机器速度波动），实测在开发
# 容器内间歇性挂起，清理阶段也可能卡。改为直接管理 Xvfb 进程：就绪探测轮询
# /tmp/.X11-unix/X<n> socket（镜像未装 x11-utils，无 xdpyinfo 可用），退出时 kill。
#
# 用法：scripts/e2e-display.sh <command...>  — 已有 DISPLAY 时直接 exec 命令。
set -euo pipefail

if [ -n "${DISPLAY:-}" ]; then
  exec "$@"
fi

command -v Xvfb >/dev/null 2>&1 || { echo "e2e-display.sh: Xvfb 不可用（容器内应安装 xvfb 包）" >&2; exit 1; }

display=99
while [ -e "/tmp/.X11-unix/X${display}" ]; do
  display=$((display + 1))
done

Xvfb ":${display}" -screen 0 1280x800x24 -nolisten tcp >/dev/null 2>&1 &
xvfb_pid=$!
cleanup() { kill "${xvfb_pid}" 2>/dev/null || true; }
trap cleanup EXIT INT TERM

export DISPLAY=":${display}"

# 就绪探测：socket 出现后再留 0.3s 余量（Xvfb 建监听先于应用层完全就绪）。
ready=""
for _ in $(seq 1 100); do
  if [ -e "/tmp/.X11-unix/X${display}" ]; then ready=1; break; fi
  if ! kill -0 "${xvfb_pid}" 2>/dev/null; then
    echo "e2e-display.sh: Xvfb 启动即退出" >&2
    exit 1
  fi
  sleep 0.1
done
if [ -z "${ready}" ]; then
  echo "e2e-display.sh: Xvfb :${display} 10s 内未就绪" >&2
  exit 1
fi
sleep 0.3

# 不用 exec：保留 trap，命令结束后由 cleanup 回收 Xvfb；命令退出码原样透传。
"$@"
