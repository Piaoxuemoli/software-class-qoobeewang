# 剩余人工任务清单（提交前必读）

> 最近更新：**2026-10-18（演示视频完成后）**。v1.0.1 已发布（v1.0.0 为 10-17 首版）。本日 agent 重做了客户端截图与答辩 PPT、修复了发行版界面缺陷、重订截图基线规则、重建 Windows 便携包、插入 AI 工具截图并重出全部 PDF，并**完成演示视频 P1~P3（成片 2:57.97，QC 全绿）**、整理最终提交包（见 §3 与附 A）。
> 原则不变：仓库内不出现学号与凭据；带真实身份的产物只进本地 `course/`（已 gitignore）。
> 用法：按 §1 的顺序做；每完成一项，在 §2 打勾并把 `docs/roadmap.md` 对应行改为 ✅。

## 1. 建议顺序（按截止时间与依赖）

```
H2 证书（24h 审核，最先启动）──┐
H6 演示视频 ──────────────────┼─► M9-6 提交包（11-13 前）─► 11-16 答辩
H12 提交版 PDF（H7 截图已入正文）┘        H8 彩排（11-16 前，用新版 PPT）
H11 已发 v1.0.1 ──► H6 直接录 v1.0.1 的界面
H3 看板截图（每周）/ H5 StarUML（不晚于 11-13）/ H9 签到（每周）
```

## 2. 状态总表

| 编号 | 事项 | 状态 | 截止 | 依赖 |
|---|---|---|---|---|
| H2 | 阿里云 AI 训练证书 | ⬜ | 立即（审核约 24h） | — |
| H6 | 录制并上传演示视频 | 🟨 成片 ✅ 10-18 / 上传 ⬜ | 越早越好 | 已定用 v1.0.1 |
| H7 | AI 工具使用截图 | ✅ 10-18 | — | 已入报告 §7 |
| H11 | 发布 v1.0.1 | ✅ 10-18 | — | 已推 tag 并上传 Windows 便携包 |
| H12 | 提交版 PDF（真实学号） | ✅ 10-18 | 11-12 | 已生成到 course/07-提交物 |
| H3 | 项目看板截图 | ✅ 10-18 | 每周 | 视图由 roadmap 生成 |
| H5 | UML 图复核 | ✅ 10-18 | 11-13 | 未装 StarUML，改用 PlantUML 重绘导出 |
| H8 | 通读代码文档 + 答辩彩排 | ⬜ | 11-16 前 | H12 |
| H9 | 每次课签到 | ⬜ | 每周 | — |
| M9-6 | 组装两个提交包 | 🟨 包已整理 / 待填链接与发送 | 11-13 | H6 上传、本人操作 |

## 3. 逐项说明

### H2 · 阿里云 AI 训练证书（10 分，立即启动）

- **为什么**：课程「AI 辅助开发」要求训练营证书，审核约 24 小时，是唯一有等待周期的待办。
- **步骤**：登录课程课件 4 给出的入口 → 完成训练 → 提交审核 → 保存证书截图。
- **产物**：`course/05-证书/`（本地）；证书在报告中单独附页提交，不占用正文图。
- **完成标准**：证书截图落盘，roadmap H2 行改 ✅。

### H6 · 录制演示视频并上传（成片已完成，仅剩上传）

> **2026-10-18 完成（agent）**：成片 `course/04-视频/DataBackup-demo-1.0.1.mp4` —— 2:57.97 / 1920×1080 / 30fps / H.264+AAC / −16.5 LUFS，QC 逐项通过（`docs/plans/video-qc.md`）。制作链路：IndexTTS-2.5 零样本配音（AISHELL-1 开源男声，Apache-2.0）+ Playwright 分段录制真实 v1.0.1 客户端/后端 + ASS 字幕注解 + ffmpeg 合成。素材与工具链见 `course/04-视频/`、`scripts/video/`、`client/e2e/record-demo/`。
> **仍需本人**：上传到 B 站/学校平台，把 URL 写进 `course/04-视频/链接.txt`，并填入 `course/07-提交物/最终提交包/02-邮箱包/演示视频链接.txt`。

- **为什么**：提交物 #4（演示 20 分）需要 ≤3 分钟视频链接。
- **执行稿**：分镜、旁白、字幕、操作脚本与 QC 见 **[07-演示视频分镜与操作脚本](../deliverables/07-演示视频分镜与操作脚本.md)**；配音选型见 **[AI 配音选型](../research/2026-10-18-AI配音选型.md)**；新对话入口见 **[video-handoff](../project/video-handoff.md)** 与 [video-prompt](../project/video-prompt.md)。
- **录制路线**：Playwright 分段录制真实客户端（v1.0.1）+ AI 声音克隆旁白 + ffmpeg 合成；不再依赖 WSLg/OBS。每段独立预置数据，逐段过 QC 再拼接。
- **要点**：
  1. 演示专用口令 `demo-2026`；错误口令只输 1 次；
  2. 成片 ≤180s、1920×1080、30fps；片尾署名"组长：王科瑾"；
  3. 旁白语速 4.2–4.6 汉字/秒；字幕每行 ≤18 字。
