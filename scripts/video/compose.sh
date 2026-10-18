#!/usr/bin/env bash
# DataBackup 演示视频 P3 合成脚本（Windows Git Bash + course/04-视频/tools 便携 ffmpeg）
#
# 输入（全部来自 P2 录制与固化产物）：
#   course/04-视频/work/seg/seg-XX-*.webm       7 段无声录制（客户端 2240x1440 / 卡片 1920x1080，30fps）
#   course/04-视频/work/ass/demo.ass            ASS 字幕+注解（scripts/video/gen_ass.py 生成）
#   course/04-视频/work/audio/narration-full.wav 整片旁白（scripts/video/build_audio.py 生成）
#   course/04-视频/tools/fonts/                 Noto Sans CJK（ASS 烧录用）
# 输出：
#   course/04-视频/work/compose/seg-XX.mp4      每段 1920x1080/30fps 无声
#   course/04-视频/work/compose/video-full.mp4  拼接后的无声全片
#   course/04-视频/work/compose/audio-final.wav 48kHz/24bit、响度 -16 LUFS 的旁白轨
#   course/04-视频/DataBackup-demo-1.0.1.mp4    成片（H.264 + AAC，faststart）
#
# 用法：
#   bash scripts/video/compose.sh              # 全流程
#   bash scripts/video/compose.sh audio        # 只做音频（验证 loudnorm）
#   bash scripts/video/compose.sh video        # 只做逐段编码 + 拼接
#   bash scripts/video/compose.sh final        # 只做最终混流（烧 ASS + 混音）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

FF="course/04-视频/tools/ffmpeg/bin/ffmpeg.exe"
FP="course/04-视频/tools/ffmpeg/bin/ffprobe.exe"
SEG="${SEG_DIR:-course/04-视频/work/seg}"
WORK="course/04-视频/work/compose"
AUD="course/04-视频/work/audio"
ASS="course/04-视频/work/ass/demo.ass"
FONTS="course/04-视频/tools/fonts"
FINAL="course/04-视频/DataBackup-demo-1.0.1.mp4"
STAGE="${1:-all}"

# 镜号:段名:时长:类型（app=2240x1440 需缩放补边；card=1920x1080 原样）
SHOTS=(
  "seg-00-title:9:card"
  "seg-01-data:20:card"
  "seg-02-setup:37:app"
  "seg-03-backup:28:app"
  "seg-04-manage:25:app"
  "seg-05-restore:35:app"
  "seg-06-verify:24:card"
)

mkdir -p "$WORK"

check_inputs() {
  local need="${1:-all}"
  local missing=0
  if [ "$need" = "all" ] || [ "$need" = "video" ]; then
    for spec in "${SHOTS[@]}"; do
      IFS=: read -r name _dur _kind <<<"$spec"
      if [ ! -f "$SEG/$name.webm" ]; then
        echo "!! 缺录制段：$SEG/$name.webm" >&2
        missing=1
      fi
    done
  fi
  if [ "$need" = "all" ] || [ "$need" = "audio" ]; then
    [ -f "$AUD/narration-full.wav" ] || { echo "!! 缺旁白整轨：$AUD/narration-full.wav（先跑 build_audio.py）" >&2; missing=1; }
  fi
  if [ "$need" = "final" ]; then
    [ -f "$WORK/video-full.mp4" ] || { echo "!! 缺拼接全片：$WORK/video-full.mp4（先跑 video 阶段）" >&2; missing=1; }
    [ -f "$WORK/audio-final.wav" ] || { echo "!! 缺归一音频：$WORK/audio-final.wav（先跑 audio 阶段）" >&2; missing=1; }
    [ -f "$ASS" ] || { echo "!! 缺 ASS：$ASS（先跑 gen_ass.py）" >&2; missing=1; }
  fi
  return "$missing"
}

# 读取某段录制 json 里的「内容起点偏移」（录像器启动 → 应用首帧之间的黑场），没有则 0
offset_of() {
  python - "$1" <<'PY'
import json, sys
try:
    with open(sys.argv[1], encoding="utf-8") as fh:
        d = json.load(fh)
except Exception:
    print("0")
    raise SystemExit(0)
v = (d.get("timing") or {}).get("contentOffsetSec")
if v is None:
    v = d.get("contentStartSec") or d.get("contentStart") or 0
print(round(float(v), 3))
PY
}

