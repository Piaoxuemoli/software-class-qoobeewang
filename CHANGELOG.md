# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

（v1.0.0 发布后的新增条目记在这里）

- 交接与沉淀文档：`docs/project/pitfalls.md` 踩坑手册（环境/CI 发布/协作三类，现象→根因→对策→预防，含发布三连坑与截图基线环境锁定等根因分析）；`docs/project/handover.md` 交接文档（状态快照、操作速查、剩余任务）；AGENTS.md 必读表挂入两文档入口

## [1.0.0] - 2026-10-17

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
- `fsmeta.Lstat` 实现（M2-1）：七种文件类型、权限位（含 setuid/setgid/sticky）、uid/gid、纳秒 a/m/ctime、glibc dev_t 拆主次设备号、软链接目标；创建时间经原始 `statx(2)` 调用（amd64=332、arm64=291）读 btime，不支持时留零值
- `walk` 包实现（M2-1）：先序遍历、同目录按名字字节序稳定排序、不跟随软链接、Include/Exclude/SkipDir 三种过滤结论、按 (Dev,Ino) 识别硬链接（第一次被过滤则以下一次保留为准）、`OnError` 跳过或中止、ctx 取消
- `repo` 包实现（M6-2）：扫描仓库目录经 `container.ReadInfo` 列出 `.dbk`（损坏文件以 `Corrupt` 列出、按文件修改时间参与排序、不影响其他项），`Info`/`Delete`/`Resolve`/`ValidateName`/`DefaultName`（源目录名为 `.` 开头去前导点、超 200 字节按 UTF-8 边界截短）
- 三个页面接入 `window.databackup`（C2）：备份页编排卡（源目录、六个筛选胶囊、dry-run 预览、压缩/加密开关）+ 进度/完成/错误/取消状态卡；备份管理页列表（搜索、详情 Sheet、还原面板、长按删除、损坏项标记）；设置页仓库目录 + 减少动效 + 版本信息。`jobStore` 实现 interaction.md §3 状态机（含加密还原的口令态与 3 次失败回退），`backupsStore` 做列表缓存；口令提交后立即清除、只经 stdin 传给后端
- 删除过渡接口 `getBackendVersion` / `backend:getVersion` 通道，版本信息统一走 `app.versions()`（C2）
- UI 打磨调研与方案（提案 C5）：同心圆角体系、表面/边框令牌微调、低饱和强调色方案 A/B、全套动效清单、版本号与侧栏信息降噪、交互与验收走查线；见 `docs/client/ui-polish.md`
- `engine` 备份/预览/还原流水线（M2-5、M3-1、M3-2、M4-2、M5-5）：`databackup backup`/`restore`/`backup --dry-run` 真正可用。scan（遍历+过滤，结果存内存）→ pack（MyPack → 可选 Huffman → 可选 AES-256-CBC+HMAC → `.dbk.partial` → fsync → 原子改名）；还原先整文件校验、通过前不写任何文件，再按类型重建全部 7 种文件类型与硬链接，恢复权限（含 setuid/setgid/sticky）、属主（root）、纳秒 atime/mtime（软链接设置自身），目录元数据逆先序最后设置；进度每阶段 ≤10 次/秒且阶段结束必发；SIGINT/SIGTERM 取消删除 `.partial`、退出码 4，还原取消先发"目标目录内容不完整"警告
- `fsmeta.CreateNode`（mkfifo/mknod 创建 FIFO、套接字、字符/块设备）与 `fsmeta.ApplyMetadata`（lchown → chmod → utimensat NOFOLLOW）（M3-1、M3-2）
- 还原的安全加固：普通文件 `O_EXCL` 新建、拒绝目标不是普通文件的硬链接（防"硬链接到软链接"逃逸）、同一路径重复视为损坏；备份打开源文件用 `O_NOFOLLOW|O_NONBLOCK` 并复核类型
- 全链路测试（M6-3）：过滤 × 压缩 × 加密 32 组合矩阵（经真实 CLI 入口备份→还原，与裁剪后的期望目录树五要素比较，逐行校验 JSONL 与进度单调/总量），list/info/delete/config 全链路与错误码矩阵 testscript（TC-CLI-310~313）
- testscript 自定义命令 `treeeq`（按五要素比较目录树）与 `testutil.DiffTrees`；备份→还原、压缩+加密、运行期错误码的 CLI 黑盒用例（TC-CLI-300~302），真实二进制收到 SIGTERM 的取消 e2e（TC-E2E-101/102）
- 客户端接真实后端的端到端链路（C3）：`client/e2e/real.spec.ts`（TC-E2E-07~09）——界面完成"过滤+压缩+加密"备份→口令还原→还原树与源树一致（Node fs 断言）、运行中取消且无 `.partial` 残留、错误口令 `E_AUTH` 后重试成功；新增 `e2e/fixtures.ts`（每用例独立临时 HOME/XDG 隔离、`dialog.showOpenDialog` 主进程 stub、失败时自动留 trace 与截图）与 `.devin/skills/databackup-e2e` 排错手册
- Linux 打包可用（C4）：`make package` 产出 `DataBackup-0.1.0.AppImage` 与 `databackup-client_0.1.0_amd64.deb`（内置后端于 `resources/bin/databackup`）；修复 electron-builder 26 的非法 `linux.arch` 键与缺失的 author/maintainer 元数据；WSL2 + WSLg 安装运行说明（含 libfuse2t64 / `--appimage-extract-and-run`、deb 依赖 `libsecret-1-0`、`/tmp` 易失等实测坑）见 `docs/client/wsl-guide.md`
- 软件测试报告成文（M8-1~M8-4）：24 张模板字段用例表（单元/集成/系统/健壮性）+ FR-01~18 ↔ TC 追踪表；覆盖率（核心包 87.2%~100%，全部 ≥80%）、race 全过、fuzz（pack 4,881,324 次 / huffman 40,645 次执行）无 panic；staticcheck / go vet / ESLint 零告警；性能实测（512MB 文本压缩率 0.616、吞吐 ≈97 MB/s、10,000 小文件备份 0.16s）与 pprof 火焰图入 `deliverables/03-软件测试报告.md`；附带修复 `make coverage` html 步骤的 cwd 错误
- 交付文档定稿（M9-1）与演示材料（M9-3、M9-4）：实验报告前置章节（目的/内容/器材）与后置章节（AI 工具总结/实验结论/心得与建议草稿）成文；01/02 经与最终代码的一致性审查修订；新增答辩 PPT 源（15 页）与演示视频脚本（≤3 分钟含录制清单）；deliverables README 登记新文件与 H7 截图占位约定
- 一次性生成全部交付 PDF 与答辩 PPT（M9-2、M9-3）：`docs/pdf/` 收录需求/设计/测试报告、合成《小组实验报告》、项目进度计划与答辩 PPT（15 页 pptx 于 `docs/ppt/`，经视觉验收两轮修正：构件图 padding、emoji 字形、顺序图可读性、强调色一致性）；全部 PlantUML 图渲染为 PNG 入库；仓库版均为占位学号，真实学号提交版由本人本地生成
- v1.0.0 发布配套（M9-5）：README 重写为发布后状态（功能全表、三种获取方式、CLI 示例、文档索引）；新增 `docs/project/human-tasks.md` 剩余人工任务详单（证书/视频/截图/提交版 PDF/提交包组装，含步骤与验收）

