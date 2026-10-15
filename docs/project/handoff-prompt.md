# 交接 prompt（复制到新会话使用）

```
你是本仓库的开发 agent。仓库根：C:\Users\Qoobeewang\Desktop\软件工程实验
GitHub：Piaoxuemoli/software-class-qoobeewang，主干 master（已开分支保护：走 PR + CI 必过）。

开工前必读（按顺序）：
AGENTS.md → docs/project/overview.md → docs/project/course-requirements.md
→ docs/roadmap.md（重点 §1 并行设计、§1.4 难度分级）→ docs/architecture/overview.md
→ docs/architecture/cli-contract.md → docs/conventions/*.md。
做客户端任务再读 docs/client/ui-design.md 和 docs/client/interaction.md。

当前状态：M0 基建全部完成，GitHub CI 全绿。下一步是第 0 波三个可并行任务：
- M1-3 接口冻结（🔴 强模型，最先做，其余任务都依赖它）
- M1-1 需求分析说明书初稿（🟢）
- C0 客户端底座（🟡）

工作方式：
1. 每个任务用独立 worktree：
   git worktree add ../databackup-wt/<任务ID> -b <type>/<任务ID>-<slug> origin/master
2. 只改该任务"触碰范围"内的文件；要动 cli-contract / fsmeta 公共类型 / engine 时先停下上报。
3. 验收：bash scripts/docker.sh check（核心改动用 check-all）；在 PR 里贴逐条验证结果。
4. 提交用 Conventional Commits，footer 带 Refs: <任务ID>；完成后只改 roadmap.md 中自己那行状态。
5. 硬约束：后端生产代码只用 Go 标准库（禁引 archive/*、compress/*、crypto/* 现成实现和任何第三方包，
   scripts/check-imports.sh 会强制检查）；口令一律走 stdin；Electron 安全三件套
   （contextIsolation/sandbox/nodeIntegration=false）不得修改；course/ 与个人身份信息永不入库；
   PDF 只在 M9-2 一次性生成；CHANGELOG [Unreleased] 同步更新。

任务：请先做 M1-3（接口冻结）。产出契约 PR 合入后，再按 roadmap §1.4 难度分级
把第 1 波任务分配给相应强度的模型并行开发。
```
