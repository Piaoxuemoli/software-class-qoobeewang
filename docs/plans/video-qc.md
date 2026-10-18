# 演示视频 P3：合成与 QC 报告（DataBackup v1.0.1）

> 对应 [视频任务交接 §5](../project/video-handoff.md) 的 P3/P4/P5 门禁；P2 分镜与录制见 [video-phase2.md](video-phase2.md)，P1 配音见 [video-phase1.md](video-phase1.md)。
> 成片：`course/04-视频/DataBackup-demo-1.0.1.mp4`（本地，gitignored；上传 B 站/学校平台由用户本人完成）。

## 1. 成片参数（ffprobe 实测）

| 项 | 实测值 | 要求 | 结论 |
|---|---|---|---|
| 时长 | **177.967 s**（2:57.97） | ≤180 s | ✅ |
| 分辨率 | **1920×1080** | 1920×1080 | ✅ |
| 帧率 | **30.000 fps**（5339 帧） | 30 fps | ✅ |
| 视频编码 | H.264 **High@4.1**，yuv420p | H.264 | ✅ |
| 音频编码 | AAC **48000 Hz / 2 ch** / 192 kbps | AAC | ✅ |
| 响度 | **−16.5 LUFS**，真峰值 **−4.5 dBFS**，LRA 6.5 LU | −16 LUFS / TP ≤ −1.5 dBFS | ✅ |
| 文件大小 | 5.37 MB（平均码率约 241 kbps） | — | 深色 UI + 静态内容，CRF18 下码率低属正常 |
| faststart | **true**（moov 在 mdat 前） | 网页/手机播放 | ✅ |
| 黑帧 | **0 段**（全帧 YMAX 检测） | 无异常黑帧 | ✅ |

## 2. 自动 QC（`scripts/video/qc.py`）

`course/04-视频/work/qc/qc.json`：`problems: []`、`warns: []`。检查项：

- 时长/分辨率/帧率/编码/音频参数/声道/采样率/响度/真峰值/黑帧/分段时长 —— **全部通过**；
- 抽帧 22 张（每镜 25% 与 75% 位置 + 关键时间点）→ `course/04-视频/work/qc/frames/`；
- 补充人工抽帧：客户端段 `work/seg/frames-app/`（每段 5 帧+首帧）、卡片段 `work/seg/frames-cards/`（13 帧）、Lead 复核帧 `work/qc/leadp3/`。

> 说明：ffmpeg `blackdetect` 对本片不可用——UI 设计就是纯黑画布，会把正常界面判成黑场；`qc.py` 改为 2 fps 采样 `signalstats.YMAX`，只把「整帧近全黑」计为黑帧（正常界面/卡片总有白色文字）。

## 3. 逐段结果

| 段 | 成片区间 | 目标 | 实得 | contentOffset | 抽帧确认 |
|---|---|---:|---:|---:|---|
| seg-00-title | 0:00–0:09 | 9 s | 9.000 s | 0 s | 片头标题/副标题/署名正常 |
| seg-01-data | 0:09–0:29 | 20 s | 20.000 s | 0.033 s | 真实 `ls -la`/`find`/空仓库；光标闪烁 |
| seg-02-setup | 0:29–1:06 | 37 s | 37.000 s | 0.494 s | 编排/过滤/压缩/加密/预览；状态栏 1.0.1 |
| seg-03-backup | 1:06–1:34 | 28 s | 28.000 s | 0.530 s | 进度→完成卡：311 文件/8 目录/458 MB→274 MB/59.8% |
| seg-04-manage | 1:34–1:59 | 25 s | 25.000 s | 0.496 s | 4 行列表（加密/完整性）、详情 Sheet |
| seg-05-restore | 1:59–2:34 | 35 s | 34.967 s | 0.576 s | 错误口令 1 次（还可尝试 2 次）→ 还原完成卡 |
| seg-06-verify | 2:34–2:58 | 24 s | 24.000 s | 0.100 s | `diff` 无输出、FIFO/软链接校验、4 个 `.dbk` |
| **合计** | **2:58** | **178 s** | **177.967 s** | — | — |

