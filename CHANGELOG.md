# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增

- 仓库基建：Go 后端骨架（`databackup --help`、`--version [--json]`）、Electron 客户端骨架（显示后端版本）、Makefile、Docker 开发镜像与文档镜像（M0-2）
- CI（后端 / 客户端 / e2e / 文档）与打 tag 自动发布工作流，PR / Issue 模板（M0-4）
- Markdown + PlantUML → PDF 文档流水线，交付文档按官方模板建立骨架（M0-4）
- AGENTS.md、项目简介、课程要求基线、架构与 CLI 协议草案、路线图、进度计划、git / 测试 / 发布 / 编码 / 文档规范（M0-3）
- 调研报告、波次 + 泳道并行开发设计、客户端视觉规范与交互时序、分区 AGENTS.md、开工前检查清单（M0-6）
- CLI 协议草案补充 `backup --dry-run`、带总量的进度事件、取消语义与退出码 4（M0-6）
- 任务难度分级（🔴/🟡/🟢）与可选任务 C4b：Windows 便携演示包，平台不支持项静默跳过（M0-6）
- CLI / JSONL 协议定稿：全部子命令参数、过滤语义、事件与各命令结果字段、错误码与退出码、样例；新增错误码 `E_EXISTS`、`E_INTERNAL`、`E_PASSWORD_REQUIRED`（M1-3）
- 后端各包公开 API 冻结（`fsmeta.FileInfo`、filter、walk、pack、huffman、crypto、container、repo、config、engine）；`cli.Classify`/`ExitCode` 错误映射；协议线上类型 `jsonl.*`（M1-3）
- 客户端共享类型 `src/shared/protocol.ts`、`src/shared/api.ts`；Go 与 TS 两端契约测试直接读取 `cli-contract.md` 比对样例（M1-3）
- `config` 包实现：XDG 配置路径、默认值补齐、校验（绝对路径/已存在须为目录）、原子写入、`config get/set` 所需的全部逻辑（M6-1）
- `filter` 包实现：类型、名字正则、修改/创建时间、尺寸、排除目录、排除用户/组六维过滤条件编译与判定（M2-2）
- `huffman` 包实现：1MiB 分块流式规范 Huffman 编解码，块头含码长表，结束块标记流尾；解码对非法输入返回 `ErrCorrupt`（M4-1）
- CLI 框架落地：全部子命令参数解析/校验（`--flag v` 与 `--flag=v`、可重复参数、`--` 分隔）、`--json` 可在子命令前后、错误统一经 `cli.Classify`/`ExitCode` 映射退出码、`--password-stdin` 口令读取规则、`engine.Reporter`→`jsonl` 事件适配；testscript txtar 覆盖每类参数错误（M1-4）
- 需求分析说明书初稿：功能结构图、系统用例图、UC-01~07 用例描述表、外部接口与非功能需求、人员分工（M1-1）
- 系统设计文档初稿：顶层/后端/客户端构件图、后端类图与类描述、4 张顺序图（备份/还原加密/dry-run/列表删除）、备份任务状态图、无数据库与 .dbk 格式说明（M1-2）
- 客户端底座（C0）：Tailwind v4 + shadcn/ui 组件（设计令牌映射、本地字体）、无边框窗口与自绘标题栏（最小化/最大化/关闭、最大化状态同步、双击拖拽区切换）、220px 侧栏与三页路由骨架、应用内"减少动效"开关（叠加 `prefers-reduced-motion`）、Sonner 通知、三页空闲态 Playwright 截图基线
- 测试夹具 `internal/testutil`（M1-5）：声明式目录树（文件/目录/软硬链接/FIFO/套接字/设备，权限/属主/纳秒时间）、`CreateTree`、按正确性五要素比较的 `AssertTreeEqual`（每个要素有反例测试）、`Golden` 黄金文件工具
- MyPack 归档格式 v1 与 `pack` 读写器（M2-3、M2-4）：512 字节小端二进制头块（魔数、CRC-32、纳秒时间、全部七种类型）、长路径/链接目标溢出到扩展块、两个零块结尾；Reader 严格校验并拒绝绝对路径、`..` 以及位于已出现软链接之下的路径；格式规范见 `docs/architecture/mypack-format.md`
- 手写 SHA-256（FIPS 180-4）、HMAC-SHA256（RFC 2104）与 PBKDF2-HMAC-SHA256（RFC 8018）（M5-1、M5-2）：公开测试向量全过，并与标准库随机对照
- 手写 AES-256（FIPS-197，S 盒按数学定义生成）与 CBC + PKCS#7 流式加解密（SP 800-38A）（M5-3）：固定 4 KiB 缓冲，填充错误统一返回 `ErrPadding`
- `.dbk` 容器格式 v1 与 `container` 包（M5-4）：明文头部（魔数 `DBAK`、算法、创建时间、源目录、PBKDF2 参数、随机盐与 IV）+ 负载 + 明文尾部统计 + 校验值；加密时 encrypt-then-MAC（HMAC-SHA256），未加密时 SHA-256；`Open` 先完整校验再返回明文流，篡改任一字节、截断、错误口令都会被检出；格式规范见 `docs/architecture/container-format.md`
- 客户端后端调用层（C1）：runner（不经 shell 的 spawn、逐行 zod 校验 JSONL、按退出码兜底错误码、口令经 stdin 写入后立即关闭、取消先 SIGTERM 3 秒后 SIGKILL）、任务注册表（备份/还原互斥返回 `E_BUSY`，新的 dry-run 取消旧的）、`window.databackup` 全部 IPC 通道（入参 zod 校验）、mock 后端（`DATABACKUP_MOCK=1`）；窗口控制 IPC 只注册一次，有任务运行时关闭窗口需确认

### 变更

- `cli.Run` 增加 `ctx` 与 `stdin` 参数，入口在 SIGINT/SIGTERM 时取消 `ctx`（M1-3）
- 客户端 `src/shared/events.ts` 由 `protocol.ts` 取代（M1-3）

- PDF 改为项目最后（M9-2）一次性生成，移除仓库中的骨架 PDF（M0-6）
- 客户端侧栏可用 `Ctrl+B`（macOS `⌘B`）折叠为 64px 图标栏，状态持久化（ui-design.md §4，W1 review）

### 修复

- `--mtime-to`/`--ctime-to` 只写日期时在夏令时切换日不再落到次日（W1 review）
- 现有配置文件损坏或 `repo` 已失效时，`config set` 仍可用来修复（W1 review）
- 过滤条件的正则与用户/组校验只在 `filter.New` 一处实现，CLI 不再重复（W1 review）
