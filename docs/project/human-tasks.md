# 剩余人工任务清单（提交前必读）

> v1.0.0 已发布（tag 由本人授权 agent 于 2026-10-17 推送，Release 产物齐全）。**agent 能做的已全部完成**；以下为必须组长本人完成的事项，按截止时间排序。每完成一项，把 `docs/roadmap.md` 对应行的状态改为 ✅（或在本文件打勾并注明日期）。
> 原则：仓库内不出现学号与凭据；带真实身份的产物只进本地 `course/`（已 gitignore）。

## 总览与依赖关系

```
H2 证书（24h 审核，最先启动）──┐
H6 演示视频 ──────────────────┼─► M9-6 提交包（11-13 前）─► 11-16 答辩
H7 AI 截图 ──► 提交版报告 PDF ─┘         H8 彩排（11-16 前）
H3 看板截图（每周）/ H5 StarUML（不晚于 11-13）/ H9 签到（每周）
```

---

## 1. H2 · 阿里云 AI 训练证书（10 分，立即启动）

- **为什么**：课程「AI 辅助开发」要求阿里云训练营证书，审核约 24 小时，是所有待办里唯一有等待周期的。
- **步骤**：登录阿里云百炼/训练营（课程课件 4 的入口）→ 完成训练 → 提交审核 → 取得证书截图。
- **产物**：证书截图存 `course/05-证书/`（本地）；其中一张后续用于 H7 截图 3（见下）。
- **完成后**：roadmap H2 行改 ✅。

## 2. H6 · 录制演示视频并上传（阻塞 M9-6，越早越好）

- **为什么**：提交物 #4（演示 20 分）需要 ≤3 分钟视频链接。
- **怎么录**：完全按 **[docs/deliverables/06-演示视频脚本.md](../deliverables/06-演示视频脚本.md)** 执行——里面有 6 段时长分配、逐项录制清单（打勾用）和录不好的 CLI 兜底方案。要点：
  1. WSL 里按 [docs/client/wsl-guide.md](../client/wsl-guide.md) 确认客户端能启动（agent 已实测通过；如遇 `libfuse2t64`/`libsecret` 问题文档里有对策）；
  2. 用 Windows 侧 OBS / Win+G 直接录 WSLg 窗口，≥1080p；
  3. 演示专用口令；错误口令只输 1 次；
  4. 片尾字幕：项目名、组长：王科瑾。
- **上传**：B 站或学校指定平台，取得公开链接，记入 `course/04-视频/链接.txt`（本地）。
- **完成后**：roadmap H6 行改 ✅，链接留给 M9-6 用。

## 3. H7 · AI 工具使用截图（实验报告第七章素材）

- **为什么**：官方模板第七章要求"使用方法截图与说明"。`docs/deliverables/04-实验报告-后置章节.md` §7 已写好文字，留有 3 个占位标记，截图后插入：
  1. **【H7 截图 1】** AI 编码 agent 执行任务：任一 agent 会话里 worktree + roadmap 任务分配的画面（例如本仓库任一 PR 的 agent 工作截图）；
  2. **【H7 截图 2】** agent 复审报告与门禁输出：`docs/plans/` 下任一 `*-review.md` + `make check`/CI 全绿画面；
  3. **【H7 截图 3】** 阿里云 AI 训练证书（来自 H2）。
- **注意**：截图里不得出现学号/手机号等隐私；agent 会话截图注意不含 API Key。
- **产物**：三张图存 `course/03-截图/`，并替换 04 章 md 中的占位标记（改完后重跑第 4 项的提交版 PDF）。

## 4. 提交版 PDF（真实学号，输出只在本地）

- **为什么**：仓库内 PDF 是占位学号版；正式提交需要真实学号版。
- **步骤**（在仓库根，Git Bash）：

  ```bash
  # 1) 复制模板并填写（course/ 已 gitignore，不会入库）
  mkdir -p course
  cp docs/deliverables/profile.example.yaml course/profile.yaml
  #    编辑 course/profile.yaml：student-id 填真实学号，date 填提交日期

  # 2) 生成到本地提交物目录
  DOCS_PROFILE=course/profile.yaml DOCS_OUT=course/07-提交物 bash scripts/docker.sh docs
  ```

