# Git 工作流规范

本项目采用课程介绍的**功能分支工作流**（课件2"软件协作工作流"第 2 种）：`master` 是唯一长期分支，所有改动经短生命周期分支 + Pull Request 合入。

## 1. 分支

| 分支 | 用途 | 规则 |
|---|---|---|
| `master` | 始终可构建、可发布 | 受保护：只能通过 PR 合入，CI 必须全绿；禁止 force push |
| `<type>/<任务ID>-<slug>` | 一个任务一个分支 | 从最新 `master` 创建，合并后删除 |

`type` 取值同提交类型：`feat` `fix` `test` `docs` `refactor` `perf` `build` `ci` `chore`。示例：`feat/M2-3-mypack-writer`、`fix/M4-1-empty-input`、`docs/M1-1-use-cases`。

没有任务 ID 的小改动（错别字、依赖升级）用 `chore/<slug>` 或 `docs/<slug>`。

## 2. 提交信息（Conventional Commits）

```
<type>(<scope>): <中文简述，祈使语气，不加句号>

<可选正文：为什么改、怎么改、影响>

Refs: <任务ID>
```

- `scope` 用模块名：`walk` `filter` `pack` `huffman` `crypto` `container` `engine` `repo` `config` `cli` `client` `docs` `ci` `build` `release`。
- 破坏性变更（CLI 协议、MyPack/容器格式不兼容）在 footer 写 `BREAKING CHANGE: ...`。
- 一个提交只做一件事；提交前 `make check` 通过。
- 示例：`feat(huffman): 实现分块流式编码器`、`fix(pack): 修正长文件名多头块的校验和计算`。

## 3. Pull Request

0. 先 `gh auth status`：**未登录则停下并上报用户**（请其执行 `gh auth login`）。禁止从 Git Credential Manager 或任何凭据存储提取凭据来调用 `gh`/GitHub API；`git push` 本身走系统凭据管理器是正常用法。
1. 分支推送后开 PR，标题即最终的 squash 提交信息（同样遵循 Conventional Commits）。
2. 按模板填写：任务 ID、变更说明、测试证据、文档同步、风险。
3. CI 全绿后 **squash merge**；合并前若 `master` 已前进，先 `git rebase origin/master` 再推送（自己的任务分支允许 `--force-with-lease`）。
4. 合并后删除分支，并在 `docs/roadmap.md` 更新任务状态。

## 4. 多 agent 并行

- 一个 agent 一个任务、一个分支、一个 worktree：`git worktree add ../databackup-wt/<任务ID> -b feat/<任务ID>-<slug> origin/master`。
- 并行任务只应修改各自模块；共享文件（`cli-contract.md`、`client/src/shared/`、`Makefile`、CI 配置）的改动单独开 PR，尽快合入，其他分支随后 rebase。
- 发现需要修改已冻结接口时停止并上报，不要在功能分支里顺手改。

## 5. 禁止事项

- 直接向 `master` 推送功能代码；force push `master`；改写已推送的公共历史。
- 提交 `course/`、个人信息（姓名、学号）、密钥/Token（如百炼 API Key）、构建产物（`backend/bin/`、`client/out/`、`dist/`）。
- 用 `--no-verify` 或注释掉检查来绕过门禁。
- 未经作者明确要求就 push、打 tag 或发布 Release。
- 自行提取/借用任何凭据存储中的凭据（含 Git Credential Manager）来调用 `gh`、GitHub API 或其他服务。

## 6. 标签

只用于发布：`vX.Y.Z`（附注标签），流程见 `release.md`。
