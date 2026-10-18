#!/usr/bin/env python3
"""由分镜 JSON + 注解定位 JSON 生成 ASS 字幕/注解文件（P3 合成用）。

设计
====
- **唯一时间来源**：`docs/deliverables/video-storyboard.json`（P2 已固化）；镜内时间（字幕/注解的
  start/end）是相对镜头起点的秒数，本脚本加上 `shots[].start` 得到成片绝对时间。
- **注解定位**：`annotation-boxes.json`（录制 teammates 产出）里每个 target 给一个框；支持两种写法：
  - `{"video": {"x","y","w","h"}}`：已是 1920×1080 成片坐标，直接用；
  - `{"css": {"x","y","w","h"}}`：客户端段窗口 CSS 坐标（1120×720，dsf=2），按
    `1080p_x = 120 + 1.5*css_x`、`1080p_y = 1.5*css_y` 换算。
  缺 target 时跳过该注解并在 stderr 警告（不静默错位）。
- **样式**（07-…§6）：字幕底部居中、36px、白字 + 60% 黑描边；注解为深绿 #14584B 标签
  （BorderStyle=3 圆角色块的近似）+ 同色 3px 目标框；安全区左右 80px、底部 120px。
- 同时输出 `timeline.json`：每镜绝对起止 + 旁白 wav + 字幕/注解条数，供混音与拼接脚本使用。

用法::

    python scripts/video/gen_ass.py \
        --storyboard docs/deliverables/video-storyboard.json \
        --boxes "course/04-视频/work/seg/annotation-boxes.json" \
        --out "course/04-视频/work/ass/demo.ass" \
        --timeline-out "course/04-视频/work/ass/timeline.json"

不加 `--boxes` 也能跑（只有字幕，注解全部跳过并警告）。
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass

# 07-…§6 视觉规范
SUB_FONTSIZE = 36
SUB_MARGIN_V = 86
SUB_OUTLINE = 2.6
SUB_SHADOW = 0.8
ANN_FONTSIZE = 26
ANN_PADDING = 10  # BorderStyle=3 时 Outline 即文字底衬的 padding
BOX_OUTLINE = 3
SAFE_X = 80
SAFE_TOP = 80
SAFE_BOTTOM = 120
GREEN_BGR = "&H004B5814"  # #14584B → ASS 是 BGR
WHITE = "&H00FFFFFF"
BLACK_ALPHA_60 = "&H60000000"

SUB_TARGET_LEN = 18  # 每行 ≤18 字
SUB_DUR = (2.4, 4.5)
ANN_DUR = (2.5, 3.5)


def ass_time(seconds: float) -> str:
    if seconds < 0:
        seconds = 0.0
    cs = int(round(seconds * 100))
    h, rem = divmod(cs, 360000)
    m, rem = divmod(rem, 6000)
    s, cs = divmod(rem, 100)
    return f"{h:d}:{m:02d}:{s:02d}.{cs:02d}"


def escape_text(text: str) -> str:
    r"""ASS 行内转义：花括号会被当作 override block，反斜杠 N 是硬换行。"""
    text = text.replace("\\", "＼")
    text = text.replace("{", "\\{").replace("}", "\\}")
    return text.replace("\n", "\\N")


def cjk_len(text: str) -> int:
    """粗略显示宽度：CJK/全角算 1，ASCII 算 0.55。"""
    width = 0.0
    for ch in text:
        width += 1.0 if ord(ch) > 0x2E7F else 0.55
    return int(round(width))


def load_boxes(path: str | None) -> dict:
    if not path:
        return {}
    try:
        with open(path, encoding="utf-8") as fh:
            raw = json.load(fh)
    except OSError as exc:
        print(f"!! 读不到注解定位文件：{exc}", file=sys.stderr)
        return {}
    boxes: dict = {}

    def add(target: str, entry: dict) -> None:
        if "video" in entry:
            box = entry["video"]
        elif "css" in entry:
            css = entry["css"]
            box = {
                "x": 120 + 1.5 * float(css["x"]),
                "y": 1.5 * float(css["y"]),
                "w": 1.5 * float(css["w"]),
                "h": 1.5 * float(css["h"]),
            }
        elif all(k in entry for k in ("x", "y", "w", "h")):
            # 直接给成片 1080p 像素坐标（卡片段卡片布局就是这种）
            box = entry
        else:
            return
        boxes[target] = {
            "x": float(box["x"]),
            "y": float(box["y"]),
            "w": float(box["w"]),
            "h": float(box["h"]),
        }

    # 支持扁平 {target: {...}}、按段分组 {"seg-02-setup": {target: {...}}, "targets": {...}}、
    # 以及卡片布局 {"seg-01-data": {"targets": {...}}}。
    for key, value in raw.items():
        if not isinstance(value, dict):
            continue
        if "video" in value or "css" in value or all(k in value for k in ("x", "y", "w", "h")):
            add(key, value)
        elif isinstance(value.get("targets"), dict):
            for target, entry in value["targets"].items():
                if isinstance(entry, dict):
                    add(target, entry)
        else:
            for target, entry in value.items():
                if isinstance(entry, dict):
                    add(target, entry)
    return boxes


def clamp_label_pos(box: dict, label_text: str, canvas_w: int, canvas_h: int) -> tuple[int, int]:
    """标签位置：优先放目标框右侧（不遮内容），其次框内右侧空白，最后框上方。

    返回 (x, y) 对应 \\an2（底部居中）的落点。全宽行（终端行）用「框内右侧」，
    这样标签压在行尾空白处而不是上一行文字上。
    """
    gap = 14
    chip_w = cjk_len(label_text) * ANN_FONTSIZE + 2 * ANN_PADDING + 6
    chip_h = ANN_FONTSIZE + 2 * ANN_PADDING + 4
    right_safe = canvas_w - SAFE_X

    bx, by, bw, bh = box["x"], box["y"], box["w"], box["h"]
    # 1) 框右侧外侧的空间放得下 → 框外右侧（小控件如开关、胶囊行）
    if bx + bw + gap + chip_w <= right_safe:
        return int(round(bx + bw + gap + chip_w / 2)), int(round(by + bh / 2 + chip_h / 2))
    # 1.5) 多行大块（如终端输出 4 行）→ 放块下方空白处，避免压住右侧正文
    if bh >= 90 and by + bh + gap + chip_h <= canvas_h - SAFE_BOTTOM:
        return int(round(bx + bw / 2)), int(round(by + bh + gap + chip_h / 2))
    # 2) 框内右侧放得下 → 压在行尾空白（全宽终端行）
    if chip_w + 24 <= bw:
        return int(round(bx + bw - chip_w / 2 - 12)), int(round(by + bh / 2 + chip_h / 2))
    # 3) 上方（下方会压下一行）
    x = bx + bw / 2
    y = by - gap
    if y - chip_h < SAFE_TOP:
        y = by + bh + gap + chip_h
    x = min(max(x, SAFE_X + chip_w / 2), right_safe - chip_w / 2)
    y = min(max(y, SAFE_TOP + chip_h), canvas_h - SAFE_BOTTOM)
    return int(round(x)), int(round(y))


def build_ass(board: dict, boxes: dict, canvas_w: int, canvas_h: int, font: str) -> tuple[str, list[str], dict]:
    shots = board.get("shots") or []
    problems: list[str] = []

    header = f"""[Script Info]
