# 演示视频 P2：分镜固化、旁白合成与分段录制

> 对应 [视频任务交接 §5](../project/video-handoff.md) 的 P2/P3 门禁。P1（配音方案）已由用户确认通过，见 [video-phase1.md](video-phase1.md)。
> 时间轴机器可读版：[video-storyboard.json](../deliverables/video-storyboard.json)（P2 已固化更新）。
> 状态：**已完成**——旁白已固化合成（74.07 s ≤80 s，§3）；7 段录制与彩排全部完成（§5 含结果、时间轴修正与遗留）。

## 1. 目标与门禁

| 门禁项 | 要求 | 当前 |
|---|---|---|
| 分镜固化 | 7 镜时间轴、每镜旁白/字幕/注解与操作脚本对齐 | ✅ §2 |
| 旁白总时长 | ≤80 s | ✅ **74.07 s** |
| 每镜不超时 | 旁白 ≤ 镜头时长 −0.8 s | ✅ 全部通过（最小余量 2.42 s） |
| 每镜动作与字幕对齐 | 操作时刻落在分镜时间点 ±1 s | ⏳ 录制中（§5） |
| 用户确认文案 | P2 门禁 | ⏳ 本阶段末一并确认 |

## 2. 固化时间轴（P2 相对 P1 的调整）

P1 预算校核发现两处问题并按规则修正：**S0 旁白 7.73 s 而镜头只有 8 s（余量 0.27 s < 门禁 0.8 s）**；画面总长正好 180 s，无余量。调整（同步已写入 `video-storyboard.json`）：

| 镜 | 原时长 | **新时长** | 原因 |
|---|---:|---:|---|
| S0 片头 | 8 s | **9 s** | 旁白余量 0.27 s → 2.42 s |
| S3 备份执行 | 30 s | **28 s** | 备份 10–14 s + 完成卡停留 10 s 足够；给片头腾 1 s |
| S6 校验片尾 | 25 s | **24 s** | 段尾淡出收紧 |
| 合计 | 180 s | **178 s** | 留 2 s 余量（QC 口径 ≤179 s） |

**最终时间轴（成片坐标）**

| 镜 | 起止 | 时长 | 画面类型 | 旁白主题 |
|---|---|---|---|---|
| S0 | 0:00–0:09 | 9 s | 卡片段 seg-00-title | 一句话定位 |
| S1 | 0:09–0:29 | 20 s | 卡片段 seg-01-data | 演示数据介绍 |
| S2 | 0:29–1:06 | 37 s | 客户端 seg-02-setup | 编排与过滤 |
| S3 | 1:06–1:34 | 28 s | 客户端 seg-03-backup | 流水线与统计 |
| S4 | 1:34–1:59 | 25 s | 客户端 seg-04-manage | 列表与容器头尾 |
| S5 | 1:59–2:34 | 35 s | 客户端 seg-05-restore | 先校验后落盘 |
| S6 | 2:34–2:58 | 24 s | 卡片段 seg-06-verify | 校验与总结 |
| **合计** | **2:58** | **178 s** | | |

## 3. 旁白实测（P2 定稿）

参考音 `voice/ref/main-ref.wav`（AISHELL-1 S0101，Apache-2.0），IndexTTS-2.5，`duration_factor=1.10`，输出 22050 Hz/mono/s16 于 `course/04-视频/voice/narration/seg_00.wav … seg_06.wav`。

| 镜 | 字数 | 预算(s) | **实测(s)** | 字/秒 | 镜头(s) | 剩余余量(s) | 结论 |
|---|---:|---:|---:|---:|---:|---:|---|
| S0 | 34 | 7.73 | **6.58** | 5.16 | 9 | 2.42 | ✅ |
| S1 | 53 | 12.05 | **10.15** | 5.22 | 20 | 9.85 | ✅ |
| S2 | 69 | 15.68 | **14.44** | 4.78 | 37 | 22.56 | ✅（WARN 占比低） |
| S3 | 51 | 11.59 | **10.50** | 4.86 | 28 | 17.50 | ✅（WARN 占比低） |
| S4 | 39 | 8.86 | **10.01** | 3.90 | 25 | 14.99 | ✅ |
| S5 | 50 | 11.36 | **12.03** | 4.16 | 35 | 22.97 | ✅（WARN 占比低） |
| S6 | 41 | 9.32 | **10.37** | 3.95 | 24 | 13.63 | ✅ |
| **合计** | **337** | **76.59** | **74.07** | **4.55** | **178** | — | ✅ ≤80 s |

