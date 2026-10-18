#!/usr/bin/env python3
"""把分镜 JSON 的字幕/注解时长规整到 07-…§6 的规范区间（P2 固化的最后一步）。

规范（07-演示视频分镜与操作脚本 §6）：
- 字幕：每条 2.4–4.5 s，单行 ≤18 字（长度不在此处改，由 gen_ass 检查）；
- 注解：每条 2.5–3.5 s，最多 2 个同时出现。

本脚本只改 `subtitles[]/annotations[]` 的 `start/end`（相对镜头起点），不改文本、不改镜头时长：
- 时长超上限 → 保留起点、截短结尾（字幕"锁存"更久没有信息增量，还会遮画面）；
- 时长低于下限 → 保留起点、延长结尾，但不越过镜头结束 / 下一条开始前 0.2 s；
- 同一镜内按开始时间排序后保证互不重叠（重叠则把后一条起点推到前一条结束 +0.2 s）。

用法::

    python scripts/video/normalize_storyboard.py                 # 只打印将要做的修改（dry-run）
    python scripts/video/normalize_storyboard.py --write         # 就地写回
    python scripts/video/normalize_storyboard.py --write --storyboard <path> --json-out <report>
"""
from __future__ import annotations

import argparse
import json
import os
import sys

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

SUB_MIN, SUB_MAX = 2.4, 4.5
ANN_MIN, ANN_MAX = 2.5, 3.5
MIN_GAP = 0.2


def clamp_items(items: list[dict], shot_end: float, lo: float, hi: float, label: str, shot_id: str,
                changes: list[str]) -> None:
    items_sorted = sorted(items, key=lambda x: float(x.get("start", 0.0)))
    prev_end = -1.0
    for item in items_sorted:
        start = float(item.get("start", 0.0))
        end = float(item.get("end", 0.0))
        if start < prev_end + MIN_GAP:
            new_start = round(prev_end + MIN_GAP, 2)
            changes.append(f"{shot_id} {label}「{item.get('text')}」起点 {start:g}→{new_start:g}（与上一条重叠）")
            start = new_start
        end = min(end, shot_end)
        dur = end - start
        if dur > hi:
            new_end = round(start + hi, 2)
            changes.append(f"{shot_id} {label}「{item.get('text')}」时长 {dur:.2f}s→{hi:.1f}s（截短结尾）")
            end = new_end
        elif dur < lo:
            new_end = min(round(start + lo, 2), shot_end)
            if new_end - start >= lo:
                changes.append(f"{shot_id} {label}「{item.get('text')}」时长 {dur:.2f}s→{lo:.1f}s（延长结尾）")
            else:
                new_start = max(0.0, round(shot_end - lo, 2))
                changes.append(
                    f"{shot_id} {label}「{item.get('text')}」镜头末尾放不下 {lo:.1f}s，起点 {start:g}→{new_start:g}"
                )
                start = new_start
                new_end = shot_end
            end = new_end
        item["start"] = round(start, 2)
        item["end"] = round(end, 2)
        prev_end = end


def main() -> int:
    ap = argparse.ArgumentParser(description="规整分镜字幕/注解时长到 §6 规范")
    ap.add_argument("--storyboard", default="docs/deliverables/video-storyboard.json")
    ap.add_argument("--write", action="store_true", help="就地写回（默认 dry-run）")
    ap.add_argument("--json-out", help="把修改清单写成 JSON")
    args = ap.parse_args()

    try:
        with open(args.storyboard, encoding="utf-8") as fh:
            board = json.load(fh)
    except OSError as exc:
        print(f"!! 读不到分镜 JSON：{exc}", file=sys.stderr)
        return 2

    changes: list[str] = []
    for idx, shot in enumerate(board.get("shots") or []):
        shot_id = shot.get("id", f"S{idx}")
        shot_end = float(shot.get("duration", 0.0))
        clamp_items(shot.get("subtitles") or [], shot_end, SUB_MIN, SUB_MAX, "字幕", shot_id, changes)
        clamp_items(shot.get("annotations") or [], shot_end, ANN_MIN, ANN_MAX, "注解", shot_id, changes)

    for change in changes:
        print("· " + change)
    if not changes:
        print("无需修改：所有字幕/注解时长已在规范区间内。")
    else:
        print(f"\n共 {len(changes)} 处修改 {'（已写回）' if args.write else '（dry-run，未写回）'}")

    if args.json_out:
        os.makedirs(os.path.dirname(os.path.abspath(args.json_out)) or ".", exist_ok=True)
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump({"changes": changes, "written": args.write}, fh, ensure_ascii=False, indent=2)
        print(f"修改清单：{args.json_out}")

    if args.write and changes:
        with open(args.storyboard, "w", encoding="utf-8") as fh:
            json.dump(board, fh, ensure_ascii=False, indent=2)
            fh.write("\n")
        print(f"已写回 {args.storyboard}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
