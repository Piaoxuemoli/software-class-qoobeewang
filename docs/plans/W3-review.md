# W3 集成泳道复审报告

> 审查对象：worktree `databackup-wt/M6-3` HEAD=fc0f00e，基线 origin/master 2bd33b0。
> 覆盖提交：#27 `f0577b6`（engine 全流水线，M2-5/M3-1/M3-2/M4-2/M5-5）、#28 `0bf420a`（契约补遗）、#29 `fc0f00e`（M6-3 全链路测试）。
> 审查方式：5 路并行深审（备份流水线 / 还原+fsmeta+walk / CLI 与测试有效性 / 安全专项 / 契约与文档一致性）+ 门禁复跑 + 关键发现人工复核。审查日期：2026-10-16。
> 问题编号 R-n 在修复后请更新本文"状态"列（待修 → 已修@commit）。

## 结论

实现质量高，六任务验收标准全部满足且证据扎实，PR 声明经复跑全部属实，无 P0（丢分/安全/数据损坏级）。**建议修复 R-1（P1）与 R-2~R-6（P2）后开 PR 合入 master**；P3 项可延后。红线核查全 PASS（零禁用 import、无学号/凭据/PDF/新依赖）。

## 门禁复跑记录（2026-10-16，审查者独立执行）

| 验证 | 结果 |
|---|---|
| `bash scripts/docker.sh check` | ✅ EXIT=0 |
| `make backend-race` + `make backend-e2e` | ✅ EXIT=0 |
| `--user 1000:1000` 跑 engine/fsmeta/walk/cmd/cli 五包 | ✅ 全过（root 用例如预期 SKIP） |

客户端 Playwright e2e 未单独复跑（W3 diff 未触碰 `client/`，风险可忽略）。

## 问题清单

### R-1 [P1] 重复的目录条目被静默容忍，违背"同一路径重复视为损坏"的状态：已修@989195f

`backend/internal/engine/restore.go:139-141`：目录条目 `Mkdir` 遇 `EEXIST` 且路径已是目录时按成功处理（`err = nil`，注释"已作为缺失的父目录自动创建过"），该目录再次入 `x.dirs`、`dirCount++`。而普通文件（O_EXCL）、软链接、硬链接、特殊节点的重复均正确报 `E_CORRUPT`（restore.go:161-166）。

合法归档（walk 先序、目录先于子项）不会触发该分支，它只服务于乱序/重复的异常归档——与 ADR-015、CHANGELOG 声明的"同一路径重复视为损坏"直接矛盾。恶意/损坏的**未加密**归档（SHA-256 无密钥可伪造）可借重复目录条目带不同元数据后者覆盖前者，还原"成功"且 Dirs 计数虚高。无越界风险。

修复：为目录做重复检测（如该路径已在 `x.parents` 标记时报 `pack.ErrCorrupt`），保留"ensureParent 补建目录后显式条目到达"的合法容忍（需区分这两种 EEXIST：补建发生在无显式条目时；可只对**同一归档内第二次出现的显式目录条目**报错）。补 TC-ENG-040 目录变体回归。

### R-2 [P2] 同名备份 E_EXISTS 的 TOCTOU：`os.Rename` 静默覆盖 状态：已修@989195f

`backend/internal/engine/engine.go:175-179`：`checkNotExists(final)` 后 `os.Rename(partial, final)`——POSIX rename 对已存在目标是**原子替换**而非报错。两个进程并发备份同名，后完成者静默覆盖先完成者的完整备份（代码注释承认窗口但只做了第二次检查，未堵 rename 覆盖语义）。

修复：`os.Link(partial, final)` 成功后 `os.Remove(partial)`、`Link` 遇 `EEXIST` 即 `ErrExists`；或 `unix.Renameat2(RENAME_NOREPLACE)`（syscall 直接可用，不违反零第三方约束）。注意保持"失败路径清理 .partial"语义与既有测试（TC-ENG-050 等）通过。

### R-3 [P2] corrupt 备份 `outputBytes` 恒为 0，与契约不符 状态：已修@b36489a

