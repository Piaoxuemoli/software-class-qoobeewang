# C3 客户端接真实后端（真实后端 e2e 链路）

## 目标

roadmap W4 任务 C3：客户端从 `DATABACKUP_MOCK=1` mock 后端切换到真实 `databackup` 二进制。

验收标准（roadmap 原文）：**Playwright：界面完成一次"过滤+压缩+加密"备份并还原成功；取消、口令错误路径通过**。

## 调研结论（2026-10-16）

### 选型：沿用 Playwright `_electron`，不引入新依赖

- Spectron 自 Electron 24 起废弃，`_electron` 是社区事实标准（Electron ≥21 支持，我们 44.x）。
- `electronApp.evaluate` 可在主进程执行——是 stub 原生对话框的唯一途径：`dialog.showOpenDialog` / `showMessageBox` 走 OS API，Playwright **无法拦截**（官方文档明确说明）。`client/src/main/dialogs.ts` 的两个函数都必须靠 evaluate 替换才能在 e2e 中确定性驱动。
- `electron.launch({ env, artifactsDir })`：env 注入实现测试隔离；Playwright ≥1.59 的 `artifactsDir` 可落 trace/截图证据（已装 1.64）。
- **不引入 `electron-playwright-helpers`**（spaceagetv，MIT）：它提供的 `stubDialog` 等能力本质是十几行 `evaluate` 封装，照模式自写即可，避免新增 npm 依赖的合规成本。
- **不采用 playwright-mcp / electron-mcp 录制生成 spec**：用例数量少，手写 fixture + locator 更快更稳；Electron 下 codegen 支持弱。
- 参考的仓库内 agent skill 范本：sanity-io `.agents/skills/playwright-best-practices/`——本任务产出的 `.devin/skills/databackup-e2e/` 参照其结构。

### 参考项目的一致模式

restura、lichtblick、orca、Playwright 自身 `tests/electron/electronTest.ts` 都收敛到同一做法：`test.extend` fixture 每次 `mkdtemp` 隔离 `userData`/`home` 等目录。映射到本项目：后端配置读 `$XDG_CONFIG_HOME/databackup/config.json`、默认仓库 `~/DataBackup`——启动时注入 `HOME` + `XDG_CONFIG_HOME` 指向临时目录即满足 testing.md §2 的密闭要求，不依赖 `--repo` 参数走另一条代码路径。

## 复审与补充调研（2026-10-16，review-agent 独立核实）

对上文调研结论逐条做了仓库内证据核查 + 外部搜索验证，结论：**方案成立，可实施**。以下为核实结果、两处修正与四点补充。

### 声明核实表