关键动作落点（成片坐标，与分镜的偏差）：

- S2：填源目录 3.10 / 排除胶囊 6.60 / 命名 12.60 / 压缩 18.60 / 加密 24.60 / 预览 30.41 → 偏差 ≤0.15 s；
- S3：备份提交 2.79（分镜 0–2，+0.79）、完成卡 15.95；
- S4：管理页 0.5、行悬停 3.2–6.65、点行 8.40、Sheet 9.78、还原按钮 21.43（+0.93）；
- S5：口令对话框 7.85、错误提示 12.45（分镜 12.5）、还原完成卡 31.76（分镜 29.5–34）。

## 4. 对照 07-…§9 QC 清单

| # | 检查项 | 结果 | 证据 |
|---|---|---|---|
| 1 | 时长：每段与分镜差 ≤1 s；总时长 ≤179 s | ✅ | §3；总 177.967 s |
| 2 | 分辨率/帧率：1920×1080 / 30fps；无黑帧、无撕裂 | ✅ | §1、§2（YMAX 黑帧检测 0 段） |
| 3 | 字幕：不与注解/控件重叠；单行 ≤18 字；出现/消失与操作同步 | ✅（1 处小瑕疵见 §6-3） | `gen_ass.py` 校验通过；抽帧复核 |
| 4 | 旁白：与字幕一致；无吞字；数字读法正确；响度 −16 LUFS | ✅（读数需人耳最终确认） | 响度实测；ASR 粗筛 7 句完整；§6-5 |
| 5 | 操作：目标控件始终在画面内；鼠标不遮挡关键文字；错误口令只 1 次 | ✅ | 录屏内注入光标；seg-05 只错 1 次 |
| 6 | 敏感信息：无学号、无真实口令、无个人路径（除 `/home/demo`） | ✅ | 抽帧扫描：仅 `/home/demo/**`；口令为圆点；界面状态栏 1.0.1 |
| 7 | 数据：完成卡数字与终端输出一致；`diff` 无输出 | ✅ | actual-run 同一次运行采集：274 MB ↔ 287,018,604 B；diff rc=0 空输出 |
| 8 | 版本：界面状态栏为 1.0.1 | ✅ | 每段抽帧均含「后端版本: 1.0.1」 |
| 9 | 全片通看 2 遍（画面一遍、闭眼听一遍） | ⏳ **需用户完成** | 见 §7 |

## 5. 合成流水线（可复现）

```bash
# 0) 前置：P2 录制段（work/seg-frozen/ 为冻结副本）、旁白（voice/narration/）、ASS 生成器
python scripts/video/build_audio.py \
  --out course/04-视频/work/audio/narration-full.wav \
  --placements-out course/04-视频/work/audio/placements.json
python scripts/video/gen_ass.py \
  --storyboard docs/deliverables/video-storyboard.json \
  --boxes course/04-视频/work/seg/annotation-boxes.json \
  --boxes course/04-视频/work/cards/card-layout.json \
  --out course/04-视频/work/ass/demo.ass \
  --timeline-out course/04-视频/work/ass/timeline.json

# 1) 逐段编码（2240×1440→1680×1080+120px 黑边 / 卡片段原样；按 json 的 contentOffset 裁头）
# 2) concat -c copy 拼接；3) 两遍 loudnorm（-16 LUFS/TP -1.5）；4) 烧 ASS + AAC + faststart
SEG_DIR=course/04-视频/work/seg-frozen bash scripts/video/compose.sh all

# 5) QC
python scripts/video/qc.py --video course/04-视频/DataBackup-demo-1.0.1.mp4 \
  --json-out course/04-视频/work/qc/qc.json --frames-dir course/04-视频/work/qc/frames
```

