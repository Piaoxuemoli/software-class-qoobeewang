# 测试规范

目标：让 agent 可以大规模、并行地改代码而不破坏已有功能；同时直接产出软件测试报告所需的证据（课程要求 ≥2 种测试类型、≥15 个用例）。

## 1. 测试分层

| 层 | 位置 | 运行方式 | 内容 |
|---|---|---|---|
| 单元测试（白盒） | 后端各包 `*_test.go`；客户端 `client/src/**/*.test.ts(x)` | `make backend-test` / `make client-test` | 纯函数与模块：过滤条件、头记录编解码、Huffman、AES/SHA 测试向量、JSONL 解析 |
| 集成测试 | 后端 `engine`、`pack` 等包内，使用真实文件系统（`t.TempDir()`） | 同上（默认执行） | 遍历 + 过滤 + 打包 + 压缩 + 加密 组合往返 |
| CLI 黑盒（testscript） | `backend/cmd/databackup/testdata/script/*.txtar`（M1-4 引入） | `make backend-test` | 一个 txtar 文件 = 一组命令 + 期望 stdout/stderr/退出码 + 输入文件；`-update` 可显式更新期望 |
| 端到端（黑盒） | `backend/e2e/`（`//go:build e2e`）；`client/e2e/`（Playwright） | `make backend-e2e` / `make e2e` | 构建出的真实二进制 / 真实客户端窗口 |
| 界面回归 | `client/e2e/layout.spec.ts`（结构契约）+ `app.spec.ts` / `layout.spec.ts` 的 `toHaveScreenshot` | `make e2e`（Docker 开发镜像）；更新基线用 `make e2e-update BASELINE_REASON="..."` | 几何与计算样式断言兜"样式整体失效"；整页只留空闲态，状态与浮层用元素截图；基线清单与评审规则见 `docs/client/visual-regression.md` |
| 模糊测试 | `Fuzz*` 函数（pack.Reader、huffman 解码、容器头解析） | `go test -fuzz=FuzzX -fuzztime=30s ./internal/pack` | 任意输入不 panic、不死循环 |
| 性能测试 | `Benchmark*` 函数 | `make bench`；pprof | 吞吐、压缩率、耗时 |
| 并发安全 | 全部后端测试 | `make backend-race` | 数据竞争检测 |

`make check` 跑单元 + 集成 + 客户端单测；`make check-all` 额外跑 race、两端 e2e。

## 2. 硬性规则

1. **任何行为变更都要有测试**；修 bug 先写能复现的失败测试，再修复。
2. **不得删除、跳过或放宽已有测试**来通过门禁。确需修改断言时，在 PR 中说明原因。
3. **测试必须密闭**：只在 `t.TempDir()`（客户端用系统临时目录）内读写；不访问网络；不依赖执行顺序；不依赖当前工作目录以外的仓库文件（`testdata/` 除外）。唯一例外是**契约测试**（`backend/internal/jsonl/contract_test.go`、`backend/internal/cli/errors_test.go`、`client/src/shared/contract.test.ts`）：它们只读 `docs/architecture/cli-contract.md`，保证文档、Go 类型、TS 类型三者一致（ADR-011）。
4. **需要 root 的测试**（设备文件、chown）用 `if os.Geteuid() != 0 { t.Skip("需要 root") }`；Docker 开发镜像和 CI 以 root 运行，会真正执行。
5. **黄金文件**放在包内 `testdata/`，只能通过 `go test ./internal/pack -update` 显式更新，且 PR 必须说明格式为何改变。
6. **密码学原语**必须通过公开测试向量（FIPS-197、NIST SHA-256、RFC 4231、PBKDF2-HMAC-SHA256 向量），并在测试中与标准库实现做随机对照（标准库只允许出现在 `_test.go`）。
7. **往返测试的比较基准是正确性五要素**：类型、路径、数据、元数据（权限/属主/时间）、链接方式。统一使用 `internal/testutil`（借鉴 restic 的声明式目录树）：

   ```go
   tree := testutil.Dir{
       "a.txt":   testutil.File{Content: "hello", Mode: 0o640},
       "sub":     testutil.Dir{"b.bin": testutil.File{Size: 1 << 20}},
       "ln":      testutil.Symlink{Target: "a.txt"},
       "hard":    testutil.Hardlink{Target: "a.txt"},
       "pipe":    testutil.FIFO{},
   }
   testutil.CreateTree(t, src, tree)
   // ... 备份 → 还原到 dst ...
   testutil.AssertTreeEqual(t, src, dst) // 五要素逐项比较
   ```

   不要各写各的目录比较逻辑。要点：

   - 未指定的 atime/mtime 统一取 `testutil.DefaultTime`、权限取默认值并显式 chmod（与 umask 无关），所以同一声明生成的两棵树五要素完全一致；"没恢复时间/权限"的缺陷必然暴露。
   - 比较器直接用 `syscall` 读 inode，不调用 `fsmeta`（不用被测代码验证被测代码）；不比较两个根目录本身、ctime/btime 与 inode 号，硬链接按"哪些路径共享 inode"的划分比较。
   - 选项：`IgnoreOwner()`（非 root 还原）、`IgnoreTimes()`、`WithAtime()`（默认不比 atime：读文件会刷新它）。含 `Device` 或他人 `Owner` 的树在非 root 下 `CreateTree` 自动 `t.Skip`。
   - 黄金文件用 `testutil.Golden(t, "testdata/x.golden", got)`，`-update` 由 testutil 统一定义，测试包不要再定义同名 flag。