- **验收**：`course/07-提交物/` 出现 6 份 PDF；`git status` 不显示任何新文件（防泄密校验）。
- **时机**：H7 截图插入 04 章之后再跑（否则报告里是占位标记）。

## 5. H3 · 项目看板截图（每周一次）

- **为什么**：课程要求"项目管理工具"过程证据。
- **步骤**：GitHub 仓库 → Projects（或现有看板）→ 截图当前视图（含日期）。每周一张存 `course/02-看板/`。
- **说明**：agent 无 `project` scope，看板维护如需 agent 协助，先 `gh auth refresh -s project`（本人执行）。

## 6. H5 · UML 图在 StarUML 中复核（不晚于 11-13）

- **为什么**：交付文档的图是 PlantUML 源，老师可能要求 StarUML 工程。
- **步骤**：对照 `docs/deliverables/diagrams/*.puml` 在 StarUML 重绘/导入复核（用例图、类图、顺序图、状态图、构件图、甘特图），必要时导出图片替换 `course/` 内副本。仓库内 PlantUML 版保持不动。
- **完成后**：roadmap H5 行改 ✅。

## 7. H8 · 通读代码与文档、答辩彩排（11-16 前）

- **要点**：以 [docs/ppt/答辩PPT.pptx](../ppt/答辩PPT.pptx) 为主线过一遍；重点熟悉 P7/P12 的性能数字与 P6/P8 的自研实现要点（答辩高频问题区）。彩排一次 ≤8 分钟完整讲述 + 视频演示衔接。
- **答辩可能被追问的点**（提前想好答案）：大目录树 ≥10 万项未实测（测试报告 §6.3 已声明）；为什么用硬链接发布而非 rename（防并发覆盖）；输出/压缩率为何随环境微漂移（mtime/uid 进压缩流，测试报告有分析）。

## 8. H9 · 每次课签到（每周，考勤影响评分公式生效）

## 9. M9-6 · 组装两个提交包（11-13 前，依赖 H6 链接）

**包 A：实验平台**（`course/07-提交物/平台包/`）
- 小组实验报告 PDF（第 4 项生成的真实学号版 `小组实验报告.pdf`）
- 答辩 PPT 的 PDF（用真实学号版流水线生成的 `答辩PPT.pdf`；如需带学号封面，可在 PowerPoint 打开 `docs/ppt/答辩PPT.pptx` 补封面后另存）
- 按平台要求上传（平台地址本人已另行取得，见 `course/` 本地记录）。

**包 B：助教邮箱**（`course/07-提交物/邮箱包/`，发送前核对清单）
- [ ] 源代码：仓库 zip（`git archive --format=zip -o 源码.zip master`，或 GitHub 下载 Source tarball；**确认不含 course/、不含学号**）
- [ ] 可执行程序：Release 页下载 `DataBackup-1.0.0.AppImage`、`databackup-client_1.0.0_amd64.deb`、`databackup-1.0.0-linux-x86_64.tar.gz`
- [ ] 构建脚本：源码包内已含（Makefile / install-deps.sh / docker.sh，README 有说明）
- [ ] 演示视频链接（H6 的 URL，写入邮件正文）
- [ ] sha256sums.txt（Release 页有，可选附上）

发送后 roadmap M9-6 行改 ✅，项目收尾。

---

## 附：已完成、无需本人再管的项

| 项 | 状态 |
|---|---|
| v1.0.0 Release（产物 + sha256 + 自动 Release Notes） | ✅ tag 已推，见 Releases 页 |
| 全部交付文档 / PDF / PPT / 视频脚本 | ✅（#37、#38） |
| 测试证据（覆盖率 / race / fuzz / 静态检查 / 性能） | ✅（#36） |
| WSL 演示环境（工具链 + 运行库 + 已装 deb） | ✅ agent 代办（H10） |

## 附：可选项（如需启用，直接让 agent 做）

- **C5 UI 打磨**（🟡）：提案在 `docs/client/ui-polish.md`，冻结后改动会连锁基线与文档，需要本人拍板。
- **C4b Windows 便携包**（加分，复杂度近 🔴）：交叉编译 + fsmeta/engine Windows 变体，周期较长，建议仅在时间富余时启动。
