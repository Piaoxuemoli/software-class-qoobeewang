# 剩余人工任务清单（提交前必读）

> 最近更新：**2026-10-18**。v1.0.0 已于 10-17 发布。本日 agent 重做了客户端截图与答辩 PPT、修复了发行版里可见的界面缺陷、重订截图基线规则、重建 Windows 便携包、插入 AI 工具截图并重出全部 PDF（见 §3 与附 A）。
> 原则不变：仓库内不出现学号与凭据；带真实身份的产物只进本地 `course/`（已 gitignore）。
> 用法：按 §1 的顺序做；每完成一项，在 §2 打勾并把 `docs/roadmap.md` 对应行改为 ✅。

## 1. 建议顺序（按截止时间与依赖）

```
H2 证书（24h 审核，最先启动）──┐
H6 演示视频 ──────────────────┼─► M9-6 提交包（11-13 前）─► 11-16 答辩
H12 提交版 PDF（H7 截图已入正文）┘        H8 彩排（11-16 前，用新版 PPT）
H11 发不发 v1.0.1（建议 11 月上旬定）──► 决定 H6 录哪一版界面
H3 看板截图（每周）/ H5 StarUML（不晚于 11-13）/ H9 签到（每周）
```

## 2. 状态总表

| 编号 | 事项 | 状态 | 截止 | 依赖 |
|---|---|---|---|---|
| H2 | 阿里云 AI 训练证书 | ⬜ | 立即（审核约 24h） | — |
| H6 | 录制并上传演示视频 | ⬜ | 越早越好 | H11 的取舍 |
| H7 | AI 工具使用截图 | ✅ 10-18 | — | 已入报告 §7 |
| H11 | **决定是否发布 v1.0.1** | ⬜ | 11 月上旬 | 本人授权 push tag |
| H12 | 提交版 PDF（真实学号） | ⬜ | 11-12 | 本人学号 |
| H3 | 项目看板截图 | ⬜ | 每周 | — |
| H5 | UML 图在 StarUML 复核 | ⬜ | 11-13 | — |
| H8 | 通读代码文档 + 答辩彩排 | ⬜ | 11-16 前 | H12 |
| H9 | 每次课签到 | ⬜ | 每周 | — |
| M9-6 | 组装两个提交包 | ⬜ | 11-13 | H6、H12 |

## 3. 逐项说明

### H2 · 阿里云 AI 训练证书（10 分，立即启动）

- **为什么**：课程「AI 辅助开发」要求训练营证书，审核约 24 小时，是唯一有等待周期的待办。
- **步骤**：登录课程课件 4 给出的入口 → 完成训练 → 提交审核 → 保存证书截图。
- **产物**：`course/05-证书/`（本地）；证书在报告中单独附页提交，不占用正文图。
- **完成标准**：证书截图落盘，roadmap H2 行改 ✅。

### H6 · 录制演示视频并上传（阻塞 M9-6）

- **为什么**：提交物 #4（演示 20 分）需要 ≤3 分钟视频链接。
- **怎么录**：严格按 [06-演示视频脚本](../deliverables/06-演示视频脚本.md) 的 6 段时长分配与录制清单执行。要点：
  1. 在 WSL 里按 [wsl-guide](../client/wsl-guide.md) 启动客户端（agent 已实测；`libfuse2t64` / `libsecret` 问题文档里有对策）；
  2. 用 Windows 侧 OBS 或 Win+G 直接录 WSLg 窗口，≥1080p；
  3. 演示专用口令；错误口令只输 1 次；
  4. 片尾字幕：项目名、组长：王科瑾。
- **录哪一版界面**：若 H11 决定发 v1.0.1，就用 v1.0.1 的产物录制（新版的详情 Sheet 与口令对话框样式是修好的）；不发则用当前 master 的 `make package` 产物，并在片头说明"演示版本为 master 构建"。
- **上传**：B 站或学校指定平台，公开链接记入 `course/04-视频/链接.txt`。

