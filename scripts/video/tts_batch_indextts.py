#!/usr/bin/env python3
"""IndexTTS-2.5 逐句旁白合成（P1 试听 3 句 / P2 完整旁白共用）。

必须在**已装好 IndexTTS 的环境**里运行（本机 WSL2 或租用 Linux + NVIDIA）。

设计要点
--------
- 不写死 API 参数名：先用 `inspect.signature` 读 `IndexTTS2.__init__` 与 `IndexTTS2.infer`
  的真实签名，只传签名里存在的参数。这样 2.5 与后续小版本的参数差异不会直接报错，
  且 `--print-api` 可以把真实签名打印出来，供文档记录。
- 逐句合成（一句一个 wav），句间静音留给后期拼轨：字幕对齐、改一句重生成都只影响一句。
- 语速用 `duration_factor`（>1 变慢、<1 变快），由 `measure_rate.py` 的建议值反推。
- 输出 meta.json（每句字数/时长/字速），供 QC 与时间轴对齐。

用法（WSL / Linux，仓库已 clone 到 ~/tts/index-tts）::

    cd ~/tts/index-tts
    uv run python /mnt/c/.../scripts/video/tts_batch_indextts.py \\
        --ref /mnt/c/.../course/04-视频/voice/ref/main-ref.wav \\
        --text-file /mnt/c/.../scripts/video/tts-test-lines.txt \\
        --out-dir /mnt/c/.../course/04-视频/voice/p1test \\
        --duration-factor 1.0

    # 只打印真实 API 签名，不加载模型
    uv run python scripts/video/tts_batch_indextts.py --print-api
"""
from __future__ import annotations

import argparse
import inspect
import json
import os
import re
import sys
import wave

# Windows 控制台/WSL 管道都可能不是 UTF-8，中文日志统一按 UTF-8 输出。
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, OSError):
        pass


def read_lines(path: str) -> list[str]:
    lines: list[str] = []
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            lines.append(line)
    return lines


def count_chars(text: str) -> int:
    """与分镜预算一致：非空白字符数。"""
    return len(re.sub(r"\s+", "", text))


def wav_duration(path: str) -> float | None:
    try:
        with wave.open(path, "rb") as wf:
            return wf.getnframes() / float(wf.getframerate())
    except (OSError, wave.Error):
        return None


def filtered_kwargs(sig: inspect.Signature, candidates: dict) -> dict:
    return {k: v for k, v in candidates.items() if k in sig.parameters}


def describe_params(sig: inspect.Signature) -> str:
    parts = []
    for name, p in sig.parameters.items():
        if name == "self":
            continue
        default = "" if p.default is inspect.Parameter.empty else f"={p.default!r}"
        parts.append(f"{name}{default}")
    return ", ".join(parts)