encode_segments() {
  echo "== 1) 逐段编码 1920x1080/30fps（按 json 内容起点裁黑场，再按分镜时长截断）=="
  local list="$WORK/concat.txt"
  : >"$list"
  for spec in "${SHOTS[@]}"; do
    IFS=: read -r name dur kind <<<"$spec"
    local src="$SEG/$name.webm"
    local dst="$WORK/$name.mp4"
    local off="0"
    [ -f "$SEG/$name.json" ] && off="$(offset_of "$SEG/$name.json")"
    if [ "$kind" = "app" ]; then
      # 2240x1440 → 1680x1080（等比 1.5556），左右各补 120 黑边
      "$FF" -y -v warning -ss "$off" -i "$src" \
        -vf "scale=1680:1080:flags=lanczos,pad=1920:1080:120:0:black,fps=30,format=yuv420p" \
        -t "$dur" -c:v libx264 -crf 18 -preset medium -profile:v high -level 4.1 -an "$dst"
    else
      # 卡片段原生 1920x1080
      "$FF" -y -v warning -ss "$off" -i "$src" \
        -vf "fps=30,scale=1920:1080:flags=lanczos,format=yuv420p" \
        -t "$dur" -c:v libx264 -crf 18 -preset medium -profile:v high -level 4.1 -an "$dst"
    fi
    local got
    got="$("$FP" -v error -show_entries format=duration -of csv=p=0 "$dst")"
    echo "file $name.mp4" >>"$list"
    echo "   $name  offset=${off}s  目标 ${dur}s  实得 ${got}s  $(du -h "$dst" | cut -f1)"
  done

  echo "== 2) 拼接（-c copy，参数已统一）=="
  "$FF" -y -v warning -f concat -safe 0 -i "$list" -c copy "$WORK/video-full.mp4"
  "$FP" -v error -show_entries format=duration -show_entries stream=width,height,r_frame_rate,codec_name \
    -of default=nw=1 "$WORK/video-full.mp4"
}

build_audio() {
  echo "== 3) 旁白响度归一（两遍 loudnorm，-16 LUFS / TP -1.5 dBFS）=="
  local metrics="$WORK/loudnorm.json"
  # 注意：loudnorm 的 print_format=json 统计是 INFO 级日志，不能用 -v error，否则抓不到
  "$FF" -y -hide_banner -nostats -i "$AUD/narration-full.wav" \
    -af "loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json" -f null - 2>"$metrics" || true
  # loudnorm 的 json 在 stderr 末尾；用 python 解析（宿主机有 python）
  local args
  args="$(python - "$metrics" <<'PY'
import json, re, sys
text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
m = re.findall(r"\{[^{}]*\"input_i\"[^{}]*\}", text, re.S)
if not m:
    print("SINGLE_PASS")
    raise SystemExit(0)
d = json.loads(m[-1])
print(
    "loudnorm=I=-16:TP=-1.5:LRA=11"
    f":measured_I={d['input_i']}:measured_TP={d['input_tp']}"
    f":measured_LRA={d['input_lra']}:measured_thresh={d['input_thresh']}"
    f":offset={d['target_offset']}:linear=true"
)
PY
)"
  if [ "$args" = "SINGLE_PASS" ]; then
    echo "   [warn] 解析不到 loudnorm 统计，退回单遍" >&2
    args="loudnorm=I=-16:TP=-1.5:LRA=11"
  fi
  "$FF" -y -v warning -i "$AUD/narration-full.wav" \
    -af "$args,aresample=48000" -ar 48000 -c:a pcm_s24le "$WORK/audio-final.wav"
  echo "   -> $WORK/audio-final.wav"
}

final_mux() {
  echo "== 4) 最终混流（烧 ASS + 混音 + faststart）=="
  "$FF" -y -v warning -i "$WORK/video-full.mp4" -i "$WORK/audio-final.wav" \
    -vf "ass=${ASS}:fontsdir=${FONTS}" \
    -c:v libx264 -crf 18 -preset slow -profile:v high -level 4.1 -pix_fmt yuv420p \
    -c:a aac -b:a 192k -ar 48000 -ac 2 \
    -movflags +faststart -shortest "$FINAL"
  echo "   -> $FINAL"
}

qc() {
  echo "== 5) 成片 QC =="
  "$FP" -v error -show_entries format=duration,size,bit_rate \
    -show_entries stream=codec_name,width,height,r_frame_rate,sample_rate,channels \
    -of default=nw=1 "$FINAL"
  "$FF" -hide_banner -i "$FINAL" -af "ebur128=peak=true" -f null - 2>&1 | tail -12 | sed -n '1,12p'
}

case "$STAGE" in
  audio) check_inputs audio || exit 1; build_audio ;;
  video) check_inputs video || exit 1; encode_segments ;;
  final) check_inputs final || exit 1; final_mux ;;
  qc) qc ;;
  all)
    check_inputs all || exit 1
    encode_segments
    build_audio
    final_mux
    qc
    ;;
  *) echo "用法: bash scripts/video/compose.sh [all|audio|video|final|qc]" >&2; exit 2 ;;
esac