### H7 · AI 工具使用截图（✅ 2026-10-18 完成）

官方模板第七章要求"使用方法截图与说明"。本人提供了 DSH / Cursor / Devin / ZCode 四张会话截图，agent 已完成脱敏与排版：

- 隐去 DSH 左下角的账户余额/消费区；裁掉 ZCode 左侧与项目无关的项目列表；其余保持原样。
- 处理后的图片在 `docs/deliverables/assets/ai-usage/`（4 张，已入库），原图留在本地 `course/03-截图/ai-usage/`。
- `04-实验报告-后置章节.md` §7 已更新工具表并插入 图 7-1 ~ 7-4；生成的 `docs/pdf/小组实验报告.pdf` 已包含这些图。
- 阿里云训练证书（H2）单独提交，不再作为正文占位图。

### H11 · 决定是否发布 v1.0.1（建议 11 月上旬定）

- **背景**：v1.0.0 的发行产物里，备份详情 Sheet 与口令对话框的样式是坏的——Tailwind v4 只扫描了 `src/renderer`，`src/components/ui` 里独占的工具类没有被生成，结果 Sheet 掉进文档流、对话框位置错乱。已发布基线截图里也能看到这个现象。
- **现状**：master 已修复（`styles.css` 增加 `@source '../../';`，补 Sheet 内边距、修筛选胶囊清除按钮），截图基线规则已重订；READM/PPT 截图与 Windows 便携包都已按修复后的版本重建（portable 产物在 `client/release/`，未入库）。
- **要决定的事**：课堂演示与提交包用哪一版产物。
  - **发 v1.0.1（建议）**：版本号 bump（按 `pitfalls.md` §2.6 的连锁清单）→ 合入 → 由本人推 `v1.0.1` tag，CI 重新产出 AppImage / deb / CLI / PDF 资产，并把重建的 Windows 便携包作为可选资产附上。需要本人授权与执行 `git push origin v1.0.1`。
  - **不发**：用 master 本地 `make package` 的产物演示；Release 页保持 v1.0.0，答辩时说明差异。
- **完成标准**：在 §2 写明结论；若发版，roadmap H11 行改 ✅。

### H12 · 提交版 PDF / PPT 重出（新增）

- **为什么**：提交版报告需要填真实学号；AI 截图已入正文，六份 PDF 的内容与 10-18 版一致。
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

### H3 · 项目看板截图（每周一次）

GitHub 仓库 → Projects（或现有看板）→ 截图当前视图（含日期），每周一张存 `course/02-看板/`。
agent 无 `project` scope；需要 agent 协助维护看板时，先由本人执行 `gh auth refresh -s project`。

### H5 · UML 图在 StarUML 中复核（不晚于 11-13）

对照 `docs/deliverables/diagrams/*.puml`（用例图、类图、4 张顺序图、状态图、构件图、甘特图）在 StarUML 重绘或导入复核，必要时导出图片替换 `course/` 内副本。仓库内 PlantUML 版保持不动。

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

**包 A：实验平台**（`course/07-提交物/平台包/`）

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
| 截图基线标准重订 + 结构契约用例 | `docs/client/visual-regression.md`、`client/e2e/layout.spec.ts`、`client/e2e/baselines.md` |
| AI 工具截图脱敏与插入 §7 | `docs/deliverables/assets/ai-usage/`、`docs/pdf/小组实验报告.pdf` |
| Windows 便携包重建与验证 | `client/release/DataBackup 1.0.0.exe`（未入库）、`docs/client/windows-portable.md` |
| 进度与踩坑记录 | `docs/project/schedule.md`、`docs/project/pitfalls.md` |

## 附 B：可选项（如需启用，直接让 agent 做）

- **C5 UI 打磨**（🟡）：提案在 `docs/client/ui-polish.md`；改动会连锁截图基线与文档，需本人拍板。
- **C4b Windows 便携包**（已完成）：`docs/client/windows-portable.md`；如需随 v1.0.1 重发，重新打包即可。
