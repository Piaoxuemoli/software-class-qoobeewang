# 1028 课堂口径修正与文档重出（临时分支）

> 状态：**二轮修正完成（2026-11-09）**——图源与文档源已对照代码逐项核对并修正，`docs/**/diagrams/*.png` 与 `docs/pdf/*.pdf`×5 已用 `bash scripts/docker.sh docs` 重出（占位学号版）。**答辩 PPT、演示视频与慕课包仍未重做**。分支 `docs/1028-course-corrections`。
> 一轮内容是组长在 master 工作区的未提交改动（2026-10-28 课堂投影口径修正，27 个文件），2026-11-04 由 agent 原样迁移提交；master 工作区已恢复干净。

## 1. 一轮改动（2026-11-04 原样迁移）

| 区域 | 文件 |
|---|---|
| 提交口径 | `docs/project/course-requirements.md`（答辩与材料都在 11-18；PPT 3 分钟；视频 ≤2 分钟的文件，四项交成电慕课） |
| 交付文档源 | `docs/deliverables/01-需求分析说明书.md`、`02-系统设计文档.md`、`05-答辩PPT.md`、`06-演示视频脚本.md`、`07-演示视频分镜与操作脚本.md` |
| UML 源 | `docs/deliverables/diagrams/*.puml`（use-cases 口令与 dry-run 改 extend、顺序图进度经 CLI stdout、删除改长按、状态图补 AwaitingPassword 等）、`docs/project/diagrams/schedule-gantt.puml`、`docs/images/uml/atlas.html` |
| 项目文档 | `docs/client/interaction.md`、`docs/project/{overview,schedule,handover,human-tasks}.md`、`docs/roadmap.md`、`CHANGELOG.md` |
| 脚本 | `scripts/docs/pptx-build.mjs` |

## 2. 二轮修正（2026-11-09，agent 对照 master 代码复核）

| 文件 | 改动 | 依据 |
|---|---|---|
| `docs/project/diagrams/schedule-gantt.puml` | 改回按日刻度（`printscale daily zoom 2`）并去掉 `saturday/sunday are closed`：closed 会把落在周末的 10-17 / 10-18 里程碑挤到下一个工作日；weekly 刻度只剩周号（43~47），读不出具体日期 | 仓库记录：W0~W3 全部 10-16 合入；v1.0.0 = 10-17（周六）14:10；v1.0.1 = 10-18（周日）12:33 |
| `state-backup-job.puml` + `interaction.md` §3 + 设计文档 §4.2 + `atlas.html` 图 10 | Running 的三条转移改为「error（非口令类）/ 非零退出 → Failed」「口令类错误（E_AUTH / E_PASSWORD_REQUIRED，未满 3 次）→ AwaitingPassword」「连续 3 次口令错误（E_AUTH）→ Ready」 | `client/src/renderer/src/stores/job.ts`：E_AUTH 与 E_PASSWORD_REQUIRED 都回 awaitingPassword，只有 E_AUTH 累计 authFailures |
| `01-需求分析说明书.md` UC-05 事件流 | 「客户端周期性调用 list」→「启动时预取，备份完成 / 删除 / 修改仓库路径后刷新」 | `stores/backups.ts` 无轮询，靠事件刷新 |
| `01-需求分析说明书.md` UC-04 小节 | 「加密时 extend UC-01」→「加密备份时扩展 UC-01」（正文行文不用英文关键字） | 行文口径统一 |
| `component-client.puml` | 补 `main/ipc/`（zod 校验处理器）构件与连线：index 注册处理器、preload 的 invoke 落到 ipc、ipc 调 backend/jobs | `client/src/main/ipc/handlers.ts` 是 preload 调用的实际落点 |

二轮后已重出并验证：PNG×12 全部刷新；《项目进度计划》第 2 页与《需求分析说明书》§5.2（第 22 页）均嵌入新甘特图（1488×301，258ppi）。

## 3. 复核结论（未盲改，留给后续决策）

- **类图 A4 可读性（D10）**：加大间距后渲染为 3883×2915px，按 A4 版心宽缩放字号只剩约 1/5.5，基数在纸上认不出。间距调整解决不了；要拆图（原语层 / 编排入口层各一张）或简化类成员后重画，属结构性改模。
- **PPT 源里的过期状态句**：`05-答辩PPT.md` P7/P8 与 `pptx-build.mjs` P7 仍写着「PPT 还没重出、视频 2:58、包还是两包」（11-04 时点状态）。重跑 `build-ppt.sh` 前必须按届时事实改写，否则会印进提交版。
- **课次星期核对**：10-19 / 10-26 / 11-02 / 11-09 均为周一，与 schedule.md「每周一上课」一致；11-18 是周三，文档按投影原文写「上午上课时」，不标星期。
- D13 / D14 / D15（视频重剪、慕课包组装、7 分钟演示小数据集）不在本分支范围。

## 4. 待办

1. 决策类图拆分方案后改 `class-backend.puml`，再重渲染。
2. 按届时事实改 PPT 源的状态句，重跑 `scripts/docs/build-ppt.sh`（勿在状态句未改时导出）。
3. 视频剪到 ≤2 分钟（D13）；组装一个慕课包（D14）；准备 7 分钟演示小数据集（D15）。
4. 全部就绪后按正常流程开 PR 合回 master（squash 显式传 `--subject/--body`）。
5. 提交版 PDF（真实学号）：`DOCS_PROFILE=course/profile.yaml DOCS_OUT=<course>/07-提交物 bash scripts/docker.sh docs`，在含 `course/` 的机器上执行；完成后更新课程仓库 `07-提交物` 的清单与校验。
6. 图集：用 `docs/images/uml/atlas.html` 重新打印 `05-UML/UML图集.pdf`。

## 5. 并行分支提醒

`feat/v110-verify-multialgo`（备份验证 + ChaCha20）也在等 Docker 设备补跑 `bash scripts/docker.sh check` / `check-all`；两分支互不冲突，先后合并均可。导出流程见 `docs/plans/V110-verify-multialgo.md` §6。
