# AGENTS.md

任何进入本仓库工作的 agent（Devin / Claude Code / Codex / Qoder …）从这里开始。本文是全局规则的唯一来源，`CLAUDE.md` 只做转发。进入子目录工作时还要读该目录的 `AGENTS.md`（`backend/AGENTS.md`、`client/AGENTS.md`），子目录规则是对本文的补充。

## 1. 项目一句话

电子科技大学《软件开发综合实验》课程项目：**数据备份软件 DataBackup**。Linux 下把目录树备份为单个备份文件并原样还原（文件类型、元数据、过滤、自研打包格式、Huffman 压缩、AES 加密），配 Grok 风格的 Electron 桌面客户端。组长王科瑾，单人组；项目使用 AI 辅助开发。详见 `docs/project/overview.md`。

## 2. 开工前必读（按顺序）

| 文档 | 用途 |
|---|---|
| `docs/project/overview.md` | 项目简介、功能清单（FR 编号）、技术选型与运行环境 |
| `docs/project/course-requirements.md` | 课程硬性要求、评分规则、提交物清单（**需求的最终依据**） |
| `docs/roadmap.md` | **并行开发设计**（波次、泳道、冲突热点）与任务清单（ID、依赖、触碰范围、验收标准） |
| `docs/project/schedule.md` | 时间进度与当前状态 |
| `docs/architecture/overview.md` | 架构、包划分、数据流水线、ADR |
| `docs/architecture/cli-contract.md` | 后端 CLI / JSONL 协议（客户端与后端的唯一接口） |
| `docs/client/ui-design.md`、`docs/client/interaction.md` | 客户端视觉规范；进程职责、IPC、状态机与时序 |
| `docs/conventions/*.md` | git、测试、发布、编码、文档规范 |
| `docs/research/2026-10-调研报告.md` | 方案依据（需要理解"为什么这样设计"时读） |

## 3. 硬性约束（违反会直接丢分，必须遵守）

1. **扩展功能必须自己实现**。课程规定：用第三方库/程序/代码"直接"实现的扩展功能只记 50% 分。后端生产代码**禁止**引入 `archive/*`、`compress/*`、`crypto/aes|cipher|des|sha*|md5|hmac|pbkdf2`、`golang.org/x/*` 及任何第三方 Go 模块；`scripts/check-imports.sh`（`make lint` 的一部分）会拦截。这些标准库只允许出现在 `_test.go` 中充当对照组。`crypto/rand` 可用于生成随机盐/IV。唯一允许的测试依赖是 testscript。
2. **后端必须是编译型语言（Go）**。客户端只负责界面和调用后端 CLI，任何备份/还原/过滤/压缩/加密逻辑都不能写在客户端里。
3. **不提交学号与凭据**：学号、任何 API Key / token 一律不进仓库。署名只写"组长：王科瑾"。带真实学号的提交版文档只能输出到本地 `course/07-提交物/`。
4. **PDF 只在项目最后（M9-2）一次性生成**。开发期间不要运行文档 PDF 构建，不要提交 PDF；只维护 Markdown 与 PlantUML 源。
5. **AI 平台与 AI 截图不归 agent 管**：不要尝试操作阿里云/百炼等平台，也不要生成"AI 使用截图"。文档中只需注明本项目使用 AI 辅助开发。
6. **`course/` 是本地课程资料目录，已被 gitignore**，不存在于克隆中，不要引用其中的文件作为构建输入。需要的课程事实已整理进 `docs/project/course-requirements.md`。
7. 目标平台是 **Linux（Ubuntu 22.04/24.04）**。平台相关代码加 `//go:build linux`。

## 4. 工作流（一个任务的标准动作）

