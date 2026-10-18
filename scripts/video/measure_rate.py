#!/usr/bin/env python3
"""测量 TTS 逐句音频的语速（非空白字符/秒），对照分镜预算 4.2–4.6。

分镜文档里的“字数”口径是**非空白字符**（汉字 + 英文 + 数字 + 标点，不含空格），
本脚本沿用同一口径，这样旁白时长预算与实测可以直接相减。

用法::

    python scripts/video/measure_rate.py \\
        --lines scripts/video/tts-test-lines.txt \\
        --wav-dir course/04-视频/voice/p1test

    # 只看某几句 / 输出 JSON 供 QC 报告引用
    python scripts/video/measure_rate.py --lines ... --wav-dir ... --json-out rate.json

退出码：0 = 全部句子语速落在目标窗口或已给出 duration_factor 建议；
        1 = 有句子缺 wav；2 = 参数/文件错误。
"""
from __future__ import annotations

import argparse
import glob
import json
import os
import re
import sys
import wave

# Windows 控制台默认 cp936，中文输出会花屏；WSL 里无害。
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

# 分镜 §5 的语速窗口（汉字/秒）
TARGET_LO, TARGET_HI = 4.2, 4.6
TARGET_MID = (TARGET_LO + TARGET_HI) / 2


def read_lines(path: str) -> list[str]:
    lines: list[str] = []
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            lines.append(line)
    return lines


def count_chars(text: str) -> tuple[int, int]:
    """返回 (非空白字符数, 汉字数)。非空白字符数即分镜预算口径。"""
    chars = len(re.sub(r"\s+", "", text))
    cjk = len(re.findall(r"[\u4e00-\u9fff]", text))
    return chars, cjk


def wav_duration(path: str) -> float:
    with wave.open(path, "rb") as wf:
        nframes, rate = wf.getnframes(), wf.getframerate()
    if rate <= 0:
        raise ValueError(f"{path}: 非法采样率 {rate}")
    return nframes / float(rate)


def main() -> int:
    ap = argparse.ArgumentParser(description="测量逐句语速（非空白字符/秒）")
    ap.add_argument("--lines", required=True, help="逐句文本文件（UTF-8，# 注释）")
    ap.add_argument("--wav-dir", required=True, help="逐句 wav 目录（按文件名排序后与文本一一对应）")
    ap.add_argument("--glob", default="*.wav", help="wav 匹配模式（默认 *.wav）")
    ap.add_argument("--target", default=f"{TARGET_LO}-{TARGET_HI}", help="目标语速窗口")
    ap.add_argument("--json-out", help="把明细写成 JSON")
    ap.add_argument(
        "--factor-hint",
        action="store_true",
        help="额外打印每句建议的 duration_factor（IndexTTS：>1 变慢、<1 变快）",
    )
    args = ap.parse_args()

    try:
        lo, hi = (float(x) for x in args.target.split("-"))
    except ValueError:
        print(f"!! --target 需形如 4.2-4.6，收到 {args.target!r}", file=sys.stderr)
        return 2
    mid = (lo + hi) / 2

    try:
        lines = read_lines(args.lines)
    except OSError as exc:
        print(f"!! 读不到文本文件：{exc}", file=sys.stderr)
        return 2
    if not lines:
        print(f"!! {args.lines} 里没有有效句子", file=sys.stderr)
        return 2

    wavs = sorted(glob.glob(os.path.join(args.wav_dir, args.glob)))
    if not wavs:
        print(f"!! {args.wav_dir} 下没有匹配 {args.glob} 的文件", file=sys.stderr)
        return 2
    if len(wavs) != len(lines):
        print(
            f"!! 句子数 {len(lines)} 与 wav 数 {len(wavs)} 不一致，按较短的一侧统计",
            file=sys.stderr,
        )

    rows: list[dict] = []
    total_chars = 0
    total_dur = 0.0
    print(f"{'#':>3}  {'字数':>4}  {'时长':>6}  {'字/秒':>6}  判定  句子")
    print("-" * 96)
    for idx, (text, wav_path) in enumerate(zip(lines, wavs)):
        chars, cjk = count_chars(text)
        try:
            dur = wav_duration(wav_path)
        except (wave.Error, EOFError, ValueError) as exc:
            print(f"!! {wav_path} 不是可读 PCM WAV：{exc}", file=sys.stderr)
            continue
        rate = chars / dur if dur > 0 else 0.0
        if rate < lo:
            verdict, factor = "偏慢", rate / mid
        elif rate > hi:
            verdict, factor = "偏快", rate / mid
        else:
            verdict, factor = "OK", 1.0
        total_chars += chars
        total_dur += dur
        rows.append(
            {
                "index": idx,
                "wav": os.path.basename(wav_path),
                "text": text,
                "chars": chars,
                "cjk": cjk,
                "duration": round(dur, 3),
                "chars_per_sec": round(rate, 3),
                "verdict": verdict,
                "suggested_duration_factor": round(factor, 2),
            }
        )
        hint = f"  (≈duration_factor {factor:.2f})" if args.factor_hint and verdict != "OK" else ""
        print(f"{idx:>3}  {chars:>4}  {dur:>6.2f}  {rate:>6.2f}  {verdict}  {text[:40]}{hint}")

    if not rows:
        print("!! 没有可统计的音频", file=sys.stderr)
        return 2

    overall = total_chars / total_dur if total_dur > 0 else 0.0
    print("-" * 96)
    print(
        f"合计 {len(rows)} 句，{total_chars} 字，{total_dur:.2f}s，"
        f"平均 {overall:.2f} 字/秒（目标 {lo}–{hi}）"
    )
    # IndexTTS 的 duration_factor：>1 变慢。给一条整轨建议，P2 卡时长时先改文案再用这个。
    print(f"整轨建议 duration_factor ≈ {overall / mid:.2f}（每句建议见 --factor-hint）")

    if args.json_out:
        payload = {
            "lines_file": args.lines,
            "wav_dir": args.wav_dir,
            "target": [lo, hi],
            "rows": rows,
            "total_chars": total_chars,
            "total_duration": round(total_dur, 3),
            "overall_chars_per_sec": round(overall, 3),
            "suggested_duration_factor": round(overall / mid, 2),
        }
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2)
        print(f"明细已写入 {args.json_out}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