def main() -> int:
    ap = argparse.ArgumentParser(description="IndexTTS-2.5 逐句合成")
    ap.add_argument("--ref", help="参考音频（8–12s 单声道 WAV）")
    ap.add_argument("--text-file", help="逐句文本（UTF-8，# 注释）")
    ap.add_argument("--text", action="append", default=[], help="直接给一句（可重复）")
    ap.add_argument("--out-dir", help="输出目录（seg_000.wav ... + meta.json）")
    ap.add_argument("--cfg-path", default="checkpoints/config.yaml")
    ap.add_argument("--model-dir", default="checkpoints")
    ap.add_argument("--lang", default="ZH")
    ap.add_argument("--duration-factor", type=float, default=1.0,
                    help="语速：>1 变慢、<1 变快（2.5 的 duration_factor）")
    ap.add_argument("--max-tokens", type=int, default=120,
                    help="自动分段的最大文本 token 数（显存不够时调小）")
    ap.add_argument("--interval-silence", type=int, default=200,
                    help="段内分段之间的静音毫秒")
    ap.add_argument("--prefix", default="seg", help="输出文件名前缀")
    ap.add_argument("--limit", type=int, default=0, help="只合成前 N 句（0=全部）")
    ap.add_argument("--skip-existing", action="store_true", help="已存在的句跳过")
    ap.add_argument("--fp32", action="store_true", help="用 fp32（默认 bf16）")
    ap.add_argument("--device", default=None, help="cuda / cpu，默认交给模型")
    ap.add_argument("--print-api", action="store_true", help="只打印真实 API 签名")
    ap.add_argument("--dry-run", action="store_true", help="打印将要传的参数，不加载模型")
    args = ap.parse_args()

    try:
        from indextts.infer_v2_5 import IndexTTS2  # noqa: WPS433
    except ImportError as exc:
        print(f"!! 导入 IndexTTS 失败：{exc}", file=sys.stderr)
        print("   > 必须在已 `uv sync` 的 index-tts 仓库根目录用 `uv run python ...` 执行", file=sys.stderr)
        return 2

    infer_sig = inspect.signature(IndexTTS2.infer)
    ctor_sig = inspect.signature(IndexTTS2.__init__)

    if args.print_api:
        print(f"IndexTTS2.__init__({describe_params(ctor_sig)})")
        print()
        print(f"IndexTTS2.infer({describe_params(infer_sig)})")
        return 0

    # 文本来源
    lines: list[str] = []
    if args.text_file:
        lines.extend(read_lines(args.text_file))
    lines.extend(args.text)
    if not lines:
        print("!! 没有文本（--text-file 或 --text）", file=sys.stderr)
        return 2
    if args.limit > 0:
        lines = lines[: args.limit]
    if not args.ref or not os.path.isfile(args.ref):
        print(f"!! 参考音频不存在：{args.ref}", file=sys.stderr)
        return 2
    if not args.out_dir and not args.dry_run:
        print("!! 需要 --out-dir", file=sys.stderr)
        return 2

    ctor_kwargs = filtered_kwargs(
        ctor_sig,
        {
            "cfg_path": args.cfg_path,
            "model_dir": args.model_dir,
            "use_bf16": not args.fp32,
            "device": args.device,
            "verbose": True,
        },
    )
    print(f"[api] IndexTTS2({describe_params(ctor_sig)})")
    print(f"[api] infer({describe_params(infer_sig)})")
    print(f"[api] 构造参数：{ctor_kwargs}")

    if args.dry_run:
        sample = filtered_kwargs(
            infer_sig,
            {
                "spk_audio_prompt": args.ref,
                "text": lines[0],
                "output_path": os.path.join(args.out_dir or ".", f"{args.prefix}_000.wav"),
                "lang": args.lang,
                "duration_factor": args.duration_factor,
                "text_normalization": True,
                "max_text_tokens_per_segment": args.max_tokens,
                "interval_silence": args.interval_silence,
                "verbose": True,
            },
        )
        print(f"[dry-run] infer 参数：{sample}")
        print(f"[dry-run] 共 {len(lines)} 句")
        return 0

    os.makedirs(args.out_dir, exist_ok=True)
    tts = IndexTTS2(**ctor_kwargs)

    meta: list[dict] = []
    failures = 0
    for idx, text in enumerate(lines):
        out_path = os.path.join(args.out_dir, f"{args.prefix}_{idx:03d}.wav")
        if args.skip_existing and os.path.isfile(out_path):
            print(f"[skip] {out_path} 已存在")
            existing = wav_duration(out_path)
            meta.append(
                {
                    "index": idx,
                    "text": text,
                    "chars": count_chars(text),
                    "wav": os.path.basename(out_path),
                    "duration": None if existing is None else round(existing, 3),
                    "status": "skip",
                }
            )
            continue

        kwargs = filtered_kwargs(
            infer_sig,
            {
                "spk_audio_prompt": args.ref,
                "text": text,
                "output_path": out_path,
                "lang": args.lang,
                "duration_factor": args.duration_factor,
                "text_normalization": True,
                "max_text_tokens_per_segment": args.max_tokens,
                "interval_silence": args.interval_silence,
                "verbose": True,
            },
        )
        print(f"[tts ] {idx:03d} ({count_chars(text)} 字) {text}")
        try:
            tts.infer(**kwargs)
        except Exception as exc:  # noqa: BLE001 - 单句失败不应中断整批
            failures += 1
            print(f"!! 第 {idx} 句合成失败：{exc}", file=sys.stderr)
            meta.append(
                {
                    "index": idx,
                    "text": text,
                    "chars": count_chars(text),
                    "wav": os.path.basename(out_path),
                    "duration": None,
                    "status": "failed",
                    "error": str(exc),
                }
            )
            continue

        dur = wav_duration(out_path)
        if dur is None:
            failures += 1
            print(f"!! 第 {idx} 句未产出可读 wav：{out_path}", file=sys.stderr)
        else:
            print(f"       -> {os.path.basename(out_path)} {dur:.2f}s")
        meta.append(
            {
                "index": idx,
                "text": text,
                "chars": count_chars(text),
                "wav": os.path.basename(out_path),
                "duration": None if dur is None else round(dur, 3),
                "status": "ok" if dur is not None else "failed",
            }
        )

    meta_path = os.path.join(args.out_dir, "meta.json")
    payload = {
        "ref": os.path.abspath(args.ref),
        "duration_factor": args.duration_factor,
        "lang": args.lang,
        "items": meta,
    }
    with open(meta_path, "w", encoding="utf-8") as fh:
        json.dump(payload, fh, ensure_ascii=False, indent=2)
    print(f"[meta] {meta_path}")

    ok = [m for m in meta if m["status"] == "ok"]
    if ok:
        total_dur = sum(m["duration"] for m in ok)
        total_chars = sum(m["chars"] for m in ok)
        print(
            f"[done] {len(ok)}/{len(meta)} 句成功，{total_chars} 字，"
            f"{total_dur:.2f}s，平均 {total_chars / total_dur:.2f} 字/秒"
        )
    else:
        print("[done] 没有成功合成的句子", file=sys.stderr)
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
