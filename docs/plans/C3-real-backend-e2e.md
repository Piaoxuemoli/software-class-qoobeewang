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