- 逐句明细：`course/04-视频/voice/narration/meta.json`、`budget.json`。
- 语速不均说明：字符数口径不体现英文/数字的音节展开（`node_modules`、`AES-256`、`18` 等会读得更长），S4/S6 字/s 偏低属正常；整轨 4.55 字/秒在 4.2–4.6 窗口内。
- 3 个 WARN（旁白占镜头 <40%）是设计使然：S2/S3/S5 的画面停留（进度、对话框报错、完成卡）本身就是内容，**不补旁白**。
- 旁白摆放方式（P3）：每镜旁白从镜头起点 +0.3 s 开始（S0 从 0.2 s 起），句间按镜头余量留白；不跨镜。

## 4. 分段录制规范（P2 交付要求）

| 段 | 目标时长 | 类型 | 起始状态 | 输出 |
|---|---:|---|---|---|
| seg-00-title | 9 s | 卡片（1920×1080） | 黑场片头 | `work/seg/seg-00-title.webm` |
| seg-01-data | 20 s | 卡片（终端） | 终端逐行打字 | `work/seg/seg-01-data.webm` |
| seg-02-setup | 37 s | 客户端（2240×1440） | 空仓库 + 源树已预置 | `work/seg/seg-02-setup.webm` |
| seg-03-backup | 28 s | 客户端 | 空仓库，表单就绪 | `work/seg/seg-03-backup.webm` |
| seg-04-manage | 25 s | 客户端 | 仓库预置 3 历史备份 + documents-demo | `work/seg/seg-04-manage.webm` |
| seg-05-restore | 35 s | 客户端 | 同 seg-04，还原目标不存在 | `work/seg/seg-05-restore.webm` |
| seg-06-verify | 24 s | 卡片（终端+片尾） | diff/ls → 片尾卡 | `work/seg/seg-06-verify.webm` |

**环境（PROBE 已实测，见 [PROBE-infra.md](../work/PROBE-infra.md)）**

- 客户端段：`databackup-dev` 容器 + `Xvfb :98 -screen 0 2560x1600x24` + `PLAYWRIGHT_BROWSERS_PATH=/workspace/course/04-视频/tools/ms-playwright`（缺此项 recordVideo 挂死）+ `HOME=/home/demo` + `DATABACKUP_BIN=/workspace/backend/bin/databackup`；`recordVideo {size:2240x1440, fps:30}`；`--force-device-scale-factor=2`。
- 卡片段：同容器 + chromium（已装 `tools/ms-playwright`）+ `Xvfb :97 -screen 0 1920x1080x24`；`recordVideo {size:1920x1080, fps:30}`，viewport 1920×1080。
- **鼠标指针**：Playwright screencast 不含 OS 光标 → 客户端段在页面注入 DOM 光标层（箭头 + 点击反馈，`pointer-events:none`），抽帧确认可见。
- 每段录「目标时长 +1~2 s」，P3 再裁到精确时长。

**录制纪律**

- 每段独立预置 HOME/仓库，互不继承状态（seg-04/05 独立预置 3 个历史备份 + `documents-demo`；seg-03 从空仓库开始）。
- 鼠标：`hover()` + `page.mouse.move(..., {steps:12})` 0.5–0.8 s；点击前停 0.4 s；关键状态后停 0.8 s。
- 口令 `demo-2026`（圆点显示）、错误口令 `wrong-demo` 只输 1 次；不回显、不写日志。
- 不出现：学号、真实口令、API Key、`/home/demo` 以外的个人路径；状态栏必须 `后端版本: 1.0.1`。
- 先彩排记录状态时刻（`work/rehearsal/`），再正式录；两段录制不同时进行（避免抢 CPU 掉帧）。

