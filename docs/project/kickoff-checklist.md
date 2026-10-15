# 开工前检查清单

> 进入 W0（M1-3 / M1-1 / C0）之前需要完成或确认的事项。完成后打勾并注明日期。

## 1. 需要本人完成

| # | 事项 | 为什么 | 状态 |
|---|---|---|---|
| 1 | GitHub 设置：默认分支改为 `master`，删除旧的 `main`，为 `master` 开启分支保护（要求 PR、要求 CI 通过、禁止 force push） | 多 agent 并行时防止误推主干（roadmap H4） | ⬜ |
| 2 | 联系李忻洋老师：考勤补救、一人成组、实验平台地址与助教邮箱、扩展分以哪版为准、Electron 界面是否影响 GUI 分 | 考勤决定评分公式是否生效（H1，最高优先级） | ⬜ |
| 3 | 确认实际上课时间，校正 `schedule.md` 中按"每周一"推算的日期 | 所有截止日期以此为基准 | ⬜ |
| 4 | 阿里云 AI 训练证书（本人自行完成） | 完成分 10 分，审核约 24h（H2） | ⬜ |
| 5 | （可选）`gh auth login`，便于 agent 把 roadmap 同步为 GitHub Issues / Projects 看板 | 看板截图是课程"项目管理工具"的过程证据（H3） | ⬜ |
| 6 | （C4 之前即可）WSL2 Ubuntu 中运行 `bash scripts/install-deps.sh`（需要输入 sudo 密码） | 演示环境（H10） | ⬜ |

## 2. agent 侧已完成 / 待完成

| # | 事项 | 状态 |
|---|---|---|
| 1 | 仓库重建、课程资料本地化、旧历史归档 | ✅ |
| 2 | Go 后端 + Electron 客户端骨架、Docker 开发镜像、Makefile、`scripts/docker.sh` | ✅ |
| 3 | CI / Release 工作流；GitHub 首次 CI 后端 lint 失败（runner 自带 Go 抢占 PATH）已修复，待推送后复验 | 🟨 |
| 4 | 调研报告与方案修订（`docs/research/`） | ✅ |
| 5 | 并行开发设计：波次、泳道、冲突热点（`docs/roadmap.md` §1） | ✅ |
| 6 | 客户端视觉规范与交互时序（`docs/client/`） | ✅ |
| 7 | CLI 协议补充 dry-run、总量进度、取消语义 | ✅ |
| 8 | 分区 agent 指令 `backend/AGENTS.md`、`client/AGENTS.md` | ✅ |
| 9 | M0-5：`docker.sh` 卷按 worktree 区分（多 agent 并行的前提，已验证两个 worktree 并发 `check` 通过） | ✅ |
| 10 | PDF 策略改为项目最后一次性生成，删除仓库中的骨架 PDF | ✅ |

## 3. 已确认的项目事实

- 小组：**组长王科瑾，单人组，无组员**。学号不进仓库，只在本地生成提交版文档时填入。
- 项目使用 AI 辅助开发（编码 agent）；实验报告第七章所需的 AI 工具截图由本人在项目最后统一截取，agent 不操作任何 AI 平台。
- PDF 只在项目完成后（M9-2）一次性生成，开发过程中只维护 Markdown。
