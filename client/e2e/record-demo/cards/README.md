# 卡片段制作（seg-00-title / seg-01-data / seg-06-verify）

DataBackup 演示视频的 3 个「卡片段」，在 `databackup-dev` 容器内用 Playwright Chromium +
Xvfb 录成 **1920×1080 @ 30fps 无声 webm**（VP9，Playwright `recordVideo` 直出）。

| 段落            | 目标时长 | 内容                                                                                          |
| --------------- | -------- | --------------------------------------------------------------------------------------------- |
| `seg-00-title`  | 9s       | 片头卡：黑底 + 左侧深绿竖条 + `DataBackup` / 副标题 / 署名行；文字淡入 300ms                  |
| `seg-01-data`   | 20s      | 终端卡（占画面 80%）：真实 `ls -la` / `find` 输出逐行打字机（每行 120ms）+ 空仓库             |
| `seg-06-verify` | 24s      | 0–8s 真实 `diff -r`（无输出）+ `ls -l *.dbk`；8–16s 数据卡；16–22s 谢谢+署名；22–24s 淡出黑场 |

**卡片只画内容像素，不画字幕**：字幕与注解由 P3 用 ASS 统一叠加（依据
`course/04-视频/work/cards/card-layout.json` 的几何）。

## 文件

```text
card.html / card.css / card.js     卡片页面与渲染器（renderAt(t) 单一时间入口，彩排/录制共用）
capture-real-output.sh             容器内：预置 /home/demo（~360MB）+ 真实备份/还原 + 采集真实命令输出
build-scenes.mjs                   由 raw/commands.txt 生成 scenes.json（禁止手写假输出，含一致性校验）
render-scenes.mjs                  rehearse（几何 JSON + 抽帧 PNG）/ record（webm）/ probe（链路探针）
run-in-docker.sh                   容器内一键入口（capture|build|probe|rehearse|record|all）
finalize-layout.mjs                宿主侧：ffprobe/blackdetect/抽帧 + 回填 card-layout.json
```

产物（gitignored，本地）：

```text
course/04-视频/work/cards/raw/commands.txt        真实命令 + 原始 stdout + 退出码（证据）
course/04-视频/work/cards/real-commands.txt       人读证据（命令原文与输出）
course/04-视频/work/cards/scenes.json             场景内容 + 时间轴 + 注解目标
course/04-视频/work/cards/card-layout.json        ★ P3 生成 ASS 注解的几何（行 y/字号/出现时刻/目标框）
course/04-视频/work/rehearsal/cards/*.json        每段彩排时序（元素出现时刻、y 坐标、字号）
course/04-视频/work/rehearsal/cards/frames/*.png  彩排抽帧
course/04-视频/work/seg/seg-*.webm                正式录制产物
course/04-视频/work/seg/frames-cards/*.png        录制抽帧（目检用）
course/04-视频/work/seg/ffprobe-cards.txt         ffprobe/blackdetect 报告
```

## 数据真实性（硬约束）

终端卡里的每一行都来自容器内真实命令的 stdout：`capture-real-output.sh` 在真实
`/home/demo/documents` 上跑 `ls -la` / `find` / `diff -r` / `ls -l`，并用真实后端
（`backend/bin/databackup` v1.0.1，huffman + aes256）做一次真实备份与真实还原。
`build-scenes.mjs` 会校验「画面显示的命令 == 实际执行的命令」，并扫描敏感信息
（只允许 `/home/demo`，不允许个人路径/口令）。

已知事实：GNU diffutils 3.8 对 FIFO 恒输出 `File ... is a fifo while file ...`（即使两侧
都是 FIFO），所以视频里的 diff 在分镜的 `--exclude node_modules` 之外再加 `--exclude pipe`，
才是真实「无输出」；严格版命令的输出保留在 `raw/commands.txt` 的 `seg06-diff-strict` 块。

## seg-06 的数据来源（actual-run 优先）

`sources/` 里没有任何手写输出：`build-scenes.mjs` 对 seg-06 按下面的优先级取数：

1. **`course/04-视频/work/seg/actual-run/`**（app-recorder 正式录制 seg-03/seg-05 后由
   `client/e2e/record-demo/shared/actual-run.mjs` 采集的真实产物）——与 S3 完成卡同一次运行，
   满足 07-…§9「完成卡数字与终端输出一致」。文件是带分节的 transcript
   （`# 命令` / `--- stdout ---` / `--- stderr ---` / `(空)`），由 `parseActualRunTranscript` 解析；
   `stats JSON` 的 `run.mode` 若为 `rehearsal` 则拒绝使用（彩排数字不进成片）。
   - diff 取 `seg05-diff.txt`（必须 stdout 为空、rc=0）；FIFO 取 `seg05-fifo-check.txt` 的
     `FIFO 类型一致` 行；`ls -l *.dbk` 优先取 `seg03-ls-dbk.txt`（只有本次 documents-demo.dbk，
     与分镜 §S6 单行示例和 S3 完成卡数字一致），没有时退回 `seg05-ls-dbk.txt`（还原后仓库里
     还有 3 个历史备份，会显示 4 行；此时 card-layout 会额外给 `dbk-documents-demo` 目标框）。