`backend/internal/cli/commands.go:385-403`（`backupInfoOf`）：契约 §2.3 说损坏项 name/path/**outputBytes**/createdAt 有意义、§3.4 样例为 `"outputBytes":4096`；实现只在 `!b.Corrupt` 分支填该字段，`repo.Backup.Info` 在 Corrupt 时为零值。基线 M6-2 引入，TC-CLI-310 只断言 `corrupt:true` 将其放过；客户端损坏项将显示 0 字节。

修复（选实现侧）：`repo.Backup` 带出文件大小（如 Corrupt 时仍填 `Info.Size = 文件 stat 大小`），corrupt 分支填 `info.OutputBytes`；TC-CLI-310 补断言损坏项 outputBytes>0。

### R-4 [P2] 契约补遗 #28 的 chown warn 条目实为语义修订，定性应补正 状态：已修@4b97a28

`docs/architecture/cli-contract.md:79`：冻结原文"非 root 跳过 lchown，**输出一条 warn**"（无条件）→ 补遗"遇到第一个属主不同的条目时输出"（有条件）。程序合规（独立契约 PR、字段/事件/退出码未动、TC-ENG-010/011 双向测试），但 PR 自述"只澄清行为"与事实不符。

修复：在 `CHANGELOG.md` 该条目处补一句"此为对冻结文本 warn 触发条件的语义修订（经独立契约 PR）"。

### R-5 [P2] 非 root 设备文件"warn 跳过"是新契约边界，零测试 状态：已修@989195f

`restore.go:156-159` 实现正确（EPERM→warn、不计 files、整体不失败），但全仓无用例锚定。

修复：engine 级用例——craft 含字符设备条目的归档，非 root（`--user 1000:1000`）下还原：断言 warn 恰好一条、`Files` 不含该项、还原整体成功；root 下仍往返成功（已有 TC-ENG-004 覆盖）。放 `failure_linux_test.go` 或新文件，遵循现有 needsRoot 跳过机制的反向（非 root 专属）。

### R-6 [P2] 矩阵测试裸类型断言会 panic 而非测试失败 状态：已修@5e41036

`backend/cmd/databackup/matrix_linux_test.go:168-182` 与 :119：`ev["phase"].(string)`、`ev["files"].(float64)` 等无 ok 判定——事件缺字段时测试进程 panic，报错不可读。

修复：改为 `v, ok := ev["phase"].(string); if !ok { t.Fatalf(...) }` 风格。

### P3（可延后，修复 PR 勿顺手扩大范围）

