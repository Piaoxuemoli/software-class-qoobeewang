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

<可选正文：只写 diff 看不出来的东西>

Refs: <任务ID>
```

- `scope` 用模块名：`walk` `filter` `pack` `huffman` `crypto` `container` `engine` `repo` `config` `cli` `client` `docs` `ci` `build` `release`。
- 破坏性变更（CLI 协议、MyPack/容器格式不兼容）在 footer 写 `BREAKING CHANGE: ...`。
- 一个提交只做一件事；提交前 `make check` 通过。

### 2.1 标题怎么写

一句话回答"这个提交改了什么"，像对同事口头解释：

- **只说一件事**。一个动词一个宾语，≤25 个汉字。放不下的信息要么砍掉，要么进正文。
- **禁止破折号副标题**。`xxx——yyy` 这种"主标题 + 副标题"结构说明你想说的不止一件事，拆分或删减。
- **尾巴不挂编号/状态**。`（M8-1~4）`、`（仓库版）`、`（M9-5 配套）` 一律去掉——任务编号属于 `Refs:` footer，状态属于正文或不写。
- **用具体动词，不用模板腔**。`成文`、`就绪`、`沉淀`、`可用`、`收敛`、`落地` 换成 `补全`、`升级到`、`新增`、`改为`、`修复`。
- **结论性证据不进标题**（"两遍全绿""实测通过"），验证方式放正文一行带过。

### 2.2 正文怎么写

**默认不写正文**。标题说不清以下任一项时才写，且不超过 5 行：

1. **为什么**这么改（动机、约束、根因）；
2. **取舍**：为什么选这个方案而不是显然的替代方案；
3. **影响面**：波及哪些行为、需要谁注意。

禁止：

- 复述 diff（"新增了 A、修改了 B、补充了 C"——看 diff 更快）；
- 数字堆砌自证（"实测 6 次 5504~5522 字节""18.8s/18.6s"——测试数据属于测试报告，正文最多留支撑结论的一个数）；
- 排查叙事（"此前本地全绿属侥幸"——排查过程留在 PR 讨论或 plans 文档，正文只留结论）；
- 口语夸张词：根治、翻车、完美、彻底、全绿、恰好。

### 2.3 真实对照（本仓库历史为例）

| 原来（AI 味） | 改写 |
|---|---|
| `docs(test): 软件测试报告成文——24 例模板用例表与全部证据（M8-1~4）` | `docs(test): 补全软件测试报告` |
| `feat(client): C4 Linux 打包可用——AppImage/deb 内置后端 + WSL 运行说明` | `feat(client): 支持 AppImage 和 deb 打包` |
| `test(client): C3 客户端接真实后端——Playwright 真实链路 e2e` | `test(client): 客户端 e2e 改跑真实后端` |
| `fix(ci): 发布资产 PDF 改用 ASCII 名，绕开 GitHub 中文名剥字碰撞` | `fix(ci): 发布资产改用 ASCII 文件名` |
| `fix(scripts): install-deps 包名选择跳过纯虚拟包，修 WSL 24.04 安装失败` | `fix(scripts): install-deps 兼容 Ubuntu 24.04` |
| `chore(release): 版本号就绪 v1.0.0` | `chore(release): 升级版本号到 1.0.0` |

正文对照（同一个提交）：

```
# 原（17 行：枚举 + 数字堆砌 + 叙事）
fix(client): C3 完成卡截图不再断言输出/压缩率数值，mask+格式断言

CI 产物取证（ci.yml 失败时上传 test-results）：差异 217px 恰在
stat-输出文件 与 stat-压缩率 数值区。机制：二者由 Huffman(打包流)
大小推出，流内嵌 mtime 纳秒/uid/gid 等元数据——固定源数据实测
root 容器 ≈5508B、uid1000 ≈5519B、CI runner ≈5591B：逐次稳定、
跨环境恒定漂移，基线环境锁定，像素级断言不可行。改为 mask 罩住
两位统计 + toHaveText 格式断言；精确数值由 TC-CLI-400 矩阵保证。
基线随之重建（mask 区域为固定色块）。全量 e2e 连跑两遍 9/9 全绿。

