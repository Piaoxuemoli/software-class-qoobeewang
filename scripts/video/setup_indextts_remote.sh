#!/usr/bin/env bash
# IndexTTS-2.5 部署脚本（租用 Linux + NVIDIA 主机，实测可复现）
#
# 实测环境：AutoDL 容器 / Ubuntu 22.04.4 / RTX 4090 24GB / 驱动 595.71.05 / CUDA 13.2
# 实测日期：2026-10-18。其它 8–24GB 卡同样适用（显存占用见 docs/plans/video-phase1.md）。
#
# 用法（在 GPU 主机上执行，仓库不需要出现在 GPU 机器上）：
#   bash scripts/video/setup_indextts_remote.sh
# 可选环境变量：
#   ROOT       工作根目录（代码/权重/缓存都放这里），默认 /root/autodl-tmp（AutoDL 数据盘）
#   GH_PROXY   拉 GitHub 时用的代理前缀，默认 https://gh-proxy.com
#   SKIP_SYNC=1 跳过 uv sync（已装过环境时）
#
# 国内网络下的三个必踩坑（脚本已规避）：
#   1) github.com 直连超时 → 先 source /etc/network_turbo，再退 gh-proxy；
#   2) `uv python install 3.11` 要拉 python-build-standalone（GitHub 大文件），会长时间卡住
#      → 改用 conda（TUNA 镜像）建 Python 3.11，再让 uv 用这个解释器；
#   3) modelscope CLI 的参数是 `--local_dir`（下划线），写 `--local-dir` 会报
#      "unrecognized arguments"。
set -euo pipefail

ROOT="${ROOT:-/root/autodl-tmp}"
GH_PROXY="${GH_PROXY:-https://gh-proxy.com}"
REPO="$ROOT/index-tts"
PY_DIR="$ROOT/py311"
export PATH="$HOME/.local/bin:/root/miniconda3/bin:$PATH"
export UV_CACHE_DIR="$ROOT/uv-cache"
export PIP_INDEX_URL="${PIP_INDEX_URL:-https://mirrors.aliyun.com/pypi/simple}"

mkdir -p "$ROOT"
echo "== 0. 环境自检 =="
nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
python3 --version 2>/dev/null || true
df -h "$ROOT" | tail -1

echo "== 1. uv（astral.sh 常卡在 GitHub，直接走 pip 镜像最稳）=="
if ! command -v uv >/dev/null 2>&1; then
  if command -v pip3 >/dev/null 2>&1; then
    pip3 install -q uv -i "$PIP_INDEX_URL"
  else
    /root/miniconda3/bin/pip install -q uv -i "$PIP_INDEX_URL"
  fi
fi
uv --version

echo "== 2. 代码（直连 → network_turbo → gh-proxy 三级回退）=="
if [ ! -d "$REPO/.git" ]; then
  if ! git clone --depth 1 https://github.com/index-tts/index-tts.git "$REPO" 2>/dev/null; then
    rm -rf "$REPO"
    if [ -f /etc/network_turbo ]; then
      # shellcheck disable=SC1091
      source /etc/network_turbo
      git clone --depth 1 https://github.com/index-tts/index-tts.git "$REPO" || {
        rm -rf "$REPO"; git clone --depth 1 "$GH_PROXY/https://github.com/index-tts/index-tts.git" "$REPO"; }
      unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
    else
      git clone --depth 1 "$GH_PROXY/https://github.com/index-tts/index-tts.git" "$REPO"
    fi
  fi
fi
git -C "$REPO" log -1 --format='%H %cd %s'

echo "== 3. Python 3.11（conda 镜像比 uv python install 快得多）=="
if [ ! -x "$PY_DIR/bin/python" ]; then
  /root/miniconda3/bin/conda create -y -p "$PY_DIR" python=3.11
fi
"$PY_DIR/bin/python" --version

echo "== 4. uv sync（torch 等大包走阿里云，约 5–10 分钟）=="
if [ "${SKIP_SYNC:-0}" != "1" ]; then
  ( cd "$REPO"
    unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
    uv sync --all-extras --python "$PY_DIR/bin/python" --default-index "$PIP_INDEX_URL"
  )
fi

echo "== 5. 权重 IndexTTS-2.5（约 5.3GB；注意 --local_dir 是下划线）=="
if [ ! -f "$REPO/checkpoints/gpt.pth" ]; then
  python3 -m pip install -q modelscope -i "$PIP_INDEX_URL" 2>/dev/null || \
    /root/miniconda3/bin/pip install -q modelscope -i "$PIP_INDEX_URL"
  ( cd "$REPO" && /root/miniconda3/bin/modelscope download \
      --model IndexTeam/IndexTTS-2.5 --local_dir "$REPO/checkpoints" )
fi
du -sh "$REPO/checkpoints"

echo "== 6. GPU 自检 =="
( cd "$REPO" && uv run tools/gpu_check.py ) || true

echo "== 7. 冒烟合成（换成自己的参考音频与文本）=="
: "${REF:=$ROOT/ref/main-ref.wav}"
if [ -f "$REF" ]; then
  ( cd "$REPO" && uv run python "$ROOT/tools/tts_batch_indextts.py" \
      --ref "$REF" --text "DataBackup 把 Linux 目录树备份成一个文件，并原样还原。" \
      --out-dir "$ROOT/smoke" --duration-factor 1.0 )
else
  echo "（跳过：没有 $REF；把参考音频放好后重跑，或用仓库自带 examples/）"
fi

echo "== DONE。批量旁白与语速统计见 scripts/video/tts_batch_indextts.py 与 measure_rate.py =="