### 变更

- `cli.Run` 增加 `ctx` 与 `stdin` 参数，入口在 SIGINT/SIGTERM 时取消 `ctx`（M1-3）
- 客户端 `src/shared/events.ts` 由 `protocol.ts` 取代（M1-3）

- PDF 改为项目最后（M9-2）一次性生成，移除仓库中的骨架 PDF（M0-6）
- 客户端侧栏可用 `Ctrl+B`（macOS `⌘B`）折叠为 64px 图标栏，状态持久化（ui-design.md §4，W1 review）
- cli-contract 补写 W3 实现确定的边界语义（只是澄清，线上格式不变）：仓库目录就是源目录 → `E_USAGE`；非 root 还原只在遇到属主与当前用户不同的条目时发一次 chown 警告（此为对冻结文本 warn 触发条件的语义修订——原文要求无条件输出，经独立契约 PR #28）；非 root 无法创建的设备文件 warn 后跳过；还原写出文件后因错误中止同样先发"目标不完整"警告
- cli-contract 补充 `--name` 缺省名边界规则：源目录名以 `.` 开头去掉全部前导点（去空记 `root`），结果超 200 字节在 UTF-8 字符边界截短（M6-2 复审补遗，实现本已如此）

### 修复

- 真实后端 e2e 截图断言收敛（C3）：源数据夹具改用确定性伪随机（xorshift32）消除逐次漂移；完成卡不再像素级断言"输出文件/压缩率"的具体数值——二者由 Huffman(打包流) 大小推出，流内含 mtime/uid 等环境元数据，实测同一份源数据 root 容器 ≈5508B、uid1000 ≈5519B、CI runner ≈5591B（PR #31 曾以 71px 之差随机挂掉）；改为 mask + 格式断言，精确值由后端 32 组合矩阵（TC-CLI-400）保证
- `--mtime-to`/`--ctime-to` 只写日期时在夏令时切换日不再落到次日（W1 review）
- 现有配置文件损坏或 `repo` 已失效时，`config set` 仍可用来修复（W1 review）
- 过滤条件的正则与用户/组校验只在 `filter.New` 一处实现，CLI 不再重复（W1 review）
- `backup --name ""`（显式给出空名）报 `E_USAGE`，不再静默退化为缺省名；`--name` 规则只在 `repo.ValidateName` 一处实现，删除 CLI 中重复的 `checkName`（M6-3）
- 还原遇同一归档中第二次出现的显式目录条目时报 `E_CORRUPT`（此前静默容忍并重复计数，与"同一路径重复视为损坏"矛盾）；ensureParent 补建的父目录再遇显式条目仍合法（W3 复审 R-1）
- 两个并发同名备份完成时后者不再静默覆盖前者：发布步骤由 `os.Rename` 改为 link+remove，`EEXIST` → `E_EXISTS`（W3 复审 R-2）
- `list`/`info` 中损坏备份的 `outputBytes` 填文件实际大小而非 0（契约 §2.3）（W3 复审 R-3）
- `walk` 只对普通文件识别硬链接：FIFO、软链接等的多个硬链接此前被标为 `HardlinkTo`，而 MyPack 只允许普通文件为硬链接，会使整个备份失败；现在各自作为独立条目备份（M2-5 集成时发现）
