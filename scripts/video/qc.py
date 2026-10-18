#!/usr/bin/env python3
"""成片 QC：参数、响度、黑场、分段时长、抽帧（P3 交付用）。

对成片跑一遍可自动化的检查，输出人类可读结论 + JSON，抽帧存 `work/qc/frames/`。
不能自动判定的（字幕可读性、旁白自然度、画面观感）另外列成人工清单。

用法::

    python scripts/video/qc.py --video course/04-视频/DataBackup-demo-1.0.1.mp4 \
        --json-out course/04-视频/work/qc/qc.json --frames-dir course/04-视频/work/qc/frames
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
FF = os.path.join(ROOT, "course", "04-视频", "tools", "ffmpeg", "bin", "ffmpeg.exe")
FP = os.path.join(ROOT, "course", "04-视频", "tools", "ffmpeg", "bin", "ffprobe.exe")

MAX_DURATION = 180.0
WIDTH, HEIGHT, FPS = 1920, 1080, 30.0
LUFS_TARGET, LUFS_TOL = -16.0, 1.0
PEAK_MAX = -1.5
SEG_TARGETS = {
    "seg-00-title": 9.0,
    "seg-01-data": 20.0,
    "seg-02-setup": 37.0,
    "seg-03-backup": 28.0,
    "seg-04-manage": 25.0,
    "seg-05-restore": 35.0,
    "seg-06-verify": 24.0,
}


def run(cmd: list[str]) -> str:
    proc = subprocess.run(cmd, capture_output=True, text=True, encoding="utf-8", errors="replace")
    if proc.returncode != 0 and proc.stderr:
        print(f"[ffmpeg stderr] {proc.stderr.strip()[:500]}", file=sys.stderr)
    return (proc.stdout or "") + (proc.stderr or "")


def probe(path: str) -> dict:
    out = run(
        [
            FP, "-v", "error", "-show_entries",
            "format=duration,size,bit_rate:stream=index,codec_type,codec_name,width,height,r_frame_rate,"
            "sample_rate,channels,nb_frames,duration",
            "-of", "json", path,
        ]
    )
    start = out.find("{")
    try:
        return json.loads(out[start:]) if start >= 0 else {}
    except json.JSONDecodeError:
        return {}


def framerate(value: str | None) -> float:
    if not value:
        return 0.0
    if "/" in value:
        num, den = value.split("/", 1)
        return float(num) / float(den or 1)
    return float(value)


def mp4_faststart(path: str) -> bool | None:
    """moov 在 mdat 之前 = faststart。只对 mp4 有意义。"""
    try:
        with open(path, "rb") as fh:
            offset = 0
            boxes = []
            while offset < os.path.getsize(path):
                fh.seek(offset)
                header = fh.read(8)
                if len(header) < 8:
                    break
                size = int.from_bytes(header[:4], "big")
                name = header[4:8].decode("latin-1")
                if size == 1:  # 64-bit size
                    size = int.from_bytes(fh.read(8), "big")
                elif size == 0:
                    size = os.path.getsize(path) - offset
                boxes.append((name, offset, size))
                if size < 8:
                    break
                offset += size
                if name == "mdat":
                    break
            moov_at = next((o for n, o, _ in boxes if n == "moov"), None)
            mdat_at = next((o for n, o, _ in boxes if n == "mdat"), None)
            if moov_at is None or mdat_at is None:
                return None
            return moov_at < mdat_at
    except OSError:
        return None


def loudness(path: str) -> dict:
    """从 ebur128 的 Summary 块取整体响度（不能直接 re.search，逐帧 momentary 行里也有 I:）。"""
    out = run([FF, "-hide_banner", "-nostats", "-i", path, "-af", "ebur128=peak=true", "-f", "null", "-"])
    idx = out.rfind("Summary:")
    seg = out[idx:] if idx >= 0 else out
    integ = re.search(r"Integrated loudness:[\s\S]*?I:\s*(-?\d+(?:\.\d+)?)\s*LUFS", seg)
    peak = re.search(r"True peak:[\s\S]*?Peak:\s*(-?\d+(?:\.\d+)?)\s*dBFS", seg)
    lra = re.search(r"Loudness range:[\s\S]*?LRA:\s*(-?\d+(?:\.\d+)?)\s*LU", seg)
    return {
        "integrated_lufs": float(integ.group(1)) if integ else None,
        "true_peak_dbfs": float(peak.group(1)) if peak else None,
        "lra": float(lra.group(1)) if lra else None,
    }


def black_intervals(path: str, luma_thresh: float = 20.0, fps: int = 2, min_span: float = 0.5) -> list[tuple[float, float]]:
    """检测「整帧接近全黑」（2 fps 采样 YMAX）。

    不用 ffmpeg 的 blackdetect：本片 UI 是纯黑画布（设计如此），blackdetect 会把正常界面判成黑场。
    这里只把 YMAX < luma_thresh 的帧算黑帧：正常界面/卡片总有白色文字，YMAX 很高。
    """
    out = run(
        [FF, "-hide_banner", "-nostats", "-i", path, "-vf",
         f"fps={fps},signalstats,metadata=mode=print", "-f", "null", "-"]
    )
    samples: list[tuple[float, float]] = []
    cur_t = None
    for line in out.splitlines():
        mt = re.search(r"pts_time:(\d+(?:\.\d+)?)", line)
        if mt:
            cur_t = float(mt.group(1))
        my = re.search(r"lavfi\.signalstats\.YMAX=(\d+(?:\.\d+)?)", line)
        if my and cur_t is not None:
            samples.append((cur_t, float(my.group(1))))
    spans: list[tuple[float, float]] = []
    start = None
    last_black = None
    for t, ymax in samples:
        if ymax < luma_thresh:
            if start is None:
                start = t
            last_black = t
        else:
            if start is not None and last_black is not None and last_black - start >= min_span:
                spans.append((start, last_black))
            start = None
            last_black = None
    if start is not None and last_black is not None and last_black - start >= min_span:
        spans.append((start, last_black))
    return spans


def extract_frames(path: str, times: list[float], outdir: str) -> list[str]:
    os.makedirs(outdir, exist_ok=True)
    files = []
    for t in times:
        dst = os.path.join(outdir, f"frame-{t:07.2f}.png")
        run([FF, "-y", "-v", "error", "-ss", f"{t:.2f}", "-i", path, "-frames:v", "1", dst])
        if os.path.isfile(dst):
            files.append(dst)
    return files


def main() -> int:
    ap = argparse.ArgumentParser(description="成片 QC")
    ap.add_argument("--video", required=True)
    ap.add_argument("--json-out")
    ap.add_argument("--frames-dir", default="course/04-视频/work/qc/frames")
    ap.add_argument("--extra-frames", default="10,20,45,70,95,120,150,170",
                    help="额外抽帧的时间点（逗号分隔秒）")
    args = ap.parse_args()

    if not os.path.isfile(args.video):
        print(f"!! 找不到成片：{args.video}", file=sys.stderr)
        return 2

    problems: list[str] = []
    warns: list[str] = []
    info = probe(args.video)
    fmt = info.get("format") or {}
    streams = info.get("streams") or []
    video = next((s for s in streams if s.get("codec_type") == "video"), {})
    audio = next((s for s in streams if s.get("codec_type") == "audio"), {})

    duration = float(fmt.get("duration") or 0)
    width, height = int(video.get("width") or 0), int(video.get("height") or 0)
    fps = framerate(video.get("r_frame_rate"))
    vcodec, acodec = video.get("codec_name"), audio.get("codec_name")
    ch, sr = int(audio.get("channels") or 0), int(audio.get("sample_rate") or 0)

    print("== 成片参数 ==")
    print(f"  时长 {duration:.3f}s / 大小 {int(fmt.get('size') or 0) / 1e6:.1f} MB / 码率 "
          f"{int(fmt.get('bit_rate') or 0) / 1e6:.2f} Mbps")
    print(f"  视频 {vcodec} {width}x{height} @ {fps:.3f} fps")
    print(f"  音频 {acodec} {sr} Hz / {ch} ch")

    if duration > MAX_DURATION:
        problems.append(f"时长 {duration:.2f}s 超过 {MAX_DURATION:.0f}s")
    if (width, height) != (WIDTH, HEIGHT):
        problems.append(f"分辨率 {width}x{height} ≠ {WIDTH}x{HEIGHT}")
    if abs(fps - FPS) > 0.05:
        problems.append(f"帧率 {fps:.3f} ≠ {FPS}")
    if vcodec != "h264":
        problems.append(f"视频编码 {vcodec} ≠ h264")
    if acodec != "aac":
        problems.append(f"音频编码 {acodec} ≠ aac")
    if sr != 48000 or ch != 2:
        warns.append(f"音频 {sr}Hz/{ch}ch（期望 48000/2）")
    faststart = mp4_faststart(args.video)
    if faststart is False:
        warns.append("moov 在 mdat 之后（未 faststart，网页播放会等下载）")

    print("\n== 响度 ==")
    loud = loudness(args.video)
    print(f"  I={loud['integrated_lufs']} LUFS / TruePeak={loud['true_peak_dbfs']} dBFS / LRA={loud['lra']} LU")
    if loud["integrated_lufs"] is None:
        warns.append("响度测量失败")
    elif abs(loud["integrated_lufs"] - LUFS_TARGET) > LUFS_TOL:
        problems.append(f"响度 {loud['integrated_lufs']} LUFS 偏离目标 {LUFS_TARGET}±{LUFS_TOL}")
    if loud["true_peak_dbfs"] is not None and loud["true_peak_dbfs"] > PEAK_MAX:
        problems.append(f"真峰值 {loud['true_peak_dbfs']} dBFS 高于 {PEAK_MAX}")

    print("\n== 黑场 ==")
    blacks = black_intervals(args.video)
    # 允许：片头起播前极短黑场 + S6 结尾淡出（≤2.5s）
    for start, end in blacks:
        span = end - start
        if end < 1.0 and start < 0.6:
            continue
        if start > duration - 3.0 and span <= 2.6:
            continue
        warns.append(f"黑场 {start:.2f}–{end:.2f}s（{span:.2f}s）")
    print(f"  检测到黑场 {len(blacks)} 段" + (f"：{blacks}" if blacks else "（无）"))

    print("\n== 分段时长（目标 vs 实得）==")
    seg_dir = os.path.join(ROOT, "course", "04-视频", "work", "compose")
    seg_rows = []
    total = 0.0
    for name, target in SEG_TARGETS.items():
        path = os.path.join(seg_dir, f"{name}.mp4")
        if not os.path.isfile(path):
            warns.append(f"缺分段 {name}.mp4")
            continue
        seg = probe(path)
        seg_dur = float((seg.get("format") or {}).get("duration") or 0)
        total += seg_dur
        delta = seg_dur - target
        verdict = "OK" if abs(delta) <= 1.0 else "WARN"
        if verdict == "WARN":
            warns.append(f"{name} 时长 {seg_dur:.2f}s 与目标 {target}s 差 {delta:+.2f}s")
        seg_rows.append({"name": name, "target": target, "actual": round(seg_dur, 3),
                         "delta": round(delta, 3), "verdict": verdict})
        print(f"  {name:<16} 目标 {target:>5.1f}s  实得 {seg_dur:>7.3f}s  Δ{delta:+.3f}s  {verdict}")
    print(f"  合计 {total:.3f}s（成片 {duration:.3f}s）")

    print("\n== 抽帧 ==")
    try:
        times = sorted({float(x) for x in args.extra_frames.split(",") if x.strip()})
    except ValueError:
        times = [10, 20, 45, 70, 95, 120, 150, 170]
    # 每个镜头的 ~20% 与 60% 位置各抽一帧
    board_path = os.path.join(ROOT, "docs", "deliverables", "video-storyboard.json")
    if os.path.isfile(board_path):
        with open(board_path, encoding="utf-8") as fh:
            board = json.load(fh)
        for shot in board.get("shots") or []:
            start = float(shot.get("start", 0))
            dur = float(shot.get("duration", 0))
            times.extend([round(start + dur * 0.25, 2), round(start + dur * 0.75, 2)])
    times = sorted({t for t in times if 0 < t < duration})
    frames = extract_frames(args.video, times, os.path.join(ROOT, args.frames_dir))
    print(f"  抽取 {len(frames)} 帧 -> {args.frames_dir}")

    print("\n== 自动检查结论 ==")
    for p in problems:
        print(f"  FAIL {p}")
    for w in warns:
        print(f"  WARN {w}")
    if not problems:
        print("  参数/响度/编码/时长：通过（WARN 见上，需人工确认）")

    payload = {
        "video": os.path.abspath(args.video),
        "duration": round(duration, 3),
        "size_bytes": int(fmt.get("size") or 0),
        "video_stream": {"codec": vcodec, "width": width, "height": height, "fps": round(fps, 3)},
        "audio_stream": {"codec": acodec, "sample_rate": sr, "channels": ch},
        "faststart": faststart,
        "loudness": loud,
        "black_intervals": blacks,
        "segments": seg_rows,
        "frames": [os.path.relpath(f, ROOT) for f in frames],
        "problems": problems,
        "warns": warns,
    }
    if args.json_out:
        os.makedirs(os.path.dirname(os.path.abspath(args.json_out)) or ".", exist_ok=True)
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2)
        print(f"\n明细：{args.json_out}")
    return 0 if not problems else 1


if __name__ == "__main__":
    raise SystemExit(main())
