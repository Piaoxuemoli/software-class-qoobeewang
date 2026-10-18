#!/usr/bin/env python3
"""按分镜时间轴把逐镜旁白铺成整片音轨（P3 合成第一步）。

- 输入：`docs/deliverables/video-storyboard.json` 的镜头起点，`course/04-视频/voice/narration/seg_XX.wav`；
- 规则：每镜旁白从「镜头起点 + offset」开始（S0 默认 +0.2 s，其余 +0.3 s，可在 placements 里核对）；
- 输出：整片等长静音轨 + 旁白（22050 Hz/mono/s16 原采样率，P3 再统一 48k/24bit + loudnorm），
  以及 `placements.json`（每镜绝对起止、是否撞到镜头末尾）。
- 只依赖标准库（wave/array），宿主无需 ffmpeg。

用法::

    python scripts/video/build_audio.py \
        --out course/04-视频/work/audio/narration-full.wav \
        --placements-out course/04-视频/work/audio/placements.json

退出码：0 = 通过；1 = 有旁白超出镜头；2 = 输入错误。
"""
from __future__ import annotations

import argparse
import array
import json
import os
import sys
import wave

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass


def read_mono(path: str) -> tuple[array.array, int]:
    with wave.open(path, "rb") as wf:
        if wf.getsampwidth() != 2:
            raise SystemExit(f"{path}: 需要 16-bit PCM，实际 {wf.getsampwidth() * 8} bit")
        chans = wf.getnchannels()
        sr = wf.getframerate()
        raw = wf.readframes(wf.getnframes())
    data = array.array("h")
    data.frombytes(raw)
    if sys.byteorder == "big":
        data.byteswap()
    if chans > 1:
        mono = array.array("h", [0]) * (len(data) // chans)
        for i in range(len(mono)):
            mono[i] = int(sum(data[i * chans : (i + 1) * chans]) / chans)
        data = mono
    return data, sr


def main() -> int:
    ap = argparse.ArgumentParser(description="把逐镜旁白铺成整片音轨")
    ap.add_argument("--storyboard", default="docs/deliverables/video-storyboard.json")
    ap.add_argument("--narration-dir", default="course/04-视频/voice/narration")
    ap.add_argument("--out", required=True, help="输出整片 wav（与旁白同采样率）")
    ap.add_argument("--placements-out", help="输出 placements.json")
    ap.add_argument("--offset", type=float, default=0.3, help="默认旁白相对镜头起点的偏移（秒）")
    ap.add_argument("--offset-s0", type=float, default=0.2, help="S0 的偏移（秒）")
    ap.add_argument("--sample-rate", type=int, default=22050)
    ap.add_argument("--tail-pad", type=float, default=0.0, help="整轨末尾补多少秒静音")
    args = ap.parse_args()

    try:
        with open(args.storyboard, encoding="utf-8") as fh:
            board = json.load(fh)
    except OSError as exc:
        print(f"!! 读不到分镜 JSON：{exc}", file=sys.stderr)
        return 2
    shots = board.get("shots") or []
    if not shots:
        print("!! 分镜里没有 shots", file=sys.stderr)
        return 2

    sr = args.sample_rate
    total_sec = sum(float(s.get("duration", 0.0)) for s in shots) + args.tail_pad
    total_frames = int(round(total_sec * sr))
    track = array.array("h", [0]) * total_frames

    placements = []
    problems: list[str] = []
    for idx, shot in enumerate(shots):
        shot_id = shot.get("id", f"S{idx}")
        start = float(shot.get("start", 0.0))
        duration = float(shot.get("duration", 0.0))
        offset = args.offset_s0 if shot_id == "S0" else args.offset
        wav_path = os.path.join(args.narration_dir, f"seg_{idx:02d}.wav")
        if not os.path.isfile(wav_path):
            problems.append(f"{shot_id}: 缺旁白 {wav_path}")
            placements.append({"id": shot_id, "wav": wav_path, "missing": True})
            continue
        samples, wav_sr = read_mono(wav_path)
        if wav_sr != sr:
            problems.append(f"{shot_id}: {wav_path} 采样率 {wav_sr} 与期望 {sr} 不一致")
        speech_sec = len(samples) / float(wav_sr)
        at = start + offset
        end = at + speech_sec
        if end > start + duration:
            problems.append(
                f"{shot_id}: 旁白 {speech_sec:.2f}s 从 {at:.2f}s 开始会超出镜头末尾 "
                f"{start + duration:.2f}s（镜头 {duration:.2f}s）"
            )
        begin_frame = int(round(at * sr))
        for i, value in enumerate(samples):
            pos = begin_frame + i
            if 0 <= pos < total_frames:
                track[pos] = value
        placements.append(
            {
                "id": shot_id,
                "wav": os.path.basename(wav_path),
                "shot_start": round(start, 3),
                "shot_duration": round(duration, 3),
                "narration_start": round(at, 3),
                "narration_end": round(end, 3),
                "narration_duration": round(speech_sec, 3),
                "fits": end <= start + duration,
            }
        )

    os.makedirs(os.path.dirname(os.path.abspath(args.out)) or ".", exist_ok=True)
    with wave.open(args.out, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sr)
        wf.writeframes(track.tobytes())

    if args.placements_out:
        os.makedirs(os.path.dirname(os.path.abspath(args.placements_out)) or ".", exist_ok=True)
        with open(args.placements_out, "w", encoding="utf-8") as fh:
            json.dump(
                {
                    "storyboard": os.path.abspath(args.storyboard),
                    "sample_rate": sr,
                    "total_seconds": round(total_frames / sr, 3),
                    "placements": placements,
                    "problems": problems,
                },
                fh,
                ensure_ascii=False,
                indent=2,
            )

    print(f"{'镜':<4}{'镜头起':>8}{'旁白起':>8}{'旁白(s)':>9}{'旁白止':>8}{'镜头止':>8}  结论")
    print("-" * 60)
    for p in placements:
        if p.get("missing"):
            print(f"{p['id']:<4} 缺旁白 {p['wav']}")
            continue
        print(
            f"{p['id']:<4}{p['shot_start']:>8.2f}{p['narration_start']:>8.2f}"
            f"{p['narration_duration']:>9.2f}{p['narration_end']:>8.2f}"
            f"{p['shot_start'] + p['shot_duration']:>8.2f}  {'OK' if p['fits'] else 'FAIL'}"
        )
    print(f"\n整轨：{total_frames / sr:.2f}s / {sr} Hz / mono / 16-bit → {args.out}")
    if problems:
        print(f"\n!! {len(problems)} 个问题：", file=sys.stderr)
        for p in problems:
            print("  - " + p, file=sys.stderr)
        return 1
    print("全部旁白都在镜头时长内")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