## 5. 彩排与正式录制结果

7 段全部彩排 → 正式录制完成（每段独占 CPU，无并发录屏）。原始 webm 在 `course/04-视频/work/seg/`，P3 合成用冻结副本 `work/seg-frozen/`（防止后续误覆盖）。

| 段 | 类型 | 目标 | webm 时长 / 帧数 | contentOffset | 抽帧 QC | 结论 |
|---|---|---:|---|---:|---|---|
| seg-00-title | 卡片 | 9 s | 9.933 s / 298 | 0 s | 中文正常、无敏感信息 | ✅ |
| seg-01-data | 卡片 | 20 s | 20.733 s / 622 | 0.033 s | 真实命令输出、光标闪烁、注解行按时出现 | ✅ |
| seg-02-setup | 客户端 | 37 s | 40.27 s / 1208 | 0.494 s | 状态栏 1.0.1、光标贴控件、口令显示圆点 | ✅（尾部 1.3 s 静态被裁） |
| seg-03-backup | 客户端 | 28 s | 29.93 s / 898 | 0.53 s | 完成卡 311/8/458MB/274MB/59.8%/13.1s | ✅ |
| seg-04-manage | 客户端 | 25 s | 27.03 s / 811 | 0.496 s | 4 行列表、加密/完整性标记、Sheet | ✅ |
| seg-05-restore | 客户端 | 35 s | 36.87 s / 1106 | 0.576 s | 错误口令 1 次、完成卡 31.76 s | ✅ |
| seg-06-verify | 卡片 | 24 s | 25.166 s / 755 | 0.1 s | diff 无输出、FIFO/软链接校验、4 行 `.dbk` | ✅ |

动作落点与分镜的偏差（成片坐标）：S2 各项 ≤0.15 s；S3 备份提交 2.79 s（分镜 0–2，+0.8）；S4 Sheet 打开 9.78 s、还原按钮 21.43 s（+0.9）；S5 错误提示 12.45 s（分镜 12.5）、完成卡 31.76 s（分镜 29.5–34）。均 ≤1 s，达标。

### 5.1 录制链路三个坑（已修，写进 `client/e2e/record-demo/*/README.md`）

1. **Playwright recordVideo 需要 registry 里的 ffmpeg 浏览器**：dev 镜像没有 → 开录后页面初始化挂死（窗口在、CDP 全超时、webm 空）。解决：`PLAYWRIGHT_BROWSERS_PATH=course/04-视频/tools/ms-playwright` 并预装该 ffmpeg（probe 阶段已完成）。
2. **静态画面停止出帧**：卡片页面静置后 Chromium screencast 不再产帧，webm 比墙钟短（seg-00 首录只有 7.6 s）。解决：页面加不可察觉 ticker（2×2 px 颜色微变）+ 终端常驻闪烁光标，强制持续重绘。
3. **Chromium 156 headless-shell 与 `--remote-debugging-pipe` 不兼容**：卡片段改用 headed + Xvfb（与客户端段同源链路）。
4. 时长自校准：录像时间轴比脚本时钟固定多约 1.2 s（含启动黑场），录制脚本用「解析 webm Duration → 超差自动重录」的闭环；`work/rehearsal/flash-calibration.json` 记录了逐段 `contentOffset`。

### 5.2 按真实动作重排字幕/注解时间（P2 发现并已修）

录制后发现分镜里有三处时间与真实操作不符，已在 `video-storyboard.json` 修正（P3 的 ASS 由它生成）：

