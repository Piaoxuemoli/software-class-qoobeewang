# C4b Windows 便携包（加分项）

## 目标

roadmap W4 可选任务 C4b：后端 `GOOS=windows` 交叉编译为 `databackup.exe`，经 electron-builder `extraResources` 内置、`portable` 目标产出单文件 exe。

验收标准（roadmap 原文）：**干净 Windows 上双击可跑全部界面流程；平台不支持的元数据/特殊文件静默跳过（不写 warn 事件、不打扰界面，仅记录到本地日志文件），对外呈现为功能完整的演示版**。

触碰范围：`backend/internal/fsmeta/`、`backend/internal/engine/`（roadmap C4b 行明确授权"补 fsmeta/engine 的 windows 变体"）、`backend/cmd/databackup/`、`backend/e2e/`（仅构建标签）、`client/src/main/backend/resolve.ts`、`client/electron-builder.yml`、`client/package.json`、`Makefile`、`docs/`。**不改冻结接口**：FileInfo 字段不动、JSONL 事件不动、cli-contract.md 不动、UI 不动（不连锁截图基线）。

## 调研结论（2026-10-17）

### 现状盘点：代码库已有 `!linux` 桩，编译面只剩一个真问题

| 位置 | 现状 | C4b 处理 |
|---|---|---|
| `fsmeta_other.go`（`!linux`） | Lstat/CreateNode/ApplyMetadata 全返回 `ErrUnsupported` | 新增 `fsmeta_windows.go` 真实现；`fsmeta_other.go` 收窄为 `!linux && !windows` 保留给其他平台 |
| `engine/open_other.go`（`!linux`） | `os.Open` 降级版（无 O_NOFOLLOW） | Windows 沿用即可（Windows 上普通文件打开即普通文件） |
| `engine/restore.go:93-95` | `os.Geteuid()`/`os.Getegid()` **Windows 编译不过** | 抽到平台文件：`ownershipSupported` 常量 + `currentOwnership()` |
| `engine/restore.go:163-167` | 设备 EPERM→warn 跳过 | 抽到平台文件 `createSpecial()`：Linux 保持现语义；Windows 直接静默跳过 |
| `restore.go` 软链接/硬链接 case | 失败即报错中止 | 抽 `symlinkEntry()`/`linkEntry()`：Linux 语义不变；Windows 创建失败→静默跳过（ErrCorrupt/ErrUnsafePath/ErrExist 仍为错误） |
| `applyMeta` 非 root chown warn | `!x.root && uid!=fi.UID` 发 warn | Windows `ownershipSupported=false` 时不发 warn、不调 chown（验收要求静默） |
| `walk.go` / `cli` / `main.go` | `syscall.ENOTDIR/EPERM/SIGTERM` 在 Windows 均有定义，`signal.NotifyContext` 可编译 | 零改动（SIGTERM 在 Windows 不会投递，进程直接被 TerminateProcess 杀，正是验收口径） |
| `repo.List` | 只收 `*.dbk`，`.partial` 已忽略 | 零改动，正好兜住 Windows 取消无优雅期的残留 |
| `cmd/databackup/main_test.go` | 无构建标签，`TestScripts` 在 Windows 上会用空 `scriptCmds` 跑全部 txtar（必失败），且 testscript 的 `HOME=/no-home` 对 Go-on-Windows 无效（读 `USERPROFILE`） | 加 `//go:build linux`；`cmds_other_test.go` 空表保留给其他非 Linux 平台 |
| `e2e/cli_test.go` | `go build -o bin` 无 `.exe` | 加运行时 `.exe` 后缀处理，保持 `-tags e2e` 在 Windows 可编译运行 |
| `client/runner.ts` `cancel()` | `proc.kill('SIGTERM')` | **不改**：Node 在 Windows 上把任何 signal 都实现为 TerminateProcess（无优雅期，`close` 回 `signal:'SIGTERM'` → 已归一为 E_CANCELLED）。SIGKILL 兜底定时器无害 |
| `client/resolve.ts` | 固定解析 `bin/databackup` | 按 `process.platform === 'win32'` 选 `databackup.exe`，打包态与开发态两条路径都改 |
| `electron-builder.yml` | 仅 linux target + linux 后端 extraResources | 加 `win:` 节：`target: portable` + `extraResources: bin/databackup.exe` |
| `Makefile` | 无 windows 目标 | 加 `backend-windows`（容器内交叉编译 `-s -w`） |

### Windows Lstat 设计（`fsmeta_windows.go`）

用 `os.Lstat` + `Sys().(*syscall.Win32FileAttributeData)`，全部标准库，不碰 `golang.org/x/*`（禁令依旧）：

