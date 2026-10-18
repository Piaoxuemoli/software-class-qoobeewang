# 新对话 Prompt：DataBackup 演示视频

把下面整段复制给新会话（同一个仓库）。新会话应先读 `docs/project/video-handoff.md`。

---

你在 `C:\Users\Qoobeewang\Desktop\软件工程实验`（DataBackup 课程项目，v1.0.1 已发布）里完成**演示视频**。这是最后一项课程交付物，用户会提供 AI 声音样本与租用的 GPU。

**开始前必读（按顺序）**：
1. `AGENTS.md`（仓库硬约束）
2. `docs/project/video-handoff.md`（任务交接、环境、资产、风险）
3. `docs/deliverables/07-演示视频分镜与操作脚本.md`（分镜、旁白、操作脚本、QC）
4. `docs/research/2026-10-18-AI配音选型.md`（TTS 选型与部署路线）
5. `docs/deliverables/06-演示视频脚本.md`（旧版时长/清单，作为背景）

**硬约束**：
- 成片 ≤180s、1920×1080、30fps、H.264/AAC；界面必须是 v1.0.1（状态栏 1.0.1）。
- 不得出现学号、真实口令、API Key、个人路径（演示数据统一 `/home/demo`）。
- 不改 `backend/`、`client/src/` 的功能代码；录制脚本、字幕生成器、合成脚本放 `scripts/video/` 或 `client/e2e/record-demo/`。
- 不提交视频、音频、模型权重；`course/` 已被 gitignore。
- 上传 B 站/学校平台由用户本人完成，你只交付本地文件与 QC 报告。

**第一阶段（先做，做完停下给用户看）**：
1. 向用户确认：GPU 型号/显存、租用方式（本机远程桌面 or SSH）、声音样本格式与时长（≤100s）、是否本人配音之外的授权。
2. 按调研文档选定 TTS 方案（优先 8–16GB 显存、支持中文零样本克隆），给出部署命令；如租用机是 Linux + NVIDIA，用 Docker/conda 均可。
3. 让用户按样本清单提供 WAV（24kHz/16bit/单声道、安静环境、60–100s、覆盖中文数字与英文术语）。
4. 用样本克隆音色，生成 3 句试听（含数字、英文 `Huffman`、`AES-256`），测量语速；把试听文件放 `course/04-视频/voice/`，向用户确认音色/语速后再继续。
5. 产出 `docs/plans/video-phase1.md`：选定模型、版本、显存占用、命令、试听结论、语速参数。

**第二阶段（用户确认 P1 后）**：
1. 按 `07-...` 与 `video-storyboard.json` 固化分镜与旁白；用脚本统计每句字数/时长，确保旁白总时长 ≤80s、每镜不超时。
2. 生成完整旁白 WAV（逐句），`ffprobe` 记录时长；超时优先改文案，其次 TTS 语速 1.05。
3. 写分段录制脚本：每段独立预置 HOME/仓库（参考 `client/e2e/capture-showcase.mjs`），用 Playwright `recordVideo` 录制，鼠标移动用 `hover` + `mouse.move(steps)`，关键状态后停 0.8s。
4. 先彩排一遍，记录每个状态的出现时刻，再正式录；每段落 `course/04-视频/work/seg-XX.webm`。

**第三阶段（合成与 QC）**：
1. 生成 ASS（字幕 + 注解框），按分镜时间轴对齐；ffmpeg 每段 scale/pad 到 1920×1080、30fps、烧 ASS、混音，响度 -16 LUFS。
2. concat 拼接；`ffprobe` 验证时长/分辨率/帧率/音轨；逐段过 `07-...` §9 清单，最后全片通看两遍。
3. 交付：`course/04-视频/DataBackup-demo-1.0.1.mp4`、`docs/plans/video-qc.md`（时长、参数、逐段结论、遗留问题）、可复现命令。
4. 把视频文件用 `present` 交给用户看；上传由用户完成。

**工作方式**：每阶段结束给用户一个短报告并停下等确认；不要一口气做完三个阶段。遇到 GPU/模型下载/网络问题，先报告再换方案。ffmpeg 若需下载，只下载便携版到 `course/04-视频/tools/`，不要装系统软件。