产物：成片 `course/04-视频/DataBackup-demo-1.0.1.mp4`；中间文件 `work/compose/`、`work/audio/`、`work/ass/`、`work/qc/`（全部 gitignored）。

## 6. 已知问题与偏离（需用户知悉）

1. **`ls -l` 属主不一致**：seg-01 卡片显示 `demo demo`（自采），seg-06 显示 `root root`（与客户端段同一次运行的真实产物）。均为真实输出，未改写；不影响内容与演示效果。
2. **客户端既有 UI 文案重复**：还原进度卡显示「还原中 · 还原中」（RunningCard 的 kind + phase 拼接），是 v1.0.1 既有 UI；本轮遵守「不改 `client/src/`」纪律，仅记录。若要修需改客户端并重出截图基线，属于范围外。
3. **S4 有 1 处标签轻微压字**：`加密：是` 的标签放在「加密」单元格右侧时，会短暂（2.6 s）压住相邻「完整性」列的文字。属注解标签排版取舍；如需完全避让可调整该 2 条注解的标签方位（改 `gen_ass.py` 规则 + 重烧 ASS，约 1 分钟）。
4. **分镜 S6 注解窗口修正**：`terminal-ls` 由 10–14 s 改为 5.4–8.0 s（终端相位 8 s 就切走），`terminal-diff` 改 2.0–5.2 s；已同步 `video-storyboard.json`。
5. **旁白内容最终以人耳为准**：IndexTTS 克隆的 `AES-256` / `.dbk` / `Huffman` 读法无法自动判定（whisper-small 回译有同音字误识），请试听确认；若个别读法不理想，可改文案/加拼音注音后重生成单句，再重跑 P3。
6. **`diff` 口径与 07 原稿不同**：容器 diffutils 3.8 对 FIFO 会报差异，故画面使用 `--exclude node_modules --exclude pipe` + `test -p` 证明 FIFO 还原（已在 `07-…` S6 说明中修正）。
7. **字幕停留**按 §6 规范规整为 2.4–4.5 s（原分镜部分字幕写 6–9 s），文案未变。
8. **`seg-02` 录了 40.27 s**（目标 +3.27），内容在 36.9 s 前完成，P3 裁到 37 s；尾部静态丢弃。

## 7. 尚需用户完成（agent 无法替代）

- **全片通看两遍**：一次看画面（字幕/注解/光标/口径），一次闭眼听旁白（吞字、数字/英文读法）。agent 无播放与听觉能力，只能做抽帧与客观测量。
- **上传** B 站/学校平台并取得链接，填入 M9-6 提交包（H6）。
- 如需修改：改文案 → 重生成对应单句（`tts_batch_indextts.py`）→ 重跑 `build_audio.py` + `gen_ass.py` + `compose.sh`，全链路已脚本化（约 6–8 分钟）。

## 8. 交付清单

| 类别 | 文件 |
|---|---|
| 成片 | `course/04-视频/DataBackup-demo-1.0.1.mp4` |
| 旁白 | `course/04-视频/voice/narration/seg_00…06.wav`、`listen-all.wav`、`meta.json`、`budget.json` |
| 录制段（冻结） | `course/04-视频/work/seg-frozen/seg-*-*.webm`（原始在 `work/seg/`，含 json/actual-run/抽帧） |
| 字幕/注解 | `course/04-视频/work/ass/demo.ass`、`timeline.json` |
| 字幕烧录与合成 | `scripts/video/{gen_ass.py,compose.sh,build_audio.py,qc.py}` |
| 录制脚本 | `client/e2e/record-demo/{shared,segments,cards}/`、`record-all.sh`、`run-in-docker.sh` |
| QC 证据 | `course/04-视频/work/qc/`（qc.json + 22 帧）、`work/PROBE-infra.md` |
| 设计文档 | `docs/plans/video-phase1.md`、`video-phase2.md`、本文件 |