| 声明 | 核实 | 证据 |
|---|---|---|
| Playwright 无法拦截原生 dialog，`evaluate` 是唯一途径 | ✅ 属实 | [官方 Electron 文档](https://playwright.dev/docs/api/class-electron)原文 "Playwright does not intercept the native Electron dialog API"；[microsoft/playwright#8278](https://github.com/microsoft/playwright/issues/8278) 确认 `filechooser` 事件对 Electron dialog 不触发 |
| `artifactsDir` 需 Playwright ≥1.59，已装 1.64 | ✅ 属实 | `electron.launch()` 的 `artifactsDir` 由 [playwright PR #39880](https://github.com/microsoft/playwright) 加入 1.59；`client/package.json:56` 为 `@playwright/test 1.64.0` |
| electron-playwright-helpers 只是 evaluate 封装、自写即可 | ✅ 属实 | [npm 包](https://www.npmjs.com/package/electron-playwright-helpers)的 `stubDialog` 即 `dialog.showOpenDialog = async () => ({...})` 模式封装；不引入符合仓库依赖红线 |
| lichtblick 的 e2e 模式 | ✅ 属实 | [lichtblick e2e/README](https://github.com/lichtblick-suite/lichtblick/blob/develop/e2e/README.md)：Playwright、web 与 desktop(Electron) 分层组织，与本项目 mock/real 两个 spec 的切分同构 |
| restura、orca、sanity-io skill 范本 | ⚠️ 未能独立核实 | 广泛搜索未定位到 restura/orca 的对应 e2e 代码（可能为小众/更名仓库）；sanity-io 的 `.agents/skills/playwright-best-practices/` 未检索到公开镜像。不依赖这三处的任何技术判断，仅作灵感来源，实施时以官方文档与 lichtblick 为准 |
| HOME/XDG 注入满足密闭要求 | ✅ 属实且更彻底 | 后端读 `$XDG_CONFIG_HOME`（resolve 于 `resolve.ts:10-18`、config 读取）与默认仓库 `~/DataBackup`；**Linux 上 Electron 自身 userData 也在 `$HOME/.config/<app>` 下，HOME 注入顺带隔离了它**——多数开源项目需单独 `--user-data-dir`/`app.setPath`，本方案一处注入双收益 |
| 状态栏版本就绪信号 | ✅ 属实 | `Sidebar.tsx:55-60`：`ready` ⇔ 启动时 `--version` 查询成功 ⇔ 显示 `0.1.0`（非 `0.0.0-mock`）；现有 mock 用例即用此模式等 `0.0.0-mock`（`app.spec.ts:24`） |
| 三个用例与状态机一致 | ✅ 属实 | TC-E2E-07/08/09 分别覆盖 interaction.md §3 的 `Ready→Running→Succeeded`、`Running→Cancelling→Cancelled`、`awaitingPassword` 的 E_AUTH 回退（`interaction.md:75-98`）；口令 3 次限制跨提交累计（`stores/job.ts:186-189,296-310`），TC-E2E-09 断言"还可尝试 2 次"与 `PasswordDialog.tsx:45` 一致 |

### 修正两处

1. **Spectron 废弃时间线**：上文"自 Electron 24 起废弃"不准确。[官方 deprecation notice](https://electronjs.org/blog/spectron-deprecation-notice) 发布于 **2022-02-01**（彼时 Spectron 已无法跟进新 Electron 版本，最新约 v17），与 Electron 24 无关。结论不变（Playwright 是官方推荐继任），仅修正事实表述。
2. **`make e2e` 现状**：`Makefile:53` 注释"launch built app against real backend"且传了 `DATABACKUP_BIN`，但 `app.spec.ts:17` 无条件注入 `DATABACKUP_MOCK:'1'` 且 mock 优先（`index.ts:12-18`，mock 时忽略 `DATABACKUP_BIN`）——**今天 `make e2e` 实际全程跑 mock**。`real.spec.ts` 不设 `DATABACKUP_MOCK` 正是修正此现状；顺手把 Makefile 注释改准确（或让注释保持前瞻性）。

### 补充四点（外部调研的增量结论）

1. **`firstWindow()` 超时要放宽**：社区反复报告 CI/Docker 慢环境下默认 30s（本项目现用 15s，`app.spec.ts:28`）不够（[Simon Willison 的经验](https://til.simonwillison.net/electron/testing-electron-playwright)：CI 上建议 90s）。真实链路首个用例要 spawn 后端 + 版本查询，比 mock 慢；fixture 的 `win` 建议给 30~60s 并允许配置。
2. **孤儿进程已闭环，排错手册照实记录**：真实链路引入"Electron 主进程被 `app.close()` 杀死后，spawn 的 `databackup` 是否残留"问题——本仓库已由 `client/src/main/index.ts:73-80` 的 `will-quit` 钩子解决（退出前 `cancelAll()`：SIGTERM→3s→SIGKILL，让后端删 `.partial` 再 quit）。Playwright 的 `electronApp.close()` 会正常触发 quit 流程。skill 排错手册应写明这条链路，避免后续误加 kill 逻辑。参考：[Docker 内 Electron e2e](https://blog.dangl.me/archive/running-fully-automated-e2e-tests-in-electron-in-a-docker-container-with-playwright)、[Spectron issue #1021 的 headless 坑](https://github.com/electron-userland/spectron/issues/1021)。
3. **teardown 收进 fixture**：现有 6 条用例每条手动 `await app.close()`（6 处），漏一处就漏杀进程。`test.extend` 化后把 `close` 放 fixture teardown（worker teardown 之前）是社区标准做法（[playwright#8208](https://github.com/microsoft/playwright/issues/8208) 讨论的自定义 electronApp fixture 即如此），比现有模式更稳。
4. **`artifactsDir` 用途具体化**：指向 `${tmp}/artifacts`，开 `trace: 'retain-on-failure'`（config 级）+ failure 截图；M8-1 测试报告与答辩排障时可直接引用 trace（时间线+每步截图），成本近零——这也是 1.59 加该参数的动机（[release notes](https://playwright.dev/docs/release-notes)）。

### 实施顺序提示

W3 的 4 个 PR 已合入 origin/master（ee3d71e 为最新）。本分支基于其后，只含 IDE 修复（fd4ac96，+8/-1，无依赖无构建行为变化，已核）与本方案文档两个提交——结构干净。实施 C3 时直接在此分支追加 `feat` 提交即可；注意 roadmap "C3 行状态：合并时更新" 由 C3 自己的 PR 完成。

## 设计

### 测试拓扑（三层断言）

```
renderer UI ──IPC──► main(runner/jobs) ──spawn──► databackup CLI ──► 文件系统
      │                                              │                    │
  UI 终态断言（toContainText/toHaveScreenshot）   JSONL 事件（隐含）   Node fs 断言（还原树=源树、无 .partial）
```

mock 链路只能证明前两层；真实链路的独有证据是第三层——文件系统结果。

### fixture（`client/e2e/fixtures.ts`）

`test.extend` 提供 `app` / `win` / `env`：

- `env`：`mkdtemp` 产出 `{ home, repo, src, dst }` 四个临时目录；`src` 预置声明式小树（普通文件、子目录、待排除的 `node_modules` 目录）；`dst` 为空目录（满足 `E_TARGET_NOT_EMPTY` 校验）。
- 启动 env：`HOME=env.home`、`XDG_CONFIG_HOME=$home/.config`、`DATABACKUP_BIN=<构建产物>`，**不设** `DATABACKUP_MOCK`；`getuid()==0` 时追加 `--no-sandbox`（沿用现有模式）。
- 就绪信号：状态栏出现真实后端版本（非 `0.0.0-mock`）。

### 用例（接 TC-E2E-06 之后编号）

| 编号 | 场景 | 关键断言 |
|---|---|---|
| TC-E2E-07 | 过滤+压缩+加密备份 → 列表出现 → 口令还原 → 完成 | UI 完成态 ×2；`repo` 中出现 `.dbk`；fs 断言还原树结构与内容（被排除目录不存在） |
| TC-E2E-08 | 备份进行中取消 | `已取消` 态；repo 中无 `.partial` 残留；`list` 不含该备份 |
| TC-E2E-09 | 加密备份口令错误→重试 | 错误口令 → `还可尝试 2 次`；正确口令 → 还原完成 + fs 断言 |

- 取消窗口：src 放若干 MB 级文件保证 pack 阶段够长；若仍过快则在 scan 阶段取消（以 progress 事件为准，不靠固定 sleep）。
- 对话框路径：还原至少一次经"浏览"按钮触发 `dialog.showOpenDialog` stub，覆盖 main 侧 `dialogs.ts`（mock 链路覆盖不到）。
- 截图：沿用 mask 规范新增 1~2 张真实链路终态图，基线只在 Docker 镜像内生成（testing.md §5）。

### 与 mock 链路的边界

`app.spec.ts` 现有 6 条保持不变（交互/截图回归，跑得快）；真实链路单独 `real.spec.ts`，同一 `npm run test:e2e` 入口，`make e2e` 已先构建 `backend/bin/databackup`。

### skill 遗产

`.devin/skills/databackup-e2e/SKILL.md`：链路拓扑、fixture 模板、三层断言规范、截图 mask 规范、排错手册（E_AUTH 时序、`.partial` 残留、xvfb、root `--no-sandbox`）、TC-E2E-NN 编号规则。供后续 agent 复用，不写 client 业务逻辑。

## 子任务

- [ ] `client/e2e/fixtures.ts`：electron fixture + 临时环境
- [ ] `client/e2e/real.spec.ts`：TC-E2E-07/08/09
- [ ] `.devin/skills/databackup-e2e/SKILL.md`
- [ ] Docker 内 `make e2e` 全绿（含既有 mock 用例）
- [ ] `make check-all` 全绿 → PR

## 测试计划

见上表；覆盖 FR-01/02（备份还原）、FR-过滤、FR-压缩、FR-加密 的界面端到端路径。测试报告（03）在 M8 统一引用 TC-E2E 编号。

## 文档同步

- `docs/client/interaction.md`：无需改（真实后端即契约行为）。
- `CHANGELOG.md` `[Unreleased]`：新增 C3 条目。
- roadmap C3 行状态：合并时更新。

## 附：IDE lint 报错处理记录（2026-10-16）

报错均为 `databackup-wt/` 下**已删除** worktree 的陈旧诊断，根因两条：

1. `tsconfig.web.json` 的 `baseUrl` 被编辑器内置新版 TS 报"已弃用"→ **已移除**（TS ≥4.1 起 `paths` 无需 `baseUrl`，相对 tsconfig 解析，行为不变）。
2. worktree 宿主无 `client/node_modules`（依赖在 per-worktree Docker 卷内）→ `types: ["node"/"vite/client"]` 解析失败。**已加 `.vscode/settings.json` 钉住 `typescript.tsdk` 指向工作区 TS**，并对主检出在宿主跑过 `npm ci`（仅 IDE 语言服务用途；该 worktree 的卷已存在，宿主目录在容器内被卷遮蔽，互不干扰）。

注意：新建的 worktree 在宿主天然没有 `node_modules`，用 IDE 打开其中的 client tsconfig/源文件仍会出现"找不到类型"噪声——以 Docker 门禁为准；如需在 worktree 长时间用 IDE 编辑 client 代码，可在该 worktree 的 `client/` 跑一次 `npm ci`（先确保 Docker 侧该 worktree 的 node 卷已初始化，再装宿主依赖，避免宿主二进制被复制进新卷）。

## 附：同类备份软件"排除"功能与命名调研（2026-10-17）

针对 C3 联调发现的「排除目录」胶囊文案与契约不符，调研同类软件的排除维度划分与命名，为 C5 文案修正提供依据。

### 业界一致把"排除"分成两个家族

| 家族 | 语义 | 代表实现 | 典型命名 |
|---|---|---|---|
| **路径/子树排除** | 字面路径，命中的目录整棵不进备份 | Duplicati `Exclude folder`（目录树选择器）、Time Machine `Exclude these items`（路径列表）、Déjà Dup `Folders to ignore`、borg `pp:` path prefix、rsync 锚定路径 | "排除文件夹/目录"、路径列表 |
| **名称/模式排除** | glob 或正则，匹配文件/目录名或整条路径 | restic `--exclude`（`filepath.Match`+`**`，对全路径）、borg `--exclude`（`fm:`/`sh:`/`re:` 前缀显式选风格）、Duplicati `Exclude regular expression`（`[regex]` 语法）、Kopia policy Ignore Rules（gitignore 式）、tar `--exclude` | "排除模式/正则/名称" |

关键观察：

- **没有任何主流工具把"目录排除"实现成正则语义**。Duplicati 论坛的反复投诉恰好是同一类混淆——用户把正则填进 folder 字段导致静默不生效（与我们抓到的问题同构），说明 UI 措辞必须显式区分"路径"与"模式"。
- **正则/模式排除**一族（restic、borg `re:`、Duplicati `[…]`）在 UI 上都明示 "regular expression / pattern"；路径排除一族一律用"folder/directories"字样 + 目录选择器。
- borg 的做法最干净：同一 `--exclude` 选项用 `fm:`/`sh:`/`re:`/`pp:` 前缀自选语义，避免歧义——我们后端用两个独立 flag（`--exclude-dir` 路径 / `--exclude-name` RE2 正则）达到同样效果，契约本身没有歧义。

### 对本项目的结论

1. 六个筛选胶囊中五个落在模式/属性家族（类型、时间、大小、用户组、名称正则），只有「排除目录」落在路径家族——C5 修正项（`ui-polish.md §6` 已收录：`FilterCapsules.tsx:298` 改为「排除目录（每行一条路径，相对源目录）」）方向正确，符合业界惯例。
2. 可作为后续候选能力（不在 C3/C5 范围）：restic/borg/tar 均支持的**标记文件排除**（`--exclude-if-present`、`CACHEDIR.TAG`、`.kopiaignore` 式 per-dir ignore file），即"目录里放了标记文件就整棵跳过"——比写路径列表更符合开发者排除 `target/`、`node_modules/` 的习惯。
