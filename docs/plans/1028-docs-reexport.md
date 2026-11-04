# 1028 课堂口径修正与文档重出（临时分支）

> 状态：源已提交，**导出待有 Docker 的设备执行**。分支 `docs/1028-course-corrections`。
> 本分支内容是组长在 master 工作区的未提交改动（2026-10-28 课堂投影口径修正，27 个文件），
> 2026-11-04 由 agent 原样迁移提交以便跨设备处理；master 工作区已恢复干净。

## 1. 本批改动内容（原样迁移，未做任何加工）

| 区域 | 文件 |
|---|---|
| 提交口径 | `docs/project/course-requirements.md`（答辩与材料都在 11-18；PPT 3 分钟；视频 ≤2 分钟的文件，四项交成电慕课） |
| 交付文档源 | `docs/deliverables/01-需求分析说明书.md`、`02-系统设计文档.md`、`05-答辩PPT.md`、`06-演示视频脚本.md`、`07-演示视频分镜与操作脚本.md` |
| UML 源 | `docs/deliverables/diagrams/*.puml`（use-cases 口令与 dry-run 改 extend、顺序图进度经 CLI stdout、删除改长按、状态图补 AwaitingPassword 等）、`docs/project/diagrams/schedule-gantt.puml`（改为仓库记录的完成日）、`docs/images/uml/atlas.html` |
| 项目文档 | `docs/client/interaction.md`、`docs/project/{overview,schedule,handover,human-tasks}.md`、`docs/roadmap.md`、`CHANGELOG.md` |
| 脚本 | `scripts/docs/pptx-build.mjs` |

CHANGELOG `[Unreleased]` 已有一条对应的说明（"PDF、PPT 和 UML 图集还没按新源重出"）。

## 2. 待办：文档源重出（在有 Docker 的设备上）

1. checkout 本分支，在软件仓库跑：
   ```
   bash scripts/docker.sh docs
   ```
   这条会把 PlantUML 重画成 PNG，并重新生成这几份 PDF：**需求分析说明书、系统设计文档、小组实验报告、项目进度计划**。软件测试报告也会跟着出一遍，正文没有改。
2. **这条命令不会动答辩 PPT，也不会动视频。** 答辩 PPT 是另一个脚本 `scripts/docs/build-ppt.sh`，**不要跑**——PPT 和视频保持 2026-10-18 那一版不动。
3. 导出完成后，把新的 PDF 拷回课程仓库的 `07-提交物/`（本地目录，不在 git 里）。
4. `05-UML/` 里的 PNG 和《UML 图集.pdf》如果还要交：用 `docs/images/uml/atlas.html` 打印一版；图集不在上面那条命令里。
5. 全部完成后按正常流程开 PR 合回 master（squash 时显式传 `--subject/--body`，见 git-workflow §2）。

## 3. 并行分支提醒

`feat/v110-verify-multialgo`（备份验证 + ChaCha20 多算法加密）也在等 Docker 设备补跑
`bash scripts/docker.sh check` / `check-all`，两个分支互不冲突（本分支只动文档与 PPT 构建脚本，
不动 Go/TS 代码），先后合并均可。导出流程见 `docs/plans/V110-verify-multialgo.md` §6。