- **版本**：用 v1.0.1（已发布）的界面与产物录制。
- **上传**：B 站或学校指定平台，公开链接记入 `course/04-视频/链接.txt`（由本人完成）。

### H7 · AI 工具使用截图（✅ 2026-10-18 完成）

官方模板第七章要求"使用方法截图与说明"。本人提供了 DSH / Cursor / Devin / ZCode 四张会话截图，agent 已完成脱敏与排版：

- 隐去 DSH 左下角的账户余额/消费区；裁掉 ZCode 左侧与项目无关的项目列表；其余保持原样。
- 处理后的图片在 `docs/deliverables/assets/ai-usage/`（4 张，已入库），原图留在本地 `course/03-截图/ai-usage/`。
- `04-实验报告-后置章节.md` §7 已更新工具表并插入 图 7-1 ~ 7-4；生成的 `docs/pdf/小组实验报告.pdf` 已包含这些图。
- 阿里云训练证书（H2）单独提交，不再作为正文占位图。

### H11 · 发布 v1.0.1（✅ 2026-10-18）

- **背景**：v1.0.0 的发行产物里，备份详情 Sheet 与口令对话框的样式是坏的——Tailwind v4 只扫描了 `src/renderer`，`src/components/ui` 里独占的工具类没有被生成，结果 Sheet 掉进文档流、对话框位置错乱。
- **已做**：版本号按 `pitfalls.md` §2.6 的连锁清单升到 1.0.1（version.go、CLI/e2e 断言、package.json/lock、fakeApi、App.test），重新生成基线、README 截图、PPT 与全部 PDF；推 `v1.0.1` tag 后由 CI 产出新资产，Windows 便携包重新构建并作为附加资产上传。
- **演示用哪版**：用 v1.0.1 的 Release 产物（或本机 `make package`），不要再用 v1.0.0。
- **完成标准**：Release 页出现 `v1.0.1` 且资产齐全。

### H12 · 提交版 PDF / PPT（✅ 2026-10-18）

- **已生成**：按本人学号（配置在本地 `course/profile.yaml`，**不入库**）生成 6 份 PDF 到 `course/07-提交物/`（5 份文档 + 带学号封面的答辩 PPT），已核对 PDF 文本中包含该学号。下面命令用于以后需要重跑时。
- **步骤**（仓库根，Git Bash）：

  ```bash
  # 1) 本地 profile：真实学号 + 提交日期（course/ 已 gitignore）
  cp docs/deliverables/profile.example.yaml course/profile.yaml

  # 2) 六份文档 PDF → course/07-提交物/
  DOCS_PROFILE=course/profile.yaml DOCS_OUT=course/07-提交物 bash scripts/docker.sh docs

  # 3) 答辩 PPT 的 PDF：直接复用仓库版（不在 docs 清单里，避免重跑整条文档链）
  cp docs/pdf/答辩PPT.pdf course/07-提交物/答辩PPT.pdf
  #    需要改封面时：在 PowerPoint 打开 docs/ppt/答辩PPT.pptx 改完另存为 PDF
  ```

- **验收**：`course/07-提交物/` 出现 6 份 PDF；`git status` 无新增文件（防泄密校验）。
- **仓库版 PDF**：10-18 已重出全部 6 份（含 AI 截图的 `小组实验报告.pdf` 与新 PPT 的 `答辩PPT.pdf`），提交版只需换学号重跑。

### H3 · 项目看板截图（✅ 2026-10-18）

本人不需要再装 GitHub Projects：agent 以 `docs/roadmap.md` 的任务状态生成了三张看板快照
（10-16 / 10-17 / 10-18），源与渲染命令可复现：

- 仓库内：`docs/images/project/board-2026-10-*.png` + `board.html`（HTML 读 `?d=<date>` 渲染对应状态）。
- 本地提交：`course/02-看板/`（同三张 + board.html）。
- 图片页脚注明"视图由 docs/roadmap.md 生成，用于课程过程证据"，不冒充 GitHub Projects 截图。

### H5 · UML 图复核（✅ 2026-10-18）

按本人指示不安装 StarUML。以同一模型用 PlantUML 重绘并导出图集：

- 10 张 UML 图（用例、类、构件 ×3、顺序 ×4、状态）与说明见 `docs/deliverables/diagrams/README.md`。
- 提交用图集：`course/05-UML/UML图集.pdf`（A4，6 页）+ 全部 PNG/PUML 源文件。
- 仓库内 PlantUML 源与 PNG 保持唯一事实来源；图集由 `docs/images/uml/atlas.html` 打印生成。

### H8 · 通读代码与文档、答辩彩排（11-16 前）

