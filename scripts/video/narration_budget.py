#!/usr/bin/env python3
"""旁白时长预算与实measurement校核（P2）。

两件事：
1) **预算**：从 `docs/deliverables/video-storyboard.json` 读 7 镜的镜头时长与旁白文本，
   按目标语速（默认 4.4 非空白字符/秒）算每镜旁白预算，并检查旁白总时长 ≤ 80 s；
2) **实测**：给了 `--wav-dir` 时，用逐句 WAV 的真实时长替换预算值，逐镜校核：
   - 旁白超过镜头时长（留 `--margin` 秒余量）→ FAIL，必须改文案或调语速；
   - 旁白占镜头 < 40% → WARN（镜头里空等太久，考虑补画面/延长注解）；
   - 总旁白 > 80 s → FAIL（视频硬约束是成片 ≤180 s）。

用法::

    # 只看预算（还没合成音频时）
    python scripts/video/narration_budget.py

    # 合成后校核（seg_00.wav … seg_06.wav）
    python scripts/video/narration_budget.py \
        --wav-dir course/04-视频/voice/narration --json-out course/04-视频/voice/narration/budget.json

退出码：0 = 全部通过；1 = 有 FAIL 项；2 = 输入/参数错误。
"""
from __future__ import annotations

import argparse
import glob
import json
import os
import re
import sys
import wave

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

TARGET_RATE = 4.4  # 分镜文档 §5 的语速口径（非空白字符/秒）


def count_chars(text: str) -> int:
    """非空白字符数（汉字 + 英文 + 数字 + 标点），与分镜 §5 预算口径一致。"""
    return len(re.sub(r"\s+", "", text))


def wav_duration(path: str) -> float | None:
    try:
        with wave.open(path, "rb") as wf:
            return wf.getnframes() / float(wf.getframerate())
    except (OSError, wave.Error):
        return None


def main() -> int:
    ap = argparse.ArgumentParser(description="旁白时长预算 / 实测校核")
    ap.add_argument(
        "--storyboard",
        default="docs/deliverables/video-storyboard.json",
        help="分镜 JSON（默认 docs/deliverables/video-storyboard.json）",
    )
    ap.add_argument("--wav-dir", help="逐句 WAV 目录（seg_00.wav … seg_06.wav）")
    ap.add_argument("--glob", default="seg_*.wav")
    ap.add_argument("--rate", type=float, default=TARGET_RATE, help="预算语速（非空白字符/秒）")
    ap.add_argument("--narration-max", type=float, default=80.0, help="旁白总时长上限（秒）")
    ap.add_argument("--video-max", type=float, default=180.0, help="成片上限（秒）")
    ap.add_argument("--margin", type=float, default=0.8, help="每镜至少要留的余量（秒）")
    ap.add_argument("--json-out", help="结果写成 JSON")
    args = ap.parse_args()

    try:
        with open(args.storyboard, encoding="utf-8") as fh:
            board = json.load(fh)
    except OSError as exc:
        print(f"!! 读不到分镜 JSON：{exc}", file=sys.stderr)
        return 2
    shots = board.get("shots") or []
    if not shots:
        print("!! 分镜 JSON 里没有 shots", file=sys.stderr)
        return 2

    wavs: list[str] = []
    if args.wav_dir:
        wavs = sorted(glob.glob(os.path.join(args.wav_dir, args.glob)))
        if len(wavs) != len(shots):
            print(
                f"!! {args.wav_dir} 下 {len(wavs)} 个 wav，与 {len(shots)} 个镜头不匹配，按镜头数截断/补空",
                file=sys.stderr,
            )

    rows: list[dict] = []
    fails = 0
    warns = 0
    total_narration = 0.0
    total_video = 0.0

    print(
        f"{'镜':<4}{'镜头(s)':>8}{'字数':>6}{'预算(s)':>9}{'实测(s)':>9}"
        f"{'字/秒':>7}{'余量(s)':>9}  结论"
    )
    print("-" * 88)
    for idx, shot in enumerate(shots):
        shot_id = shot.get("id", f"S{idx}")
        shot_dur = float(shot.get("duration", 0.0))
        text = (shot.get("narration") or "").strip()
        chars = count_chars(text)
        budget = chars / args.rate if args.rate > 0 else 0.0
        measured = None
        if idx < len(wavs):
            measured = wav_duration(wavs[idx])
        actual = measured if measured is not None else budget
        rate = chars / actual if actual > 0 else 0.0
        margin = shot_dur - actual

        verdict = "OK"
        if margin < args.margin:
            verdict = "FAIL 旁白超出镜头余量"
            fails += 1
        elif actual < shot_dur * 0.4:
            verdict = "WARN 旁白占比过低"
            warns += 1

        print(
            f"{shot_id:<4}{shot_dur:>8.1f}{chars:>6}{budget:>9.2f}"
            f"{(f'{measured:.2f}' if measured is not None else '-'):>9}"
            f"{rate:>7.2f}{margin:>9.2f}  {verdict}"
        )
        total_narration += actual
        total_video += shot_dur
        rows.append(
            {
                "id": shot_id,
                "title": shot.get("title"),
                "shot_duration": shot_dur,
                "chars": chars,
                "budget_duration": round(budget, 3),
                "measured_duration": None if measured is None else round(measured, 3),
                "chars_per_sec": round(rate, 3),
                "margin": round(margin, 3),
                "verdict": verdict,
                "wav": os.path.basename(wavs[idx]) if idx < len(wavs) else None,
            }
        )

    print("-" * 88)
    print(
        f"旁白合计 {total_narration:.2f} s（上限 {args.narration_max:.0f} s，"
        f"实测语速 {sum(r['chars'] for r in rows) / total_narration if total_narration else 0:.2f} 字/秒）"
    )
    print(f"画面合计 {total_video:.2f} s（成片上限 {args.video_max:.0f} s）")

    if total_narration > args.narration_max:
        print(f"!! 旁白总时长超出上限 {args.narration_max:.0f}s：优先删修饰词改文案，其次调快语速", file=sys.stderr)
        fails += 1
    if total_video > args.video_max:
        print(f"!! 画面总时长超出成片上限 {args.video_max:.0f}s", file=sys.stderr)
        fails += 1

    if args.json_out:
        payload = {
            "storyboard": os.path.abspath(args.storyboard),
            "target_rate": args.rate,
            "shots": rows,
            "total_narration": round(total_narration, 3),
            "total_video": round(total_video, 3),
            "narration_max": args.narration_max,
            "video_max": args.video_max,
            "fails": fails,
            "warns": warns,
        }
        os.makedirs(os.path.dirname(os.path.abspath(args.json_out)) or ".", exist_ok=True)
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2)
        print(f"明细已写入 {args.json_out}")

    if fails:
        print(f"结论：{fails} 项 FAIL，{warns} 项 WARN —— 先改文案再合成", file=sys.stderr)
        return 1
    print(f"结论：通过（{warns} 项 WARN 仅提示）")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