# 改写（4 行：结论 + 原因 + 依据去向）
fix(client): 完成卡截图改为遮罩统计数值

输出文件和压缩率由打包流里的 mtime、uid 决定，不同环境下渲染
必然不同，像素断言无法稳定。改为遮罩这两个数值并校验格式，
数值正确性已由 TC-CLI-400 组合矩阵覆盖。
```

```
# 原
fix(scripts): install-deps pick() 避开 pipefail+grep -q 的 SIGPIPE 陷阱

#33 的 pick() 在 set -o pipefail 下失效：grep -q 命中即退出，apt-cache
被 SIGPIPE 打断返回非零，整条管道判负，noble 上六个 t64 备选包全部
被跳过（脚本对 pick 失败有容错，最终仍 exit 0，形成"装好了"的假象；
WSL 24.04 实测 libasound/libgtk 等 Electron 运行库缺失）。改为先捕获
输出再以 herestring 匹配，彻底绕开管道。修正后 WSL 内补齐全部 t64
库，GUI 正常启动。

# 改写
fix(scripts): install-deps 在 24.04 上漏装六个运行库

grep -q 提前退出会通过 SIGPIPE 弄坏整条管道，pipefail 下六个
t64 备选包被静默跳过。改为先存输出再匹配。
```

## 3. Pull Request

0. 先 `gh auth status`：**未登录则停下并上报用户**（请其执行 `gh auth login`）。禁止从 Git Credential Manager 或任何凭据存储提取凭据来调用 `gh`/GitHub API；`git push` 本身走系统凭据管理器是正常用法。
1. 分支推送后开 PR，标题遵循 §2.1。
2. 按模板填写：任务 ID、变更说明、测试证据、文档同步、风险。
3. CI 全绿后 squash merge。**必须显式指定标题和正文**，不要让 GitHub 拼接分支提交列表：

   ```bash
   gh pr merge <PR> --squash --subject "<§2.1 标题>" --body "<§2.2 正文，可空>"
   ```

   不带 `--body` 时 GitHub 会把分支上全部提交信息（含各 agent 署名）拼进 squash 正文，历史上出现过单个合并提交正文带 8 段广告署名的事故。
4. 合并前若 `master` 已前进，先 `git rebase origin/master` 再推送（自己的任务分支允许 `--force-with-lease`）。
5. 合并后删除分支，并在 `docs/roadmap.md` 更新任务状态。

## 4. 多 agent 并行

- 一个 agent 一个任务、一个分支、一个 worktree：`git worktree add ../databackup-wt/<任务ID> -b feat/<任务ID>-<slug> origin/master`。
- 并行任务只应修改各自模块；共享文件（`cli-contract.md`、`client/src/shared/`、`Makefile`、CI 配置）的改动单独开 PR，尽快合入，其他分支随后 rebase。
- 发现需要修改已冻结接口时停止并上报，不要在功能分支里顺手改。
- agent 产出的提交按 §2 写人话；`Generated with ...` / `Co-Authored-By` 署名在 squash 时被 `--body` 覆盖，不会进入 master 历史。

## 5. 禁止事项

- 直接向 `master` 推送功能代码；force push `master`；改写已推送的公共历史。
- 提交 `course/`、个人信息（姓名、学号）、密钥/Token（如百炼 API Key）、构建产物（`backend/bin/`、`client/out/`、`dist/`）。
- 用 `--no-verify` 或注释掉检查来绕过门禁。
- 未经作者明确要求就 push、打 tag 或发布 Release。
- 自行提取/借用任何凭据存储中的凭据（含 Git Credential Manager）来调用 `gh`、GitHub API 或其他服务。

## 6. 标签

只用于发布：`vX.Y.Z`（附注标签），流程与 Release Notes 写法见 `release.md`。