8. 测试运行要快：单个包的单元测试应在秒级完成；耗时测试放进 `Benchmark` 或 e2e。
9. **验收标准必须可由命令判定**：每个 roadmap 任务的验收都对应能运行的测试或检查，agent 以"检查通过"而不是"看起来完成"作为结束信号。
10. 测试依赖：只允许 Go 标准库 + testscript；新增其他测试依赖需在 PR 说明理由（生产代码依然零依赖，`check-imports` 强制）。

## 3. 用例编号与追踪

- 每个对应测试报告的用例用子测试命名，编号格式 `TC-<模块>-<三位序号>`：

  ```go
  t.Run("TC-PACK-001 普通文件往返", func(t *testing.T) { ... })
  ```

  模块缩写：`WALK` `FILT` `PACK` `HUFF` `AES` `SHA` `KDF` `CONT` `ENG` `REPO` `CFG` `CLI` `UI` `E2E` `PERF` `ROB`（健壮性）。
- 测试报告中的每个用例表必须能在代码里搜到同名编号；`docs/deliverables/03-软件测试报告.md` 维护 FR ↔ TC 追踪表，每个 FR 至少一个用例。
- 编号一经使用不再复用；删除用例时在报告中标注"已废弃"。

## 4. 覆盖率与证据

- `make coverage` 生成 `build/coverage/backend.html`；核心包（walk、filter、pack、huffman、crypto/*、container、engine）语句覆盖率目标 ≥80%。
- 测试报告需要的证据（覆盖率截图、benchmark 输出、pprof/perf 火焰图、race 结果、lint 结果）在 M8 统一采集，图片放 `docs/deliverables/assets/`。

## 5. 客户端

- 单元测试（Vitest）覆盖：JSONL 解析、后端事件到界面状态的映射、表单校验、参数到 CLI 参数的转换。
- 业务交互测试使用 mock 后端（可注入的 `BackendRunner`），不依赖真实二进制。
- e2e（Playwright `_electron`）只覆盖关键路径：启动显示版本、完成一次备份并还原、取消、口令错误；无显示器环境下由 `xvfb-run` 执行。
- 状态机测试：`interaction.md` §3 的每条状态转移至少一个用例（用 mock 后端驱动事件序列）。
- 截图基线只在 Docker 开发镜像中生成和比对，避免主机字体差异。
- 截图基线分三层：结构契约（几何/计算样式，不含图片）→ 整页空闲态（3 张）→ 浮层元素截图（Sheet / 对话框 / Popover）。新增或改动基线必须同步 `client/e2e/baselines.md`，并由人工逐张核对；细节见 `docs/client/visual-regression.md`。