ScriptType: v4.00+
PlayResX: {canvas_w}
PlayResY: {canvas_h}
ScaledBorderAndShadow: yes
WrapStyle: 2
YCbCr Matrix: TV.709

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Subtitle,{font},{SUB_FONTSIZE},{WHITE},&H000000FF,{BLACK_ALPHA_60},&H80000000,0,0,0,0,100,100,0.5,0,1,{SUB_OUTLINE},{SUB_SHADOW},2,{SAFE_X},{SAFE_X},{SUB_MARGIN_V},1
Style: Annotation,{font},{ANN_FONTSIZE},{WHITE},&H000000FF,{GREEN_BGR},&H00000000,-1,0,0,0,100,100,0,0,3,{ANN_PADDING},0,2,40,40,40,1
Style: AnnBox,{font},1,{WHITE},&H000000FF,{GREEN_BGR},&H00000000,0,0,0,0,100,100,0,0,1,{BOX_OUTLINE},0,7,0,0,0,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
"""

    events: list[str] = []
    missing_targets: set[str] = set()
    timeline = []

    for idx, shot in enumerate(shots):
        shot_id = shot.get("id", f"S{idx}")
        start = float(shot.get("start", 0.0))
        duration = float(shot.get("duration", 0.0))
        shot_end = start + duration

        for sub in shot.get("subtitles") or []:
            t0 = start + float(sub.get("start", 0.0))
            t1 = min(start + float(sub.get("end", 0.0)), shot_end)
            text = str(sub.get("text", "")).strip()
            if not text or t1 <= t0:
                continue
            dur = t1 - t0
            if not (SUB_DUR[0] - 0.2 <= dur <= SUB_DUR[1] + 1.0):
                problems.append(f"{shot_id} 字幕「{text}」时长 {dur:.1f}s 不在 {SUB_DUR} 区间")
            if cjk_len(text) > SUB_TARGET_LEN:
                problems.append(f"{shot_id} 字幕「{text}」显示宽度 ≈{cjk_len(text)} 字 > {SUB_TARGET_LEN}")
            lay = 0
            events.append(
                f"Dialogue: {lay},{ass_time(t0)},{ass_time(t1)},Subtitle,,0,0,0,,{escape_text(text)}"
            )

        shot_anns = shot.get("annotations") or []
        for ann_idx, ann in enumerate(shot_anns):
            t0 = start + float(ann.get("start", 0.0))
            t1 = min(start + float(ann.get("end", 0.0)), shot_end)
            text = str(ann.get("text", "")).strip()
            target = str(ann.get("target", "")).strip()
            if t1 <= t0:
                continue
            dur = t1 - t0
            if not (ANN_DUR[0] - 0.3 <= dur <= ANN_DUR[1] + 0.5):
                problems.append(f"{shot_id} 注解「{text}」时长 {dur:.1f}s 不在 {ANN_DUR} 区间")
            box = boxes.get(target)
            if box is None:
                missing_targets.add(target)
                continue

            bx, by = float(box["x"]), float(box["y"])
            bw, bh = float(box["w"]), float(box["h"])
            # 目标框（AnnBox + 透明填充 + 绿色描边）
            events.append(
                f"Dialogue: 1,{ass_time(t0)},{ass_time(t1)},AnnBox,,0,0,0,,"
                f"{{\\an7\\pos({int(round(bx))},{int(round(by))})\\p1\\1a&HFF\\bord{BOX_OUTLINE}"
                f"\\3c{GREEN_BGR}}}m 0 0 l {int(round(bw))} 0 {int(round(bw))} {int(round(bh))} "
                f"0 {int(round(bh))}{{\\p0}}"
            )
            # 标签（绿底白字）：框外右侧 → 框内右侧空白 → 框上方
            lx, ly = clamp_label_pos({"x": bx, "y": by, "w": bw, "h": bh}, text, canvas_w, canvas_h)
            events.append(
                f"Dialogue: 2,{ass_time(t0)},{ass_time(t1)},Annotation,,0,0,0,,"
                f"{{\\an2\\pos({lx},{ly})}}{escape_text(text)}"
            )

        # 注解重叠检查（同镜内 >2 个同时出现）
        points = sorted([(float(a.get("start", 0)), float(a.get("end", 0))) for a in shot_anns])
        for i in range(len(points)):
            overlapping = sum(1 for s, e in points if s <= points[i][0] < e)
            if overlapping > 2:
                problems.append(f"{shot_id} 有 {overlapping} 个注解在 t={points[i][0]}s 同时出现（上限 2）")
                break

        timeline.append(
            {
                "id": shot_id,
                "title": shot.get("title"),
                "kind": shot.get("kind"),
                "start": round(start, 3),
                "duration": round(duration, 3),
                "end": round(shot_end, 3),
                "narration_wav": f"course/04-视频/voice/narration/seg_{idx:02d}.wav",
                "subtitles": len(shot.get("subtitles") or []),
                "annotations": len(shot_anns),
            }
        )

    if missing_targets:
        problems.append(f"以下注解 target 缺少定位框，已跳过：{sorted(missing_targets)}")

    body = header + "\n".join(events) + "\n"
    meta = {
        "shots": timeline,
        "total_duration": round(sum(t["duration"] for t in timeline), 3),
        "subtitles": sum(t["subtitles"] for t in timeline),
        "annotations": sum(t["annotations"] for t in timeline),
        "missing_targets": sorted(missing_targets),
        "problems": problems,
    }
    return body, problems, meta


def main() -> int:
    ap = argparse.ArgumentParser(description="生成 ASS 字幕/注解 + timeline.json")
    ap.add_argument("--storyboard", required=True)
    ap.add_argument(
        "--boxes",
        action="append",
        help="注解定位 JSON，可重复传多个（后者覆盖同名 target）",
    )
    ap.add_argument("--out", required=True, help="输出 .ass 路径")
    ap.add_argument("--timeline-out", help="输出 timeline.json 路径")
    ap.add_argument("--font", default="Noto Sans CJK SC", help="ASS 字体名（需在 fontsdir 中可用）")
    ap.add_argument("--canvas", default="1920x1080", help="画布尺寸 WxH")
    ap.add_argument(
        "--only-shot",
        help="只输出某一镜（如 S2），并把时间基准归零——用于对单段 webm 做烧录预览",
    )
    args = ap.parse_args()

    try:
        width, height = (int(x) for x in args.canvas.lower().split("x"))
    except ValueError:
        print(f"!! --canvas 需形如 1920x1080，收到 {args.canvas!r}", file=sys.stderr)
        return 2

    try:
        with open(args.storyboard, encoding="utf-8") as fh:
            board = json.load(fh)
    except OSError as exc:
        print(f"!! 读不到分镜 JSON：{exc}", file=sys.stderr)
        return 2

    if args.only_shot:
        shots = board.get("shots") or []
        picked = [s for s in shots if s.get("id") == args.only_shot]
        if not picked:
            print(f"!! 分镜里没有 {args.only_shot}", file=sys.stderr)
            return 2
        shot = dict(picked[0])
        shot["start"] = 0  # 时间基准归零，直接对单段 webm 烧录
        board = {"shots": [shot]}
        print(f"[only-shot] {args.only_shot}：时间基准归零，时长 {shot.get('duration')}s")

    boxes: dict = {}
    for path in args.boxes or []:
        part = load_boxes(path)
        if not part:
            print(f"[boxes] 注意：{path} 里没有可用坐标", file=sys.stderr)
        boxes.update(part)
    print(f"[boxes] 共 {len(boxes)} 个 target 定位：{', '.join(sorted(boxes))}")
    ass_text, problems, meta = build_ass(board, boxes, width, height, args.font)

    os.makedirs(os.path.dirname(os.path.abspath(args.out)) or ".", exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as fh:
        fh.write(ass_text)
    print(f"[ass] {args.out}：{meta['subtitles']} 条字幕、{meta['annotations']} 条注解、"
          f"{len(meta['missing_targets'])} 个 target 缺定位")

    if args.timeline_out:
        os.makedirs(os.path.dirname(os.path.abspath(args.timeline_out)) or ".", exist_ok=True)
        with open(args.timeline_out, "w", encoding="utf-8") as fh:
            json.dump(meta, fh, ensure_ascii=False, indent=2)
        print(f"[timeline] {args.timeline_out}（总时长 {meta['total_duration']}s）")

    if problems:
        print(f"\n[检查] {len(problems)} 条提示：", file=sys.stderr)
        for p in problems:
            print("  - " + p, file=sys.stderr)
    else:
        print("[检查] 字幕/注解时长、长度、重叠、定位均通过")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