- **主线**：以新版 `docs/ppt/答辩PPT.pptx`（14 页）过一遍；重点熟悉 P6/P7 的自研实现要点与 P12 的性能数字、已知限制。
- **彩排**：≤8 分钟完整讲述 + 视频演示衔接，至少一次。
- **可能被追问的点**（提前想好答案）：
  - 大目录树 ≥10 万项未实测（测试报告 §6.3 已声明）；
  - 为什么用硬链接发布而不是 rename（防并发覆盖）；
  - 压缩率为何随环境微漂移（mtime/uid 进入压缩流，测试报告有分析）；
  - 为什么 v1.0.0 的界面样式有缺陷、如何发现并修复（Tailwind 扫描范围；本次由截图复核暴露）。

### H9 · 每次课签到（每周）

考勤影响评分公式是否生效，剩余课次全勤。

### M9-6 · 组装两个提交包（11-13 前）

> **2026-10-18（agent）**：两个包已整理到 **`course/07-提交物/最终提交包/`**（`01-平台包/`、`02-邮箱包/`、`03-演示视频/`、`04-文档与素材/`、`05-校验/` + `平台包.zip`/`邮箱包.zip` + `00-README-提交清单.md`），可执行程序直接取自 v1.0.1 Release，源码包由 `git archive` 生成。剩余为本人操作：填视频链接、上传平台、发邮件。

**包 A：实验平台**（`course/07-提交物/最终提交包/01-平台包/`）

- 小组实验报告 PDF（H12 生成的真实学号版 `小组实验报告.pdf`）
- 答辩 PPT 的 PDF（H12 复用的 `答辩PPT.pdf`，14 页新版）
- 按平台要求上传（平台地址见 `course/` 本地记录）。

**包 B：助教邮箱**（`course/07-提交物/邮箱包/`，发送前核对）

- [ ] 源代码 zip（`git archive --format=zip -o 源码.zip master` 或 GitHub Source tarball；**确认不含 `course/`、不含学号**）
- [ ] 可执行程序：Release 页的 AppImage、deb、CLI tar.gz（若发 v1.0.1，改用 v1.0.1 的资产）
- [ ] 构建脚本：源码包内已含（Makefile / install-deps.sh / docker.sh，README 有说明）
- [ ] 演示视频链接（H6 的 URL，写进邮件正文）
- [ ] sha256sums.txt（Release 页有，可选附上）

发送后 roadmap M9-6 行改 ✅，项目收尾。

---

## 附 A：本次（2026-10-18）已完成、无需本人再管

| 项 | 产物 |
|---|---|
| 客户端真实截图 7 张（2× 像素，真实后端） | `docs/images/client/01~07-*.png`；重拍脚本 `client/e2e/capture-showcase.mjs` |
| README 截图展示区（7 张图，含进行中与设置页） | `README.md` §界面 |
| 答辩 PPT 重做（14 页，去 AI 腔、图文结合） | `docs/ppt/答辩PPT.pptx`；生成脚本 `scripts/docs/pptx-build.mjs`；源文 `docs/deliverables/05-答辩PPT.md` |
| 界面缺陷修复（Sheet/对话框/胶囊）与全部 PDF 重出 | `docs/pdf/` 6 份；`client/src/renderer/src/` |
| 截图基线改为自动审计 + 结构契约用例 | `docs/client/visual-regression.md`、`client/e2e/layout.spec.ts`、`scripts/visual-audit.mjs`、`client/e2e/baselines.json` |
| AI 工具截图脱敏与插入 §7 | `docs/deliverables/assets/ai-usage/`、`docs/pdf/小组实验报告.pdf` |
| Windows 便携包重建与验证 | `client/release/DataBackup 1.0.1.exe`（未入库）、Release 资产 `DataBackup-1.0.1-portable.exe` |
| v1.0.1 发布 | tag `v1.0.1`；AppImage / deb / CLI / 源码 / 6 份 PDF / sha256 + Windows 便携包 |
| 提交版 PDF（真实学号，配置在本地 `course/profile.yaml`） | `course/07-提交物/` 6 份（含带学号封面的答辩 PPT） |
| **演示视频（H6 成片）** | `course/04-视频/DataBackup-demo-1.0.1.mp4`（2:57.97，QC 全绿）；制作文档 `docs/plans/video-{phase1,phase2,qc}.md`；工具链 `scripts/video/`、`client/e2e/record-demo/` |
| **最终提交包（M9-6）** | `course/07-提交物/最终提交包/`（平台包/邮箱包/演示视频/文档素材/校验 + 两个 zip） |
| 项目看板快照（H3） | `docs/images/project/` + `course/02-看板/`（3 个日期） |
| UML 图集（H5） | `course/05-UML/UML图集.pdf` + 10 张 PNG/PUML |
| 进度与踩坑记录 | `docs/project/schedule.md`、`docs/project/pitfalls.md` |

## 附 B：可选项（如需启用，直接让 agent 做）

- **C5 UI 打磨**（🟡）：提案在 `docs/client/ui-polish.md`；改动会连锁截图基线与文档，需本人拍板。
- **C4b Windows 便携包**（已完成）：已随 v1.0.1 Release 提供 `DataBackup-1.0.1-portable.exe`；使用说明见 `docs/client/windows-portable.md`。
