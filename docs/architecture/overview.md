# 架构概览

> 状态：初版（M0）。M1-2 系统设计时细化为构件图/类图/顺序图，届时本文与 `deliverables/02-系统设计文档.md` 必须保持一致。

## 1. 总体结构

```
┌──────────────────────────── Electron 客户端 (client/) ────────────────────────────┐
│  renderer (React)          preload (contextBridge)          main (Node)            │
│  备份 / 管理 / 设置页面 ──► window.databackup API ──IPC──► BackendRunner            │
└───────────────────────────────────────────────────────────────┬────────────────────┘
                                                                │ spawn（不经 shell）
                                                                │ 参数 = CLI 子命令；口令走 stdin
                                                                │ stdout = JSON Lines 事件流
┌──────────────────────────── Go 后端 (backend/) ───────────────▼────────────────────┐
│ cmd/databackup ─► internal/cli ─► internal/engine (BackupService / RestoreService)  │
│                                     │                                               │
│      walk ─► filter ─► pack ─► huffman ─► crypto ─► container ─► 备份仓库目录(.dbk)  │
│                                     │                                               │
│                         repo / config / fsmeta / jsonl / version                    │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

- 后端是唯一的业务实现，CLI 可独立使用，也是测试的主要入口。
- 客户端只做界面与调用：所有参数翻译成 CLI 参数，所有反馈来自 JSONL 事件。协议见 `cli-contract.md`。

## 2. 后端包划分（`backend/internal/`）

> M1-3 已冻结全部公开签名（实现返回"尚未实现"），用 `go doc ./internal/<包>` 浏览。修改公开签名需上报；`fsmeta.FileInfo` 只能新增字段。

| 包 | 职责 | 主要公开 API | 依赖 |
|---|---|---|---|
| `version` | 版本号（后端唯一来源） | `Version` | — |
| `jsonl` | JSON Lines 输出；协议全部线上类型与错误码常量（与 `cli-contract.md` 一一对应，契约测试校验） | `Emitter`、`Progress`/`Log`/`Error`、`*Result`、`BackupInfo`、`Code*` | — |
| `cli` | 子命令与参数解析、人类可读 / JSON 两种输出、哨兵错误 → 错误码 → 退出码 | `Run(ctx, args, stdin, stdout, stderr)`、`Classify`、`ExitCode`、`ErrUsage` | engine, repo, config, jsonl，及各包哨兵错误 |
| `fsmeta` | 文件信息模型 `FileInfo`（相对路径、类型、权限、uid/gid、大小、a/m/c/btime、软链接目标、硬链接指向、设备号、inode 标识）；读取、特殊节点创建、元数据恢复（Linux） | `FileInfo`、`FileType`、`Lstat`、`CreateNode`、`ApplyMetadata` | syscall |
| `walk` | 先序遍历源目录，产出过滤后的 `FileInfo`；同目录按名字排序；识别硬链接（dev+ino）；不跟随软链接 | `Walk(ctx, root, Options, fn) (Stats, error)` | fsmeta, filter |
| `filter` | 过滤条件（类型、名字正则、时间、尺寸、排除目录、排除用户/组）；结论 Include / Exclude / SkipDir | `Criteria`、`Filter`、`New`、`ErrInvalidCriteria` | fsmeta |
| `pack` | MyPack 归档格式（512 字节头记录、长文件名、校验和、结尾零块），用法同 `archive/tar` | `Writer.WriteHeader/Write/Close`、`Reader.Next/Read`、`ErrCorrupt`、`ErrUnsafePath` | fsmeta |
| `huffman` | 分块流式 Huffman 压缩 / 解压 | `NewWriter`、`NewReader`、`ErrCorrupt` | — |
| `crypto/sha256`、`crypto/hmac`、`crypto/pbkdf2`、`crypto/aes` | 手写密码学原语，签名仿标准库；AES-256-CBC + PKCS#7 流式封装 | `sha256.New`、`hmac.New`、`pbkdf2.Key`、`aes.NewCBCWriter/NewCBCReader` | — |
| `container` | 备份文件 `.dbk`：明文头部（魔数、版本、算法、创建时间、源目录、KDF 参数、盐、IV）+ 负载 + 明文尾部（统计）+ 校验值（加密：HMAC-SHA256；不加密：SHA-256） | `NewWriter`/`Writer.Finish`、`Open`（先校验后读）、`ReadInfo`、`ErrAuth`、`ErrPasswordRequired` | crypto/* |
| `engine` | 编排备份与还原流水线、进度节流、错误归类 | `Backup`、`DryRun`、`Restore`、`Reporter` | 以上全部 |
| `repo` | 备份仓库：扫描目录列出备份（读容器头尾）、查看、删除；名称规则与默认名 | `Repo.List/Info/Delete/Resolve`、`ValidateName`、`DefaultName` | container |
| `config` | 用户配置（仓库路径），存于 `$XDG_CONFIG_HOME/databackup/config.json` | `Load`、`Save`、`DefaultPath`、`Config.Set` | — |
| `testutil` | 测试专用：目录树构造器、按正确性五要素比较目录树、golden 文件工具（M1-5） | `CreateTree`、`AssertTreeEqual` | fsmeta |

依赖方向只能自上而下；原语包（`pack`、`huffman`、`crypto/*`）不得依赖 `engine`/`cli`，保证可独立测试与并行开发。面向用户的错误一律用包级哨兵错误，经 `fmt.Errorf("...: %w", err)` 包装向上传递，由 `cli.Classify` 统一映射（契约 §4）。

## 3. 数据流水线

**备份**：`walk`（先序 + 过滤）→ `pack.Writer` → [`huffman.Writer`] → [AES-256-CBC 加密 + HMAC] → `container` 写入仓库目录 `<名称>.dbk`。

**还原**：读 `container` 头 → [完整校验：加密备份校验 HMAC（错误口令/篡改在此发现），未加密备份校验 SHA-256；先校验后解密，校验通过前不写任何文件] → [解密] → [`huffman.Reader`] → `pack.Reader` → 按类型重建文件 → 子项全部还原后再设置目录元数据。

要点：

- 全程流式 `io.Writer` / `io.Reader` 串联，大文件不整体读入内存。
- 先压缩后加密（密文不可压缩）。
- 还原目标目录必须不存在或为空；归档内路径一律相对，拒绝绝对路径与 `..`。
- 非 root 用户还原时跳过 `chown` 并输出 warn 事件，不视为失败。
- "创建时间"过滤：Linux 下优先用 `statx` 的 btime，不可用时退化为 ctime（状态变更时间），在需求说明书中注明。

## 4. 客户端结构（`client/src/`）

| 目录 | 职责 |
|---|---|
| `main/` | 无边框窗口与窗口控制；`backend/runner.ts` 子进程调用与 JSONL 解析、取消；`backend/jobs.ts` 任务注册表；`ipc/` zod 校验的处理器；目录选择对话框 |
| `preload/` | `contextBridge` 暴露类型化的 `window.databackup` |
| `renderer/` | React + Tailwind v4 + shadcn/ui；页面：备份（编排卡 + 6 个筛选弹层 + 进度卡）、备份管理、设置；zustand 状态（任务状态机、备份列表、设置）；React Bits 动效组件 |
| `shared/` | 主进程与渲染进程共享的类型（请求、事件、错误码），与 CLI 协议一一对应 |

详细设计：视觉 `../client/ui-design.md`，进程职责、IPC 接口、状态机与时序 `../client/interaction.md`。

后端路径解析顺序：环境变量 `DATABACKUP_BIN` → 打包后 `resources/bin/databackup` → 开发时 `../backend/bin/databackup`。

安全：`contextIsolation`、`sandbox` 开启，`nodeIntegration` 关闭；IPC 入参 zod 校验；子进程不经 shell；口令只通过 stdin 传递，不出现在命令行参数和日志中。

## 5. 架构决策记录

| ADR | 决策 |
|---|---|
| ADR-001 | 后端 Go（仅标准库），客户端 Electron + React + TypeScript，后端以 CLI + JSONL 与客户端通信 |
| ADR-002 | 课程资料仅存本地 `course/`，仓库只含软件、文档与工程设施；旧历史归档不推送 |
| ADR-003 | 开发/门禁在 Docker 开发镜像，演示在 WSL2 + WSLg，不使用云服务器 |
| ADR-004 | 扩展功能全部自研，禁用 import 名单由脚本强制 |
| ADR-005 | 不使用数据库：备份列表通过扫描仓库目录读取容器头获得，避免索引与实际文件不一致 |
| ADR-006 | 客户端界面：Tailwind v4 + shadcn/ui 为基础组件，React Bits 只用于状态变化时刻；Grok 式纯黑单色视觉 |
| ADR-007 | 窗口 `frame: false` + 自绘标题栏与窗口按钮，不使用 `titleBarOverlay`（Electron 44 在 Linux 上存在 WCO 按钮不显示的问题，electron#52576） |
| ADR-008 | 生产代码零第三方依赖；测试允许引入 testscript（CLI 黑盒测试），其他测试依赖需在 PR 中说明 |
| ADR-009 | 备份写入 `.partial` 后原子改名；取消时删除 `.partial`，退出码 4 |
| ADR-010 | PDF 只在项目最后（M9-2）一次性生成，开发期不提交 PDF |
| ADR-011 | `cli-contract.md` 是协议唯一来源：其中 ` ```jsonl ` 样例由 Go 与 TS 契约测试直接读取比对，文档、Go 类型、TS 类型三者不一致即测试失败 |
| ADR-012 | 容器统计（files/dirs/bytes）写在负载之后的明文尾部，头尾都不加密：流式写入时统计最后才知道；`list`/`info` 无需口令。校验值覆盖头部‖负载‖尾部，还原前完整校验 |
| ADR-013 | 进度节流统一在 `engine` 实现（每阶段 ≤10 次/秒，阶段结束必发），`walk` 只做遍历 |

新决策追加到本表；需要展开论证时写 `docs/architecture/adr/NNN-<slug>.md`。
