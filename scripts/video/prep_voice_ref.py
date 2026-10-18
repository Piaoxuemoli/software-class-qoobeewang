#!/usr/bin/env python3
"""从开源语料的长音频里切出 IndexTTS-2.5 零样本参考音频（8–12s），并做客观质检。

宿主没有 ffmpeg 也能跑：只依赖 Python 标准库（wave / array / math / json）。
参考音频要求见 docs/research/2026-10-18-AI配音选型.md §8。

用法::

    # 固定窗口（人耳/看波形定好起点时用）
    python scripts/video/prep_voice_ref.py \\
        --input "course/04-视频/voice/raw/aishell-S0002-full.wav" \\
        --output course/04-视频/voice/ref/main-ref.wav --start 12.4 --duration 10

    # 在长音频里自动挑“最像连续说话”的窗口
    python scripts/video/prep_voice_ref.py \\
        --input raw.wav --output ref.wav --auto --duration 10

    # 只量指标、不写文件
    python scripts/video/prep_voice_ref.py --input raw.wav --analyze

输出指标：时长、采样率、位深、声道、峰值 dBFS、RMS dBFS、估算 SNR、F0 中位数、削波比例；
`--json-out` 写成 JSON，方便 SOURCE.md / QC 报告引用。

`--analyze` 与写文件共用同一套指标；写文件时按 `--target-peak-db`（默认 -4）做峰值归一，
不做降噪/EQ/压缩（见调研文档：过度处理会让克隆“不像本人”）。
"""
from __future__ import annotations

import argparse
import array
import json
import math
import os
import sys
import wave

# Windows 控制台默认 cp936，中文输出会花屏；WSL 里无害。
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

TARGET_PEAK_DB = -4.0
FLOOR_DB = -120.0


def db(amplitude: float) -> float:
    return 20.0 * math.log10(amplitude) if amplitude > 0 else FLOOR_DB


def read_pcm_mono(path: str) -> tuple[list[float], int, int, int]:
    """读 PCM WAV 为 [-1,1] 的浮点单声道样本。返回 (samples, sr, bit_depth, channels)。"""
    try:
        with wave.open(path, "rb") as wf:
            nch = wf.getnchannels()
            sw = wf.getsampwidth()
            sr = wf.getframerate()
            raw = wf.readframes(wf.getnframes())
    except wave.Error as exc:
        raise SystemExit(
            f"读不了 {path}：{exc}\n"
            "> 若是浮点 WAV（WAVE_FORMAT_IEEE_FLOAT）或压缩格式，请先用 ffmpeg 转 16-bit PCM。"
        ) from exc

    if sw == 1:
        vals = [(b - 128) / 128.0 for b in raw]
    elif sw == 2:
        arr = array.array("h")
        arr.frombytes(raw)
        if sys.byteorder == "big":
            arr.byteswap()
        vals = [v / 32768.0 for v in arr]
    elif sw == 3:
        vals = [
            int.from_bytes(raw[i : i + 3], "little", signed=True) / 8388608.0
            for i in range(0, len(raw), 3)
        ]
    elif sw == 4:
        arr = array.array("i")
        arr.frombytes(raw)
        if sys.byteorder == "big":
            arr.byteswap()
        vals = [v / 2147483648.0 for v in arr]
    else:
        raise SystemExit(f"不支持的位深：{sw * 8} bit（{path}）")

    if nch > 1:
        mono = [
            sum(vals[i : i + nch]) / nch for i in range(0, len(vals) - nch + 1, nch)
        ]
        vals = mono
    return vals, sr, sw * 8, nch


def frame_db(vals: list[float], win: int) -> list[float]:
    frames: list[float] = []
    for i in range(0, len(vals) - win + 1, win):
        seg = vals[i : i + win]
        energy = sum(v * v for v in seg) / win
        frames.append(db(math.sqrt(energy)))
    return frames


def percentile(sorted_vals: list[float], p: float) -> float:
    if not sorted_vals:
        return FLOOR_DB
    k = min(len(sorted_vals) - 1, max(0, int(round((len(sorted_vals) - 1) * p))))
    return sorted_vals[k]


