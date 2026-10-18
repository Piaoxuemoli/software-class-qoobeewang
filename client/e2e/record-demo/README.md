# 客户端分段录制（P2·task-5）

DataBackup 演示视频的 4 个客户端段（seg-02-setup / seg-03-backup / seg-04-manage / seg-05-restore）
的 Playwright 录制脚本。**只改本目录，不动 `client/src` 与 `backend/`。**

## 快速开始（宿主 Windows Git Bash，仓库根目录）

```bash
# 1) 彩排第一遍：带对齐色块（精确测 kMs），录到 work/rehearsal/…-pass1.{webm,json}
SEG_MODE=rehearsal SEG_PASS=1 SEG_SYNC_FLASH=1 bash client/e2e/record-demo/run-in-docker.sh

# 2) 彩排第二遍：用第一遍实测的 kMs，确认动作落在分镜时间点
SEG_MODE=rehearsal SEG_PASS=2 bash client/e2e/record-demo/run-in-docker.sh

# 3) 正式录制：work/seg/<id>.webm + <id>.json + annotation-boxes.json + actual-run/
SEG_MODE=formal bash client/e2e/record-demo/run-in-docker.sh

# 只跑某一段（调试用）
SEG_IDS="seg-02-setup" SEG_MODE=rehearsal SEG_PASS=1 SEG_SYNC_FLASH=1 \
  bash client/e2e/record-demo/run-in-docker.sh
```

> 正式录制前请确认没有另一个 teammate 同时在容器里录屏（CPU 抢占会掉帧/拉长后端耗时）。

## 目录

```text
shared/seed.mjs       数据预置（独立重建 /home/demo：~500MB 源树 + 仓库预置；与录制分离）
shared/cursor.mjs     DOM 光标层（录屏无 OS 光标 → 页面内注入箭头 + 点击涟漪 + 平滑移动）
shared/timeline.mjs   分镜时间轴（成片秒 ↔ 墙钟，kMs 偏移）
shared/actions.mjs    真人感鼠标/键入/进度轮询等动作
shared/record.mjs     通用运行器（启动 / 录屏 / 日志 / 关片 / ffmpeg 量时长 / 对齐色块测 kMs / finalize）
shared/actual-run.mjs 录制后的真实产物捕获（diff/FIFO/软链接/ls/完成卡数字，供 S6 卡片与 S3/S5 字幕对齐）
shared/calibrate.mjs  后端速度校准（备份/还原/错误口令耗时，不录屏）
segments/*.mjs        4 个段脚本
record-all.sh         容器内入口（Xvfb + 环境 + 逐段执行）
run-in-docker.sh      宿主一键入口（docker run + node_modules 挂卷）
```

## 硬前提（PROBE-infra §B0/B1 实测）

- 必须在 `databackup-dev` 容器里跑（宿主 `client/node_modules` 是 Linux 依赖卷，无 Electron 二进制）。
- `PLAYWRIGHT_BROWSERS_PATH=/workspace/course/04-视频/tools/ms-playwright`（不设 recordVideo 会挂死）。
- `Xvfb :98 -screen 0 2560x1600x24`；`--force-device-scale-factor=2`；窗口 CSS 1120×720 / 物理 2240×1440。
- `recordVideo {size:2240x1440, fps:30}`；`HOME=/home/demo`、`DATABACKUP_BIN` 指向 v1.0.1 二进制。
- 录屏不含鼠标指针 → 注入 DOM 光标层（本目录 `cursor.mjs`）。
- 关片前必须等后端任务结束（`guardClose` 会拦运行中的任务）。

## 输出与日志约定

| 模式            | webm                                                      | JSON                 | 其它                                                            |
| --------------- | --------------------------------------------------------- | -------------------- | --------------------------------------------------------------- |
| rehearsal passN | `course/04-视频/work/rehearsal/<id>-rehearsal-passN.webm` | 同名 `.json`         | `work/seed/*.json`、`work/logs/*.log`、`actual-run/<id>-passN/` |
| formal          | `course/04-视频/work/seg/<id>.webm`                       | `work/seg/<id>.json` | `work/seg/annotation-boxes.json`、`work/seg/actual-run/`        |

JSON 里关键字段：

- `video.durationSec`：webm 实际时长（目标 = 分镜时长 + 1.8s 余量，P3 再裁）。
- `timing.kMsMeasured` / `timing.contentOffsetSec`：**视频原点 → 内容原点**的偏移。
  视频前 `contentOffsetSec` 秒是启动/稳定画面；P3 裁掉这一段后，`events[].contentMs`
  就是成片坐标（与 `video-storyboard.json` 的分镜时间轴对齐）。
- `keyStates[]`：关键状态出现时刻（`videoMs` 原始视频坐标，`contentMs` 裁掉启动段后的成片坐标）。
- `annotations`：注解目标控件的 CSS 坐标（窗口 1120×720，×2 = 物理像素）。

## 已知事实（2026-10-18 实测）

- 备份源树 ≈510MB（含 node_modules 120 文件）；S3 真实备份 ≈13.2s（Huffman+AES，输出 274 MB，压缩率 59.8%）。
- S5 加密备份还原 ≈13.7s（先整文件校验再写盘）；错误口令被拒 ≈1.5s。
- 录屏视频时长 ≈ 墙钟「时间轴起点→关片」+ kMs（实测 kMs ≈0.5s，见每次 rehearsal JSON 的 `timing.kMsFlashMeasured`）。

### 正式录制结果（4 段，2026-10-18）

| 段             | 文件                           |   时长 | 帧数 | 目标 | 备注                        |
| -------------- | ------------------------------ | -----: | ---: | ---: | --------------------------- |
| seg-02-setup   | `work/seg/seg-02-setup.webm`   | 40.27s | 1208 |  37s | 尾部 ~1.3s 为静态停留，可裁 |
| seg-03-backup  | `work/seg/seg-03-backup.webm`  | 29.93s |  898 |  28s | 备份 ~13.2s                 |
| seg-04-manage  | `work/seg/seg-04-manage.webm`  | 27.03s |  811 |  25s | 独立段，从管理页开始        |
| seg-05-restore | `work/seg/seg-05-restore.webm` | 36.87s | 1106 |  35s | 错误口令 1 次，还原 13.7s   |

- 全部 2240×1440 / 30fps / VP9；`contentOffsetSec` ≈ 0.5s（P3 裁掉开头启动段后，`events[].contentMs` 即分镜坐标）。
- 每段带同名 `.json`（关键状态/完成卡读数/注解坐标）、`annotation-boxes.json`（12 个注解目标 CSS 坐标 + boxSemantics/note）、`actual-run/`（录制同一次运行的真实产物，S6 卡片与 S3/S5 数字同源）。
- 抽帧 QC：`work/seg/frames-app/`（每段 5 帧 + 首帧），状态栏 `后端版本: 1.0.1`、光标可见、仅 `/home/demo` 路径、无敏感信息；开头黑帧 0.07–0.17s（Electron 首帧，落在 contentOffset 内，后期裁掉）。
- 已知客户端 UI 小瑕疵（不改 client/src）：还原进度卡标题显示「还原中 · 还原中」（RunningCard 的 kind+phase 文案重复）。