2. 没有可用 actual-run 时，退回本脚本自己的真实采集（`raw/commands.txt`），并在
   `scenes.json` / `card-layout.json` 的 `provenance.seg06Source` 里明确标注 `own-capture-fallback`
   —— 这种 fallback 与客户端 S3 完成卡不是同一次运行，交付前需 Lead 确认。

## 字体（容器内实测）

- 中文/无衬线：`Noto Sans CJK SC`（`fc-list :lang=zh` 30 条）
- 等宽：`DejaVu Sans Mono`（`fc-list :spacing=mono`），中文回退 `Noto Sans Mono CJK SC`

## 运行（宿主 Git Bash）

```bash
cd "C:/Users/Qoobeewang/Desktop/软件工程实验"
export MSYS_NO_PATHCONV=1
VOL=$(bash scripts/docker.sh volumes | awk '/databackup-node-/{print $1; exit}')

run() {
  docker run --rm \
    -v "C:/Users/Qoobeewang/Desktop/软件工程实验:/workspace" -w /workspace \
    -v "$VOL:/workspace/client/node_modules" \
    -e DISPLAY= databackup-dev \
    bash /workspace/client/e2e/record-demo/cards/run-in-docker.sh "$@"
}

run capture     # 预置数据 + 真实命令/备份/还原 + 采集证据
run build       # raw -> scenes.json
run probe       # 可选：4s 片头卡链路探针（headless + Xvfb headed 各一份）
run rehearse    # 彩排：rehearsal/cards/*.json + card-layout.json + PNG 抽帧
# —— 报告 Lead、等通知后再正式录（避免与客户端段同时录屏抢 CPU）——
run record      # 正式录制 -> course/04-视频/work/seg/seg-*.webm
node client/e2e/record-demo/cards/finalize-layout.mjs   # 宿主侧 ffprobe/抽帧/回填
```

一次性依赖（持久目录，不装系统软件）：

```bash
# Playwright ffmpeg（recordVideo 硬前提，PROBE-infra.md §B0）与 Chromium
docker run --rm -v "<repo>:/workspace" -w /workspace/client \
  -v "$VOL:/workspace/client/node_modules" \
  -e PLAYWRIGHT_BROWSERS_PATH=/workspace/course/04-视频/tools/ms-playwright \
  databackup-dev bash -c \
  'node node_modules/playwright-core/cli.js install ffmpeg && node node_modules/playwright-core/cli.js install chromium'
```

## 录制参数与两个已踩的坑

- viewport `1920×1080`、`deviceScaleFactor: 1`、`recordVideo: { size: 1920×1080, fps: 30 }`
- 容器内 Xvfb `:97 -screen 0 1920x1080x24`（与客户端组的 `:98` 错开）
- 正式录制每段等于「分镜时长 + 1s」；动画时钟从页面就绪时刻开始，webm 第一帧就是 scene t=0，
  P3 按镜头起点直接对齐，`contentStartSec=0`。
- **坑 1（headless 不可用）**：容器内 Playwright 1.64 + Chromium 156 的 chromium-headless-shell
  与 Playwright 的 `--remote-debugging-pipe` 不兼容，启动即报
  `Headless commands are not compatible with remote debugging`。因此卡片段统一走
  **headed + Xvfb :97**（与任务要求的 Xvfb 链路一致；`CARD_HEADED=1`）。
- **坑 2（录像时间轴偏移 +1.22s）**：webm 时长 ≈ `(tClose − tCtx) + 1.22s`（Playwright 录像器
  时间轴相对脚本时钟的固定偏移）。已用 probe 6s/10s 两点标定：录制时等待
  `recordSec − loadMs − CARD_RECORD_OVERHEAD_MS(默认 1220)`，成片时长 ≈ 分镜时长 +1s，误差 ≤1 帧。
  probe 数据见 `course/04-视频/work/cards/probe-video/`。
- 只录某几段：`run-in-docker.sh record --only seg-00-title,seg-01-data`（逗号分隔）。
- 不修改 `client/src`、`backend`、docker 镜像；不 `git add/commit/push`。
