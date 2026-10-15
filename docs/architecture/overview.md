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

| 包 | 职责 | 依赖 |
|---|---|---|
| `version` | 版本号（后端唯一来源） | — |
| `jsonl` | JSON Lines 事件输出 | — |
| `cli` | 子命令与参数解析、人类可读 / JSON 两种输出、退出码映射 | engine, repo, config, jsonl |
| `fsmeta` | 文件信息模型 `FileInfo`（类型、相对路径、mode、uid/gid、时间、大小、链接目标、设备号、inode 标识）；`lstat` 读取；元数据恢复（Linux） | syscall |
| `walk` | 先序遍历源目录，产出 `FileInfo` 序列；识别硬链接（dev+ino）；不跟随软链接 | fsmeta, filter |
| `filter` | 过滤条件（类型、名字正则、时间、尺寸、排除目录、排除用户/组）；`Filter` 接口 + 组合 | fsmeta |
| `pack` | MyPack 归档格式 `Writer` / `Reader`（512 字节头记录、长文件名、校验和、结尾零块） | fsmeta |
| `huffman` | 分块流式 Huffman 编码 `io.Writer` / 解码 `io.Reader` | — |
| `crypto/sha256`、`crypto/hmac`、`crypto/pbkdf2`、`crypto/aes` | 手写密码学原语；AES-256-CBC + PKCS#7 流式封装 | — |
| `container` | 备份文件 `.dbk` 容器：头部（魔数、格式版本、算法标识、盐、IV、迭代次数、创建时间、源目录名）+ 负载 + HMAC | crypto/* |
| `engine` | 编排备份与还原流水线、进度事件、错误归类 | 以上全部 |
| `repo` | 备份仓库：扫描目录列出备份（读容器头）、查看、删除 | container |
| `config` | 用户配置（仓库路径等），存于 `$XDG_CONFIG_HOME/databackup/config.json` | — |
| `testutil` | 测试专用：目录树构造器、按正确性五要素比较目录树、golden 文件工具 | — |

依赖方向只能自上而下；原语包（`pack`、`huffman`、`crypto/*`）不得依赖 `engine`/`cli`，保证可独立测试与并行开发。

## 3. 数据流水线

**备份**：`walk`（先序 + 过滤）→ `pack.Writer` → [`huffman.Writer`] → [AES-256-CBC 加密 + HMAC] → `container` 写入仓库目录 `<名称>.dbk`。

**还原**：读 `container` 头 → [校验 HMAC（错误口令/篡改在此发现，先校验后解密）] → [解密] → [`huffman.Reader`] → `pack.Reader` → 按类型重建文件 → 子项全部还原后再设置目录元数据。

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

新决策追加到本表；需要展开论证时写 `docs/architecture/adr/NNN-<slug>.md`。