def median(vals: list[float]) -> float:
    if not vals:
        return FLOOR_DB
    s = sorted(vals)
    return s[len(s) // 2]


def best_window(frames: list[float], win_frames: int, step_frames: int) -> int:
    """挑“连续说话、动态范围小”的窗口，返回起始帧号。"""
    if len(frames) <= win_frames:
        return 0
    best_score, best_start = -1e9, 0
    for start in range(0, len(frames) - win_frames + 1, step_frames):
        seg = sorted(frames[start : start + win_frames])
        p10, p90 = percentile(seg, 0.10), percentile(seg, 0.90)
        # 20 分位高 = 整段都有声；动态范围太大（长停顿/忽大忽小）扣分
        score = p10 - 0.6 * max(0.0, (p90 - p10) - 16.0)
        if score > best_score:
            best_score, best_start = score, start
    return best_start


def estimate_snr(frames: list[float]) -> float | None:
    s = sorted(frames)
    noise = percentile(s, 0.05)
    speech_frames = [v for v in s if v >= noise + 10.0]
    if len(speech_frames) < 5:
        return None
    return median(speech_frames) - noise


def median_f0(vals: list[float], sr: int) -> float | None:
    """自相关估基频中位数（仅用于判断男声/女声量级，不做严谨声学分析）。"""
    step = max(1, int(sr * 0.05))
    win = max(2, int(sr * 0.04))
    lag_lo, lag_hi = max(2, int(sr / 350)), int(sr / 70)
    estimates: list[float] = []
    for i in range(0, max(1, len(vals) - win), step):
        seg = vals[i : i + win]
        energy = sum(v * v for v in seg)
        if energy < 1e-4:  # 太安静，跳过
            continue
        best_lag, best_r = 0, -1e18
        for lag in range(lag_lo, min(lag_hi, win - 1) + 1):
            acc = 0.0
            for j in range(0, win - lag, 2):
                acc += seg[j] * seg[j + lag]
            if acc > best_r:
                best_r, best_lag = acc, lag
        if best_lag > 0:
            f0 = sr / best_lag
            if 70.0 <= f0 <= 350.0:
                estimates.append(f0)
    if not estimates:
        return None
    estimates.sort()
    return estimates[len(estimates) // 2]


def metrics(vals: list[float], sr: int) -> dict:
    peak = max((abs(v) for v in vals), default=0.0)
    rms = math.sqrt(sum(v * v for v in vals) / len(vals)) if vals else 0.0
    win = max(1, int(sr * 0.02))
    frames = frame_db(vals, win)
    clipped = sum(1 for v in vals if abs(v) >= 0.9999)
    snr = estimate_snr(frames)
    f0 = median_f0(vals, sr)
    return {
        "duration": round(len(vals) / sr, 3),
        "sample_rate": sr,
        "peak_dbfs": round(db(peak), 2),
        "rms_dbfs": round(db(rms), 2),
        "snr_db": None if snr is None else round(snr, 1),
        "f0_median_hz": None if f0 is None else round(f0, 1),
        "clipped_samples": clipped,
    }


def write_wav_16bit(path: str, vals: list[float], sr: int, target_peak_db: float) -> float:
    peak = max((abs(v) for v in vals), default=0.0)
    gain = 1.0 if peak <= 0 else (10 ** (target_peak_db / 20.0)) / peak
    data = array.array(
        "h", (max(-32768, min(32767, int(round(v * gain * 32767)))) for v in vals)
    )
    if sys.byteorder == "big":
        data.byteswap()
    os.makedirs(os.path.dirname(os.path.abspath(path)) or ".", exist_ok=True)
    with wave.open(path, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sr)
        wf.writeframes(data.tobytes())
    return gain


def fmt(v, unit="") -> str:
    return f"{v}{unit}" if v is not None else "n/a"


def main() -> int:
    ap = argparse.ArgumentParser(description="切出 IndexTTS 参考音频并做客观质检")
    ap.add_argument("--input", required=True, help="源 WAV（PCM）")
    ap.add_argument("--output", help="目标参考 WAV（16-bit PCM 单声道）")
    ap.add_argument("--start", type=float, default=None, help="截取起点（秒）")
    ap.add_argument("--duration", type=float, default=10.0, help="截取时长（秒，默认 10）")
    ap.add_argument("--auto", action="store_true", help="自动选择最像连续说话的窗口")
    ap.add_argument("--analyze", action="store_true", help="只输出指标，不写文件")
    ap.add_argument("--target-peak-db", type=float, default=TARGET_PEAK_DB)
    ap.add_argument("--json-out", help="把指标写成 JSON")
    args = ap.parse_args()

    if not os.path.isfile(args.input):
        print(f"!! 找不到输入文件：{args.input}", file=sys.stderr)
        return 2
    if not args.analyze and not args.output:
        print("!! 需要 --output（或 --analyze）", file=sys.stderr)
        return 2
    if args.auto and args.start is not None:
        print("!! --auto 与 --start 互斥，已按 --start 处理", file=sys.stderr)

    vals, sr, bit_depth, channels = read_pcm_mono(args.input)
    total_dur = len(vals) / sr
    print(
        f"[in ] {os.path.basename(args.input)}: {sr} Hz / {bit_depth} bit / {channels} ch / {total_dur:.2f} s"
    )

    src = metrics(vals, sr)
    print(f"[src] 峰值 {src['peak_dbfs']} dBFS, RMS {src['rms_dbfs']} dBFS, "
          f"SNR≈{fmt(src['snr_db'], ' dB')}, F0≈{fmt(src['f0_median_hz'], ' Hz')}, "
          f"削波 {src['clipped_samples']} 样本")

    win = max(1, int(sr * 0.02))
    if args.start is not None:
        start_s = max(0.0, args.start)
    elif args.auto:
        frames = frame_db(vals, win)
        start_frame = best_window(
            frames, int(args.duration * sr / win), max(1, int(0.25 * sr / win))
        )
        start_s = start_frame * win / sr
        print(f"[auto] 选中起点 {start_s:.2f} s（窗口 {args.duration:.2f} s）")
    else:
        start_s = 0.0

    end_s = start_s + args.duration
    if end_s > total_dur:
        print(
            f"!! 窗口 {start_s:.2f}–{end_s:.2f}s 超出音频长度 {total_dur:.2f}s，已截到末尾",
            file=sys.stderr,
        )
        end_s = total_dur
    seg = vals[int(start_s * sr) : int(end_s * sr)]
    if len(seg) < int(0.5 * sr):
        print("!! 截出来的片段太短（<0.5s），参数有误", file=sys.stderr)
        return 2

    seg_metrics = metrics(seg, sr)
    print(
        f"[seg] {seg_metrics['duration']} s / 峰值 {seg_metrics['peak_dbfs']} dBFS / "
        f"RMS {seg_metrics['rms_dbfs']} dBFS / SNR≈{fmt(seg_metrics['snr_db'], ' dB')} / "
        f"F0≈{fmt(seg_metrics['f0_median_hz'], ' Hz')}"
    )
    if seg_metrics["snr_db"] is not None and seg_metrics["snr_db"] < 25:
        print("     [warn] SNR < 25 dB：换一段更干净的，参考音频底噪会被克隆进去", file=sys.stderr)
    if seg_metrics["clipped_samples"] > 0:
        print("     [warn] 命中削波样本：源录音峰值过高", file=sys.stderr)
    f0 = seg_metrics["f0_median_hz"]
    if f0 is not None:
        guess = "男声量级" if f0 < 165 else "女声量级"
        print(f"     [info] F0 中位数 {f0} Hz，粗略判断为{guess}（仅参考，最终以试听为准）")

    if not args.analyze and args.output:
        gain = write_wav_16bit(args.output, seg, sr, args.target_peak_db)
        print(
            f"[out] 写出 {args.output}：{len(seg) / sr:.2f} s / {sr} Hz / 16 bit / mono，"
            f"峰值归一到 {args.target_peak_db} dBFS（增益 ×{gain:.3f}）"
        )
        print("      [note] 未做降噪/EQ/压缩/变速：这些处理会让克隆音色失真")

    if args.json_out:
        payload = {
            "input": os.path.abspath(args.input),
            "output": os.path.abspath(args.output) if args.output else None,
            "window": {
                "start_s": round(start_s, 3),
                "end_s": round(end_s, 3),
                "duration_s": round(len(seg) / sr, 3),
            },
            "source": src,
            "segment": seg_metrics,
            "target_peak_dbfs": args.target_peak_db,
        }
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2)
        print(f"[json] {args.json_out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
