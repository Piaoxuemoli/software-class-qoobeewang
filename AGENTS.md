# AGENTS.md

任何进入本仓库工作的 agent（Devin / Claude Code / Codex / Qoder …）从这里开始。本文是 agent 规则的唯一来源，`CLAUDE.md` 等只做转发。

## 1. 项目一句话

电子科技大学《软件开发综合实验》课程项目：**数据备份软件 DataBackup**。Linux 下把目录树备份为单个备份文件，并能原样还原（文件类型、元数据、过滤、自研打包格式、Huffman 压缩、AES 加密），配一个 Electron 桌面客户端。详见 `docs/project/overview.md`。

## 2. 开工前必读（按顺序）

| 文档 | 用途 |
|---|---|
| `docs/project/overview.md` | 项目简介、功能清单（FR 编号）、技术选型与运行环境 |
| `docs/project/course-requirements.md` | 课程硬性要求、评分规则、提交物清单（**需求的最终依据**） |
| `docs/roadmap.md` | 里程碑与任务拆分（任务 ID、验收标准、依赖、可否并行） |
| `docs/project/schedule.md` | 时间进度与当前状态 |
| `docs/architecture/overview.md` | 架构、模块划分、数据流水线 |
| `docs/architecture/cli-contract.md` | 后端 CLI / JSONL 协议（客户端与后端的唯一接口） |
| `docs/conventions/*.md` | git、测试、发布、编码、文档规范 |

## 3. 硬性约束（违反会直接丢分，必须遵守）

1. **扩展功能必须自己实现**。课程规定：用第三方库/程序/代码"直接"实现的扩展功能只记 50% 分。后端生产代码**禁止**引入 `archive/*`、`compress/*`、`crypto/aes|cipher|des|sha*|md5|hmac|pbkdf2`、`golang.org/x/*` 及任何第三方 Go 模块；`scripts/check-imports.sh`（`make lint` 的一部分）会拦截。这些标准库只允许出现在 `_test.go` 中充当对照组（differential testing）。`crypto/rand` 可用于生成随机盐/IV。
2. **后端必须是编译型语言（Go）**。客户端界面可以用 TypeScript，但任何备份/还原/压缩/加密逻辑都不能写在客户端里——客户端只负责界面和调用后端 CLI。
3. **不提交个人信息与凭据**：姓名、学号、百炼 API Key、任何 token 一律不进仓库。带真实身份的提交版文档只能输出到本地 `course/07-提交物/`（见 `docs/conventions/docs.md`）。
4. **`course/` 是本地课程资料目录，已被 gitignore**，不存在于克隆中，不要引用其中的文件作为构建输入，也不要试图把它加回版本库。需要的课程事实已整理进 `docs/project/course-requirements.md`。
5. 目标平台是 **Linux（Ubuntu 22.04/24.04）**。特殊文件、属主等只在 Linux 上有意义；平台相关代码加 `//go:build linux`。

## 4. 工作流（一个任务的标准动作）

1. 在 `docs/roadmap.md` 找到任务 ID（如 `M2-3`），确认依赖已完成、验收标准清楚。任务较大（跨模块或预计超过半天）先写 `docs/plans/<任务ID>-<slug>.md`（目标 / 设计 / 子任务 / 测试计划）。
2. 从最新 `master` 拉分支：`<type>/<任务ID>-<slug>`，例如 `feat/M2-3-mypack-writer`。并行开发多个任务时每个任务用独立 `git worktree`。
3. 先写/改测试，再写实现（见 `docs/conventions/testing.md`）。
4. 同步文档：行为或接口变化 → 更新 `docs/architecture/*`、`docs/deliverables/*` 对应章节和 `CHANGELOG.md` 的 `[Unreleased]`。
5. 提交前必须通过 **`make check`**（Windows 主机没有 make，用 `bash scripts/docker.sh check`）；合并前或改动核心模块时跑 **`make check-all`**（主机：`bash scripts/docker.sh check-all`）。
6. Conventional Commits：`feat(pack): 实现 MyPack 头记录写入`，footer 写 `Refs: M2-3`。
7. 开 PR（模板会要求填任务 ID、测试证据、文档同步情况），CI 全绿后 squash 合并到 `master`。
8. 在 `docs/roadmap.md` 把任务状态改为完成，必要时更新 `docs/project/schedule.md`。

## 5. 常用命令

**Windows 主机**（只需 Git Bash + Docker Desktop，不需要 make）：

| 命令 | 作用 |
|---|---|
| `bash scripts/docker.sh check` | 在开发镜像里跑 `make check` |
| `bash scripts/docker.sh check-all` | 在开发镜像里跑 `make check-all` |
| `bash scripts/docker.sh make <目标...>` | 在开发镜像里跑任意 make 目标，如 `make coverage` |
| `bash scripts/docker.sh shell` | 进入开发容器 |
| `bash scripts/docker.sh docs` | Markdown → PDF（结果在 `docs/pdf/`） |
| `bash scripts/docker.sh images` | 重建开发镜像与文档镜像（网络受限时可设 `GH_PROXY` / `GH_MIRROR` 环境变量） |

**Linux / 开发容器内**：

| 命令 | 作用 |
|---|---|
| `make help` | 列出全部目标 |
| `make check` | 快速门禁：格式 + lint（含禁用 import 检查）+ 后端构建与测试 + 客户端构建与单测 |
| `make check-all` | 完整门禁：check + race 检测 + 后端 e2e + 客户端 e2e |
| `make backend` / `make backend-test` / `make coverage` / `make bench` | 后端构建、测试、覆盖率、基准 |
| `make client-install` / `make client` / `make client-test` / `make e2e` | 客户端依赖、构建、单测、端到端 |
| `make docs` | Markdown → PDF（需文档工具链，一般用上面的 docker 方式） |
| `make package` | 产出后端二进制 + 客户端 AppImage/deb 到 `dist/` |

## 6. 仓库地图

```
backend/      Go 后端（CLI + 核心库，零第三方依赖）
client/       Electron + React + TypeScript 客户端
docs/         项目文档（Markdown 源 + docs/pdf/ 下的 PDF 备份）
scripts/      构建、lint、文档、发布脚本
docker/       开发镜像与文档镜像
.github/      CI、Release、PR/Issue 模板
course/       本地课程资料（gitignored，仅作者机器上存在）
```

## 7. 不要做的事

- 不要在没有用户明确要求时 push、打 tag、发布 Release、改 GitHub 设置。
- 不要删除或放宽已有测试来让 CI 变绿；修 bug 必须带回归测试。
- 不要修改 `testdata/` 下的 golden 文件，除非任务就是改格式，且在 PR 里说明原因。
- 不要绕过门禁（`--no-verify`、跳过 CI、注释掉 lint 规则）。
- 不要引入新的依赖（Go 侧禁止；npm 侧需在 PR 中说明理由，版本精确锁定且发布满 7 天）。
- 不要往 `master` 直接推功能代码，不要 force push `master`。