| # | 位置 | 摘要 |
|---|---|---|
| P3-1 | backup.go:279-281 | pack 中途读错误硬失败（E_IO），与契约"无法读取 warn 跳过"字面不一致——硬失败更安全，建议在 ADR-014 注明"打开失败 warn 跳过、读失败中止" |
| P3-2 | engine.go DryRun | dry-run 统计为扫描口径上界（打开才失败的文件不计入 backup 结果），建议 ADR-014 注明 |
| P3-3 | engine.go:128-137 | 取消恰落在 rename 成功后会误报 E_CANCELLED（文件已生成，重试得 E_EXISTS），窗口极窄 |
| P3-4 | engine.go:227-232 | syncDir 静默吞 fsync 失败，建议至少 warn |
| P3-5 | backup.go 两阶段变化 | 变短补零/变长截断/类型替换/消失四条路径零测试（备份侧最大覆盖缺口，建议 C3 前补） |
| P3-6 | engine.go:21-41 | checkTarget 悬空软链接目标报 E_IO 且信息不友好（`mkdir ...: file exists`） |
| P3-7 | restore.go:181-193 | ensureParent 失败未归 E_CORRUPT（恶意归档分类漂移） |
| P3-8 | walk.go:76 | `seen` 字段注释过期（仍写"非目录且 Nlink>1"） |
| P3-9 | fsmeta_linux.go:104-105 | mkfifo 失败被包成 `Op:"mknod"` 的 PathError，误导 |
| P3-10 | container.go:74-90 | 容器头明文存源目录绝对路径（设计如此，无口令可读，记录在案） |
| P3-11 | 还原全程 | 无解压总量上限/磁盘空间检查，未加密备份可伪造高膨胀流写满磁盘（仅磁盘耗尽，无逃逸） |
| P3-12 | scripts/check-imports.sh:16 | 禁用正则漏 `crypto/sha3`（红线写法是 `crypto/sha*`），建议改 `crypto/sha[0-9a-z]*$` |
| P3-13 | main.go:19 | SIGHUP/SIGQUIT 不在优雅取消内（契约只承诺 SIGINT/SIGTERM，属契约内，记录） |
| P3-14 | password.go:32-39 | 口令 buffer 最多残留 1 字节 `\r`（`defer clear` 之后），可忽略 |
| P3-15 | TC-CLI-310 | list 排序顺序 CLI 层未锁定（repo 单测 TC-REPO-004 已覆盖） |
| P3-16 | matrix :133-136 | `--types f` 的 `IgnoreTimes()` 顺带豁免该 4 组合文件 mtime 校验（其余 28 组合已覆盖） |
| P3-17 | e2e/cancel_test.go:112 | `os.ReadDir` 错误被吞，"无残留"断言可能静默通过 |
| P3-18 | main_test.go:38 | `expect` 命令绕过 `run()`（无信号处理），与 `databackup` 命令不等价 |
| P3-19 | roadmap.md:157 | M6-3 验收措辞"组合矩阵 testscript"与实现（Go 测试经 `cli.Run`）不符，plans 文档已如实 |
| P3-20 | CHANGELOG | 漏记人类可读模式进度行改为 `\r` 原地刷新的行为变化 |
| P3-21 | cli-contract.md:37-38 | `--encrypt` "必须配合 --password-stdin" 与 dry-run "被接受但忽略"内部歧义（实现为 dry-run 不检查配套，合理但未写明） |
| P3-22 | 安全加固 | 还原全程用路径字符串而非 dirfd 锚定，本地并发写权限者可竞态（工业级修法 openat 逐级锚定，课程场景可接受） |
| P3-23 | fsmeta setuid | root 还原**他人提供的**备份可得到 setuid 文件（tar 同款风险；还原自己的备份是正确行为） |

## 验收与声明核实摘要

- 六任务验收全过：M2-5（TC-CLI-300 五要素 / TC-ENG-050、TC-E2E-101 无残留）、M3-1（TC-ENG-003/004，root 用例非 root SKIP）、M3-2（TC-ENG-005/010/012、TC-FSM-030~035）、M4-2（TC-ENG-006 压缩率 0.666 有断言）、M5-5（TC-CLI-301 退出码 3 / TC-ENG-034 篡改 / TC-ENG-008 组合）、M6-3（32 组合矩阵经 `cli.Run`、五要素严格比较、进度单调与总量一致有逐行断言、错误码矩阵齐全 + 直接解析契约文档的双保险测试）。
- PR 声明逐条核对：备份侧、还原侧、CLI 侧、契约侧声明**全部属实**（R-1~R-4 所列偏差除外）；`-count=5` 稳定性声明经竞态窗口分析可信。
- 安全关键点核实成立：先整文件验 HMAC 后落地、路径白名单 + 软链接祖先拒绝、O_EXCL、硬链接目标必须普通文件、口令仅 stdin 且不出现在任何输出、encrypt-then-MAC 属性未被破坏、盐/IV 每次随机。
- 红线：生产代码零禁用 import（grep + `go list` 双重复核）、无学号/凭据/PDF/新依赖。
- walk 修复（硬链接只对普通文件识别）正确，TC-WALK-016 构成有效回归。

## 修复指引

修复 PR 范围 = R-1~R-6（P3 不动，尤其 P3-5 测试缺口与 P3-11/22 加固项另行安排）。约束：不改 CLI 协议与公开签名；测试先行；`make check` + 非 root 五包全过后在 worktree 追加提交（footer `Refs: W3`），更新本表状态列。
