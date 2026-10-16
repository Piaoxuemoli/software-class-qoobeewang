---
name: databackup-e2e
description: DataBackup 客户端 Playwright Electron e2e——真实后端链路（fixture/断言/截图）与排错。触发：写或改 client/e2e 用例、调 e2e/截图基线、验收 C3、排查 xvfb-run 挂起。
---

# databackup-e2e：Electron 端到端链路手册

适用场景：为 `client/e2e/` 新增或修改用例、更新截图基线、在 Docker 开发镜像里跑/调 e2e。
前置阅读：`docs/plans/C3-real-backend-e2e.md`（方案依据）、`docs/client/interaction.md`（状态机与错误映射）。

## 链路拓扑（三层断言）

```
renderer UI ──IPC──► main(runner/jobs) ──spawn──► databackup CLI ──► 文件系统
      │                                              │                    │
  UI 终态断言（toContainText / toHaveScreenshot）  JSONL 事件          Node fs 断言
```

- **mock 链路**（`app.spec.ts`，`DATABACKUP_MOCK=1`）：只证前两层，快，做交互/截图回归。
- **真实链路**（`real.spec.ts` + `fixtures.ts`，不设 `DATABACKUP_MOCK`）：额外证文件系统层——
  `.dbk` 落盘、无 `.partial` 残留、还原树与源树一致。这是 mock 永远给不了的证据。

## 运行方式

| 环境 | 命令 |
|---|---|
| Windows 主机 | `bash scripts/docker.sh make e2e`（= 构建 backend+client 后 xvfb 下跑全部 spec） |
| 容器内 | `make e2e` 或 `cd client && npx playwright test` |
| 只跑真实链路 | `npx playwright test e2e/real.spec.ts` |
| 更新截图基线 | `npx playwright test e2e --update-snapshots`（**只能在 Docker 开发镜像里做**，主机字体不同） |

## fixture（`client/e2e/fixtures.ts`）要点

- `env`：`mkdtemp` 出 `{home, xdg, repo, src, dst}`。启动时注入 `HOME`+`XDG_CONFIG_HOME`
  → 后端 `config.json`、默认仓库 `~/DataBackup`、Electron userData **一处注入三处隔离**。
  `src` 预置 `hello.txt`/`sub/data.bin`/`node_modules/dep/index.js`；`dst` 为空目录。
- `app`：`electron.launch`（root 下自动加 `--no-sandbox`），`artifactsDir` 落临时目录；
  trace 手动挂 `app.context().tracing`，**失败才写 `trace.zip`**（testInfo.outputPath）。
  teardown 里 `app.close()` 走正常 quit——main 的 `will-quit` 钩子会 `cancelAll()`
  运行中的后端（SIGTERM→3s→SIGKILL），**不要**再自写杀进程逻辑。
- `win`：`firstWindow({timeout:60_000})` + 等状态栏 `/后端版本: (?!0\.0\.0-mock)\S+/`。
  CI/Docker 慢，60s 是社区经验值；**不要**缩短。
- 助手：`stubOpenDialog(app, [path, ...])`（`evaluate` 替换 `dialog.showOpenDialog`，
  队列式返回；必须在点"浏览…"之前调用）、`runCli(args, stdin)`（前置造数据）、
  `assertTreeEqual(src, dst, {excludeDirs})`、`makeBigTree(dir)`。

## 写新用例的清单

1. 用 `test`/`expect` 从 `./fixtures` 导入，不用 `@playwright/test` 直接导入。
2. 编号接 `TC-E2E-NN` 递增（testing.md §3），中文描述场景。
3. 断言凑齐三层：UI 终态 + 仓库/文件系统事实 +（必要时）协议行为。
4. 等时序事件不用 `sleep`：取消等 `job-percent`/`current-path` 出现
   （此时 jobId 已注册，`jobs.cancel` 才真正生效——RunningCard 渲染早于 spawn 完成）。
5. 截图沿用 mask 规范：遮一切随时间/路径变化的元素（stat-耗时、sidebar-repo、
   路径行 `div.truncate.font-mono` 等），其余统计量保持稳定可做证据。
6. 需要慢源树才能取消时，用 `makeBigTree`（8000 小文件 + 32MB），别靠 sleep。

## 已踩过的坑（排错手册）

| 症状 | 原因 | 处理 |
|---|---|---|
| `xvfb-run -a …` 挂住、容器内只有 xvfb-run+Xvfb 无 node | xvfb-run 的 USR1 就绪握手丢信号（竞态；退出时清理阶段也可能卡） | 已根治：`make e2e` 改走 `scripts/e2e-display.sh`（自起 Xvfb + socket 就绪探测，分支 `fix/xvfb-e2e-race`）。若所在分支尚未包含该修复，临时解法：`docker exec <cid> kill -USR1 1` |
| 还原树多了被"排除"的目录 | `--exclude-dir` 接**相对源目录的路径**，不是正则；UI 胶囊文案"每条正则"与契约不符 | 填字面目录名（`node_modules`）；文案问题是已知偏差，留给 C5/UI 打磨处理 |
| 备份管理列表看不到刚造的备份 | `backupsStore` 只在启动预取/备份成功/删除/改 repo 时刷新 | 点"刷新"按钮，或在 app 启动前把数据造好 |
| 点取消后任务仍成功 | RunningCard 渲染早于 `backup.start` 返回 jobId，此时 `cancel()` 是空操作 | 先等 progress（`job-percent`/`current-path`）出现再点 |
| `E_AUTH` 后不回口令框 | 连续 3 次错误会回 ready；mock 口令是 `mock`，真实链路用你造种时的口令 | 断言"还可尝试 N 次"时确认次数累计语义 |
| CI 上 `firstWindow` 超时 | 慢环境下默认 15~30s 不够 | fixture 已放宽到 60s，勿缩短 |
| `find电子` | Electron 原生 dialog 不可被 Playwright 拦截 | 一律走 `stubOpenDialog`（evaluate 替换 main 侧函数） |
| e2e 结束后残留 databackup/.partial | 已由 `will-quit`→`cancelAll()` 闭环 | 正常 `app.close()` 即可；若仍见残留，查后端是否忽略 SIGTERM |

## 红线

- 不把业务判断写进 e2e helper（fs 断言只做"集合+内容"级别，元数据五要素是后端 testutil 的职责）。
- 不改 golden 截图基线除非任务就是改 UI，且在 PR 说明。
- 截图/trace 证据只进 `test-results/`（gitignored）；入库的只有 `*-snapshots/` 基线。