1. 在 `docs/roadmap.md` 找到任务 ID，确认依赖已合入、**触碰范围**和验收标准清楚。跨模块或预计超过半天的任务先写 `docs/plans/<任务ID>-<slug>.md`。
2. 独立 worktree + 分支：`git worktree add ../databackup-wt/<任务ID> -b <type>/<任务ID>-<slug> origin/master`。
3. 先写/改测试，再写实现（见 `docs/conventions/testing.md`）。验收标准必须用命令验证通过，而不是"看起来完成"。
4. 只改触碰范围内的文件；需要改冻结接口（CLI 协议、共享类型、`fsmeta` 公共类型）或 `engine` 时停下上报。
5. 同步文档：行为或接口变化 → 更新 `docs/architecture/*`、`docs/deliverables/*` 对应章节和 `CHANGELOG.md` 的 `[Unreleased]`。
6. 提交前通过 **`make check`**（Windows 主机：`bash scripts/docker.sh check`）；合并前或改动核心模块时跑 **`make check-all`**（主机：`bash scripts/docker.sh check-all`）。
7. Conventional Commits：`feat(pack): 实现 MyPack 头记录写入`，footer 写 `Refs: M2-3`。
8. 开 PR，CI 全绿后 squash 合并；在 `docs/roadmap.md` 只更新自己任务那一行的状态。

## 5. 常用命令

**Windows 主机**（只需 Git Bash + Docker Desktop，不需要 make）：

| 命令 | 作用 |
|---|---|
| `bash scripts/docker.sh check` | 在开发镜像里跑 `make check` |
| `bash scripts/docker.sh check-all` | 在开发镜像里跑 `make check-all` |
| `bash scripts/docker.sh make <目标...>` | 在开发镜像里跑任意 make 目标，如 `make coverage` |
| `bash scripts/docker.sh shell` | 进入开发容器 |
| `bash scripts/docker.sh images` | 重建开发镜像与文档镜像（网络受限时可设 `GH_PROXY` / `GH_MIRROR`） |
| `bash scripts/docker.sh docs` | 生成 PDF——**仅 M9-2 使用** |

**Linux / 开发容器内**：

| 命令 | 作用 |
|---|---|
| `make help` | 列出全部目标 |
| `make check` | 快速门禁：格式 + lint（含禁用 import 检查）+ 后端构建与测试 + 客户端构建与单测 |
| `make check-all` | 完整门禁：check + race 检测 + 后端 e2e + 客户端 e2e（含截图对比） |
| `make backend` / `make backend-test` / `make coverage` / `make bench` | 后端构建、测试、覆盖率、基准 |
| `make client-install` / `make client` / `make client-test` / `make e2e` | 客户端依赖、构建、单测、端到端 |
| `make package` | 产出后端二进制 + 客户端 AppImage/deb 到 `dist/` |

## 6. 仓库地图

```
backend/      Go 后端（CLI + 核心库，生产代码零第三方依赖）   → backend/AGENTS.md
client/       Electron + React + TypeScript 客户端             → client/AGENTS.md
docs/         项目文档（Markdown；PDF 在 M9-2 生成到 docs/pdf/）
scripts/      构建、lint、文档、发布、Docker 包装脚本
docker/       开发镜像与文档镜像
.github/      CI、Release、PR/Issue 模板
course/       本地课程资料（gitignored，仅作者机器上存在）
```

## 7. 不要做的事

- 不要在没有用户明确要求时 push、打 tag、发布 Release、改 GitHub 设置。
- 不要删除或放宽已有测试来让 CI 变绿；修 bug 必须带回归测试。
- 不要修改 `testdata/` 下的 golden 文件或界面截图基线，除非任务就是改它们，且在 PR 里说明原因。
- 不要绕过门禁（`--no-verify`、跳过 CI、注释掉 lint 规则）。
- 不要引入新依赖（Go 生产代码禁止；npm 需在 PR 中说明理由，版本精确锁定且发布满 7 天）。
- 不要往 `master` 直接推功能代码，不要 force push `master`。
- 不要生成或提交 PDF（M9-2 除外）。
- **不要自行提取或借用凭据**：禁止从 Git Credential Manager、浏览器、环境变量、SSH agent 等任何凭据存储中读取 GitHub/Git 凭据来调用 `gh` 或 GitHub API。需要 `gh` 时先跑 `gh auth status`，未登录就停下，请用户本人执行 `gh auth login`（交互式，agent 不可代办）。`git push/fetch/pull` 走系统凭据管理器是正常用法，不受此限。