| FileInfo 字段 | Windows 取值 |
|---|---|
| Path | rel（"/" 分隔，与 Linux 一致） |
| Type | `ModeDir`→Dir；`ModeSymlink`→Symlink（`os.Readlink` 取 LinkTarget）；`ModeNamedPipe`→FIFO；`ModeDevice`/`ModeCharDevice`→CharDevice；`ModeSocket`→Socket；regular→Regular；其他→错误走 OnError |
| Mode | regular 基线 `0o666`，目录 `0o755`，symlink `0o777`；`FILE_ATTRIBUTE_READONLY` 时 regular 降为 `0o444`（往返可恢复只读位） |
| UID/GID | `0`（Windows 无此概念；`--exclude-user/group` 按 SID 查表会失败→E_USAGE，数字 ID 语义无意义，文档注明） |
| Size/Atime/Mtime | `st.Size()`；`LastAccessTime`/`LastWriteTime`（`Filetime.Nanoseconds()`，100ns 精度） |
| **Btime** | **Windows 有真创建时间**：`CreationTime`（比 Linux statx 还稳）。CreateTime() 过滤天然可用 |
| Ctime | NTFS 无 change time，退化取 Mtime |
| LinkTarget | `os.Readlink` 原样保存（可能带 `\\?\` 前缀，不解析不清洗） |
| Dev/Ino/Nlink | 尽力而为：`CreateFile(access=0, FILE_FLAG_BACKUP_SEMANTICS)` + `GetFileInformationByHandle`（均在 stdlib `syscall`），失败则 `Nlink=1` 退化为"不识别硬链接"（roadmap 允许放弃；识别成功则 NTFS 硬链接正常去重） |

**junction 防死循环（关键坑预置）**：`os.Lstat` 对 junction（reparse point、非 name-surrogate）可能只报 `ModeDir` 不报 `ModeSymlink`；若当目录递归，像 `%USERPROFILE%\AppData\Local\Application Data` 这类自指 junction 会无限递归。对策：目录带 `FILE_ATTRIBUTE_REPARSE_POINT`（0x400）时尝试 `os.Readlink`，成功则归类 TypeSymlink 不再进入；失败也按 Symlink 处理（LinkTarget 置空）而不是当目录——宁可存成"指向未知的链接"也不冒死循环风险。

**ApplyMetadata**：symlink 直接返回 nil（Windows 无 lutimes）；其余 `os.Chmod`（映射只读位）+ `os.Chtimes`（atime/mtime）。`ApplyOptions.Chown` 被忽略（由 engine 保证 Windows 永不传 true）。Btime/Ctime 无法经 `os` 包设置，不还原（验收口径"无 uid/gid/btime"）。

**CreateNode**：保持 `ErrUnsupported`（Windows 侧不可达——restore 的 `createSpecial` 先行拦截静默跳过）。

### 静默跳过设计（engine，仅 Windows 行为分叉）

- 新增 `engine/platform_linux.go` / `platform_windows.go`：
  - `ownershipSupported`（linux=true / windows=false）+ `currentOwnership() (uid, gid, root)`
  - `createSpecial(p, fi) (skip bool, err error)` / `symlinkEntry` / `linkEntry` 三件套
  - Windows `logSkipped(format, args...)`：追加写 `%LOCALAPPDATA%\databackup\windows-skip.log`（`os.UserCacheDir()`，打开失败静默丢弃；`DATABACKUP_SKIPLOG` 环境变量可覆盖路径，供测试断言）。每行 `时间 | restore | <rel> | <原因>`。
- `extractor.root/uid/gid` 字段保留，由 `currentOwnership()` 填充；`applyMeta` 的 warn 与 `Chown` 都以 `ownershipSupported` 门控 → Windows 完全不发"跳过属主"warn。
- `publishBackup` 不改：`os.Link` 在 NTFS 可用且保留 EEXIST→ErrExists 语义；FAT 系仓库会报"发布备份文件"错误，属可接受限制（文档注明演示要求 NTFS）。
- 取消语义：Windows 无 SIGTERM 优雅期 → TerminateProcess 后 `.partial` 可能残留，`repo.List` 已忽略（契约 §3.5 的 Windows 变体，写进 windows-guide 与 overview）。

### 打包链路

```
Docker 容器: GOOS=windows go build -trimpath -ldflags "-s -w" → backend/bin/databackup.exe
宿主 npm:   client/ npm run package:win = electron-vite build && electron-builder --win portable
产物:       client/release/DataBackup 1.0.0.exe（NSIS 自解压到 %TEMP%，免安装免管理员）
```

- `electron-builder.yml` 的 extraResources **按平台分别配置**（实测顶层+win 合并语义会让 Linux 二进制也进 Windows 包）：`linux.extraResources` → `bin/databackup`，`win.extraResources` → `bin/databackup.exe`。
- `resolve.ts`：`const BIN_NAME = process.platform === 'win32' ? 'databackup.exe' : 'databackup'`；开发态 `../backend/bin/<BIN>` 让宿主上 `electron-vite dev` 也能直连 exe。
- 宿主 Node 是 v25（engines 要求 <23）：npm 只警告 EBADENGINE。electron-builder 26 在 **Docker 容器内**打 win portable 目标可行（portable 走 makensis，不需 wine）；但输出目录不能落在 bind mount（rename EACCES），输出到容器内 /tmp 再 `cp` 回来。
- worktree 的 `client/node_modules` 宿主打包顺序约束（AGENTS 提示）：**先** `bash scripts/docker.sh make client-install`（初始化 Docker 卷）**再**宿主 `npm ci`，避免 Windows 二进制被复制进新建的 Linux 卷。

### Windows 侧测试策略

宿主无 Go，但能执行 .exe：**Docker 内 `GOOS=windows go test -c -o *.test.exe`，宿主 Git Bash 直接跑**。

- `fsmeta_windows_test.go`（新）：Lstat regular/dir/symlink/只读位/时间戳/reparse-point-junction 归类；不用 testutil（linux-only），手写 `os.WriteFile`/`os.Mkdir`/`os.Symlink` 夹具（symlink 失败自动 `t.Skip`）。
- `cmd/databackup` windows 冒烟测试（新，`windows_test.go`）：进程内 `cli.Run` 走 backup→list→info→restore→delete 往返（仅 regular/dir，无 testutil 依赖的手工断言）。
- `GOOS=windows go build ./...` + `go vet ./...` 进 `lint.sh`（编译门禁防退化）。
- testscript 套件保持 linux-only（main_test.go 加标签）。

## 实施顺序

1. 本方案文档（本提交）
2. 后端：`fsmeta_windows.go` + `fsmeta_other.go` 收窄标签；`engine/platform_{linux,windows}.go` + restore.go 改造；`main_test.go`/`e2e` 标签；`lint.sh` 加 GOOS=windows 编译检查；Makefile `backend-windows`
3. Docker 验证：`GOOS=windows go build/vet ./...` 绿；`make check`（Linux 面不回归）
4. Windows 测试文件 + 交叉编译 test exe 宿主执行
5. 宿主 `databackup.exe` CLI 冒烟全链路（含加密压缩过滤、中文名、junction、取消残留）
6. 客户端：resolve.ts、electron-builder.yml、package.json；宿主打 portable 包并人工验证双击流程
7. 文档同步：`overview.md` 平台节、`docs/client/windows-guide.md`、CHANGELOG、roadmap 行、pitfalls（若踩新坑）
8. `bash scripts/docker.sh check` 全绿 → 请示用户后 push/PR

## 风险与检查点

| 风险 | 检查点 | 备选 |
|---|---|---|
| `syscall.GetFileInformationByHandle`/`CreateFile`/`ByHandleFileInformation` 不可用于纯 Go 编译 | 第 3 步编译 | 退化 Nlink=1（放弃硬链接去重，roadmap 允许） |
| junction 归类假设与 Go 版本行为不符 | 第 5 步在宿主对 `AppData\Local\Application Data` 实测 | 增加 reparse-point 特判路径 |
| Node 25 跑 electron-builder 26 失败 | 第 6 步 | 用户在宿主执行单条打包命令；或 WSL+wine（成本高，尽量避免） |
| `win.extraResources` 合并语义不符（exe 未进包） | 第 7 步解开 %TEMP% 自解压目录核实 | 改成顶层 extraResources 双条目（linux 构建需 exe 存在的代价） |
| SmartScreen 拦截 | 人工验证时观察 | 本地构建产物通常无 MOTW 标记不触发；触发属预期行为（未签名），文档说明 |

## 实施结果（2026-10-17）

全部完成，关键证据：

- **编译**：`make backend-windows` → `backend/bin/databackup.exe`（3.8 MB，CGO_ENABLED=0）；`lint.sh` 新增 `GOOS=windows go build/vet ./...` 门禁，Linux/Windows 双平台编译面同时受控。
- **测试**：各包 `go test -c` 交叉编译为 `.test.exe`，宿主真机全部 PASS（含新增 `fsmeta_windows_test.go`、`engine/roundtrip_windows_test.go`、`cmd/databackup/windows_smoke_test.go`）。
- **CLI 冒烟（宿主 Windows）**：备份/还原/list/info/delete、Huffman、AES-256、错误口令 E_AUTH(exit 3)、`.partial` 忽略、400 MiB 流式进度、进程取消（TerminateProcess 后 `.partial` 残留符合预期）、Unicode 名/内容、只读位往返、junction 不递归、符号链接无权限时静默跳过+写 `windows-skip.log`。
- **打包**：`electron-builder --win portable`（容器内 /tmp 输出）→ `client/release/DataBackup 1.0.0.exe`（115 MB）；`resources/bin/` 只含 `databackup.exe`。
- **打包产物真机链路**：Playwright `executablePath` 直驱 `win-unpacked/DataBackup.exe`——窗口打开、状态栏 `后端版本: 1.0.0`（经 IPC 走 `resources/bin/databackup.exe version`）、UI 内 `config.set`+`backup.start`（加密+压缩）→ `pw1.dbk` 落盘 → `restore.start` 还原树逐字节一致 → 错误口令 `E_AUTH` → `repo.list` 正常。宿主机为干净无 Go 的 Windows 11，全程无管理员权限。
- **门禁**：`bash scripts/docker.sh check` 全绿（Linux 面无回归）。
- **新坑**：ELECTRON_RUN_AS_NODE 泄漏、bind mount rename EACCES、junction/TempDir 句柄等已记入 `docs/project/pitfalls.md` §1.8–1.10。