- **S4 行注解被 Sheet 遮挡**：真实 Sheet 在 9.78 s 打开，原注解 `row-encrypted 7.5–10.5`、`row-integrity 11.0–14.0` 会落到被遮住的加密/完整性列 → 改为 `4.0–6.6` / `6.8–9.4`（悬停阶段内），`detail-source` 改 `10.5–14.0`；四条字幕同步前移。
- **S5**：错误提示真实出现 12.45 s（原 14.0）→ 注解改 `12.8–15.6`；进度注解改 `27.0–30.5`（完成卡 31.76 s 前）；两条字幕同步。
- **S3**：完成卡 15.95 s 出现（原字幕 17.5）→ 「完成：统计与压缩率」字幕提前到 `15.6–20.1`。
- **S6**：`terminal-ls` 注解原在 10–14 s，但终端相位 8 s 就切走 → 改 `5.4–8.0`（`ls -l` 行 4.72 s 已出现）；`terminal-diff` 改 `2.0–5.2`。终端卡片内容也按实测修正为「diff 无输出 → `test -p` 证明 FIFO → `ls -l`」（见 `07-…` S6 说明与 §5.3）。

### 5.3 完成卡与终端数字一致性（actual-run 机制）

S6 的 `ls -l` 数字必须与 S3 完成卡一致，但两段是**独立运行**（各自重建 HOME/仓库）。做法：录制脚本在 `app.close()` 之后、容器退出之前，用同一次运行的现场采集真实产物到 `work/seg/actual-run/`：`.dbk` 的 `ls -l`/`stat`、完成卡 innerText（`seg03-stats.json`）、还原后的 `diff`/FIFO/软链接校验、`ls -l *.dbk`（`seg05-*`）。S6 卡片用这些真实输出生成，所以「274 MB ↔ 287,018,604 B」「311 files / 8 dirs」在画面上自洽。

> 两次独立运行里 documents-demo.dbk 体积相差约 0.2–1 KB（源树 mtime/uid 元数据导致，见 `pitfalls.md` §2.4 的同类现象）；四舍五入到 MB 后无差异，字幕不写死字节数。

### 5.4 遗留与已知问题

- **`ls -l` 属主不一致**：seg-01（自采）显示 `demo demo`，seg-06（客户端同源 actual-run）显示 `root root`——都是真实输出，未改写；不影响内容。
- **客户端既有 UI 文案重复**：「还原中 · 还原中」（RunningCard 的 kind+phase），属 v1.0.1 既有 UI，本轮不改 `client/src`（见 §4 纪律），作为已知问题记录。
- seg-02 录 40.27 s（目标 +3.27 s），内容在 36.9 s 前讲完，P3 裁到 37 s，尾部静态丢弃。
- 卡片段 `blackIntervals` 里 seg-06「8.1–25.1 s」是 blackdetect 对深色卡片（平均亮度 <2%）的误判，不是真黑帧，不据此裁剪。

## 6. 逐段 QC 门禁（过不了不进入 P3）

- [ ] 时长与目标差 ≤1 s（+2 s 录制余量另计）
- [ ] 1920×1080（卡片）或 2240×1440（客户端）/ 30 fps / 无坏帧
- [ ] 状态栏 `后端版本: 1.0.1`
- [ ] 无学号、真实口令、API Key、`/home/demo` 以外路径
- [ ] 鼠标光标可见、不遮挡关键文字；点击位置与预期控件一致
- [ ] seg-04/05 仓库列表为「3 历史备份 + documents-demo」；seg-05 错误口令只 1 次且提示「还可尝试 2 次」
- [ ] seg-03 完成卡数字来自真实运行；seg-06 `diff` 无输出
- [ ] 每段抽帧存档（`work/seg/frames-*`）并人工过目

## 7. 与 P3 的接口

- 段文件：`course/04-视频/work/seg/seg-XX-*.webm` + 同名 `.json`（关键状态时刻/目标时长/实际时长）。
- 卡片段布局：`course/04-视频/work/cards/card-layout.json`（终端行 y 坐标与出现时刻、片尾元素位置），供 ASS 注解定位。
- 时间轴：`docs/deliverables/video-storyboard.json`（P2 已固化）为字幕/注解唯一来源；P3 的 ASS 生成器按它 + card-layout 计算绝对时间。
- 旁白：`course/04-视频/voice/narration/seg_00..06.wav`（22050 Hz；P3 统一转 48k/24bit，整轨 loudnorm −16 LUFS / TP ≤−1.5 dBFS）。
