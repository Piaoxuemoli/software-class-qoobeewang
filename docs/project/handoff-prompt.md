# 交接 prompt（复制到新会话使用）

> 节点状态：W2 第 2 批（M2-1、M6-2、C2）已于 2026-10-16 全部合入，本批 prompt 仅作归档留存。
> 当前节点：W3 集成泳道（M2-5 → M3-1 → M3-2 → M4-2 → M5-5 → M6-3，严格串行，engine 归单一 agent）。
> 节奏：agent 两日冲刺，全部代码任务目标 10-18 前完成（见 schedule.md）。

```
你是本仓库的开发 agent。仓库根：C:\Users\Qoobeewang\Desktop\软件工程实验
GitHub：Piaoxuemoli/software-class-qoobeewang，主干 master（分支保护：走 PR + CI 必过）。

开工前必读（按顺序）：
AGENTS.md → backend/AGENTS.md 或 client/AGENTS.md → docs/project/overview.md
→ docs/roadmap.md（§1 并行设计、§2 任务表）→ docs/architecture/overview.md（§2 包表、§5 ADR）
→ docs/architecture/cli-contract.md（已冻结，协议唯一来源）→ docs/conventions/*.md。
做 C2 再读 docs/client/ui-design.md、docs/client/interaction.md（§2.1 IPC 通道与 mock 约定、§3 状态机、§4 时序、§6 错误映射）。
后端公开 API 用 go doc 看：bash scripts/docker.sh shell 后 cd backend && go doc -all ./internal/<包>。

当前状态（均已合入 master）：
- 接口：CLI/JSONL 协议、后端各包公开签名、fsmeta.FileInfo、客户端 src/shared 类型（M1-3）。
- 后端：CLI 框架（M1-4）、config（M6-1）、filter（M2-2）、huffman（M4-1）、testutil 目录树夹具与五要素比较器（M1-5）、
  MyPack 读写（M2-3/M2-4，docs/architecture/mypack-format.md）、手写 SHA-256/HMAC/PBKDF2/AES-CBC（M5-1~3）、
  .dbk 容器（M5-4，docs/architecture/container-format.md）。
- 客户端：底座（C0）、后端调用层与 mock 后端（C1）。
- 文档：需求、设计初稿（M1-1、M1-2）。

本批任务（每个任务一个 worktree、一个分支、一个 PR，三个可同时进行）：

| ID | 级别 | 一句话 | 触碰范围 |
|---|---|---|---|
| M2-1 | 🟡 | walk 遍历 + fsmeta.Lstat 实现 | backend/internal/walk/、backend/internal/fsmeta/（只实现 Lstat，可新增文件） |
| M6-2 | 🟡 | repo 仓库管理：List/Resolve/Info/Delete/ValidateName/DefaultName | backend/internal/repo/ |
| C2   | 🟡 | 三个页面接 mock 后端，状态机与交互测试，截图基线 | client/src/renderer/、client/e2e/；preload 与 main 只允许删除过渡接口 getBackendVersion |

各任务要点（与冻结接口相关，必须遵守）：

M2-1 walk + fsmeta.Lstat
- fsmeta.Lstat(root, rel)（fsmeta_linux.go，签名不变）：
  - 用 syscall.Lstat 取类型、Mode&0o7777、UID/GID、Atime/Mtime/Ctime（纳秒）、Dev/Ino/Nlink。
  - Size 只对普通文件填写，其他类型为 0。软链接用 os.Readlink 填 LinkTarget。
  - 字符/块设备从 Rdev 按 glibc 编码拆出 DevMajor/DevMinor（参考 internal/testutil/create_linux.go 的
    devMajor/devMinor；生产代码不得 import testutil，自己写一份）。
  - Btime 用 statx 读取：syscall 包没有 statx，用 syscall.Syscall6 发起原始系统调用。按架构分文件定义调用号
    （statx_amd64.go: 332，statx_arm64.go: 291），参数为 dirfd=AT_FDCWD(-100)、flags=AT_SYMLINK_NOFOLLOW(0x100)、
    mask=STATX_BTIME(0x800)。
    struct statx 按 include/uapi/linux/stat.h 解析，总长 256 字节：
    - stx_mask u32 在偏移 0；
    - stx_btime 在偏移 80，是 statx_timestamp {i64 sec; u32 nsec; i32 pad}。
    只有返回的 stx_mask 含 STATX_BTIME 时才填 Btime；ENOSYS、EPERM、EINVAL 或不支持时 Btime 留零值，不报错。
    所有常量都要命名，并注明来源。
  - HardlinkTo 不由 Lstat 填。CreateNode/ApplyMetadata 不要实现，它们属于 W3 的 M3-1/M3-2。fsmeta_other.go 不改。
- walk.Walk 严格按 walk.go 包注释和 Options/Stats 字段注释实现：
  - 先序遍历，os.ReadDir 按名字字节序排序，Path 用 "/" 分隔的相对路径，root 本身不输出。
  - 不跟随软链接：指向目录的软链接也只是一个 symlink 条目，所以不会死循环。
  - 过滤结论：
    - Include：调用 fn 并计数，目录继续进入；
    - Exclude：Skipped+1，目录仍然进入（被 --types 排除的目录不记录，但其子项照常判定）；
    - SkipDir：Skipped+1，不进入，也不计其内部。
  - 硬链接：只对"被保留的、非目录且 Nlink>1"的条目按 (Dev,Ino) 记录第一次出现的路径。之后再出现的同一 inode
    填 HardlinkTo=第一次的 Path、Size=0，不计入 Bytes。第一次出现被过滤掉时，下一次被保留的出现就是"第一次"。
  - 错误：Lstat 子项失败，或 ReadDir 某目录失败（这个目录本身已按结论输出过），都调用 OnError(rel, err)：
    返回 nil 则 Errors+1，跳过该项（或该目录的内容），继续遍历；返回非 nil 则中止并返回它。OnError 为 nil 时直接中止。
  - root 本身 Lstat 失败或不是目录时原样返回错误，engine 会在调用前检查并映射错误码。
  - 每处理一个条目检查一次 ctx，取消时返回 ctx.Err()。fn 返回错误则中止并原样返回。
- 测试：
  - 目录树一律用 internal/testutil.CreateTree 构造。用例命名 TC-WALK-xxx（walk）、TC-FSM-xxx（Lstat）。
  - Lstat 覆盖每种类型：FIFO 用 testutil.FIFO；套接字可以用 testutil.Socket；设备只在 root 下测，非 root 时 t.Skip。
    另外覆盖纳秒时间、软链接目标、属主。
  - walk 覆盖：顺序稳定（多次运行结果相同）、目录软链接不死循环、三种过滤结论、硬链接识别（含第一次被过滤）、
    Stats 五个字段、OnError 跳过与中止、ctx 取消、fn 出错。
  - 不可读目录用 chmod 000 构造，root 下 t.Skip（root 能读）。Docker 默认以 root 运行，所以要用非 root 再跑一遍：
    docker run --user 1000:1000 -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOMODCACHE=/tmp/mod ...

M6-2 repo
- 严格实现 repo.go 的签名与注释，仓库内容只通过 container.ReadInfo 读取头尾（ADR-005，无索引）。
- New：filepath.Abs，不要求目录存在。
- List：
  - 只看 Dir 下名字以 ".dbk" 结尾的普通文件（DirEntry.Type().IsRegular()），".dbk.partial"、子目录、其他文件都忽略。
  - 逐个打开并调用 container.ReadInfo。任何错误（解析失败、无权限读取）都以 Corrupt=true 列出，Info 为零值，
    ModTime 取自 stat，不影响其他文件。
  - 排序：按"展示用创建时间"降序，即 Corrupt 时用 ModTime，否则用 Info.Header.CreatedAt；时间相同按 Name 升序。
  - 仓库目录不存在时返回空切片（非 nil），不报错。
- Resolve：
  - 含 "/" 视为路径，filepath.Abs 后原样返回，不检查后缀与存在性。
  - 否则视为名称：去掉可选的 ".dbk" 后缀，用 ValidateName 校验，返回 Dir/<名称>.dbk。
- Info：不存在返回包装 ErrNotFound 的错误；ReadInfo 失败时返回 Corrupt=true 的 Backup 与包装 container.ErrCorrupt 的错误。
  Backup.Name 是文件名去掉 .dbk。
- Delete：
  - Resolve 之后，路径不以 ".dbk" 结尾返回 ErrInvalidName；不存在返回 ErrNotFound；不是普通文件拒绝删除，返回 ErrInvalidName。
  - 损坏的备份也能删除，返回尽量读到的信息。
- ValidateName：1~200 字节，不含 "/" 与 NUL，不以 "." 开头；违反时包装 ErrInvalidName。
  cli 里已有一份同样规则的 checkName，本任务不改 cli，W3 的 M6-3 会统一。
- DefaultName：
  - 源目录名取 filepath.Base(filepath.Clean(source))，"/" 记作 root；时间用 now.In(time.Local).Format("20060102-150405")。
  - 契约没写的两种边界，按下面处理，写进 DefaultName 的文档注释，并在 PR 的"需上报"一节列出，由强模型补进契约：
    - 源目录名以 "." 开头：去掉前导点，去完为空则用 root；
    - 结果超过 200 字节：在 UTF-8 字符边界上截短源目录名部分。
- 测试：TC-REPO-xxx，在 t.TempDir() 里用 container.NewWriter 写真实的 .dbk 构造仓库。要覆盖：
  - 解析失败的文件和空文件都以 Corrupt 列出，其他备份照常列出；
  - .partial、子目录 x.dbk、其他扩展名被忽略；
  - 排序（含时间相同）、目录不存在；
  - Resolve 的名称、带后缀、路径三种情况；
  - Delete 的非 .dbk、不存在、删除损坏备份；
  - ValidateName 边界（200/201 字节、"."、"/"、NUL）；
  - DefaultName（"/"、末尾带 "/"、点开头、超长中文名）。

C2 三个页面
- 按 ui-design.md §5（备份首页、备份管理、设置）、§4 布局、§6 组件选型实现，全部接 mock 后端。
- 状态放 zustand store：jobStore 按 interaction.md §3 状态机，backupsStore 做列表缓存，设置用现有 app store 或新的 settings store。
- 只调用 window.databackup（类型见 src/shared/api.ts）。客户端不做任何业务判断：过滤条件原样转成请求字段，
  合法性由后端以 E_USAGE 返回，界面按 interaction.md §6 展示错误。
- 口令只在提交时传给 backup.start/restore.start，提交后立即从组件/store 状态中清除，不写日志；
  E_AUTH 回到口令输入态，连续 3 次失败回到 Ready（见 02 设计文档 §4）。
- 改用 app.versions() 之后删除过渡接口 getBackendVersion：
  要删的地方有 preload/index.ts、main/ipc/handlers.ts、main/ipc/channels.ts 里的 backendVersion 通道及其测试，
  以及 interaction.md §2.1 表中那一行。这是本任务唯一允许改的 main/preload 内容。
- 不得修改 client/src/shared/、package.json、package-lock.json。需要新的 shadcn 组件时，按 shadcn 源码放进
  src/components/ui/（依赖的 radix 包已在 package.json 中时可以直接用）；缺依赖就停下上报。
- 动效：响应应用内"减少动效"开关与 prefers-reduced-motion；WebGL 背景默认关闭。只用设计令牌，彩色只用于状态。
- 测试：
  - vitest + testing-library 覆盖 interaction.md §3 状态机的全部转移。测试里注入一个假的 window.databackup；
    可以复用或仿照 main/backend/mock.ts 的脚本思路，但渲染层测试不能 import main 代码。
  - Playwright e2e：启动应用时设置环境变量 DATABACKUP_MOCK=1，保证界面流程可复现。至少走通：
    - 一次带过滤、压缩、加密的备份（显示进度后完成）；
    - 取消备份；
    - 还原加密备份时口令错误后重试成功（mock 口令是 mock）；
    - 删除备份。
  - 截图基线：每个页面的空闲态，以及进度卡、完成态、还原 Sheet、口令对话框。
    只在 Docker 开发镜像中生成：先 bash scripts/docker.sh make backend client，再 bash scripts/docker.sh shell，
    在容器里 cd client && xvfb-run -a npx playwright test --update-snapshots。PR 里说明每张基线为何新增或改变。
- 用例命名 TC-UI-xxx。

通用工作方式：
1. git fetch && git worktree add ../databackup-wt/<ID> -b <type>/<ID>-<slug> origin/master
2. 先写测试再实现；只改触碰范围内的文件。需要改冻结接口（cli-contract、jsonl 类型、src/shared、
   任何包的公开签名、fsmeta.FileInfo 已有字段）或 engine 时：停下，在 PR 描述"需上报"一节写清原因，不要自行修改。
3. 验收：bash scripts/docker.sh check；后端核心包再跑 bash scripts/docker.sh check-all。
   Windows 主机上要用 Git Bash（"C:\Program Files\Git\bin\bash.exe"），系统默认的 bash 是 WSL。
   客户端 npm ci 偶发 ECONNRESET 时重试即可（registry 已是 npmmirror）。
4. Conventional Commits，footer 写 Refs: <ID>。PowerShell 下把提交信息写进无 BOM 的 UTF-8 文件，再 git commit -F。
   更新 CHANGELOG [Unreleased]，只改 roadmap.md 中自己那一行的状态。
5. 本 prompt 授权：推送任务分支、开 PR、CI 全绿后 squash 合并。多个 PR 都改 CHANGELOG，后合并的先 rebase 再合并。
   禁止推 master、force push master、打 tag、发布。
6. 硬约束：
   - 后端生产代码只用 Go 标准库（check-imports 强制）；
   - 口令只走 stdin；Electron 安全三件套（contextIsolation、sandbox、nodeIntegration=false）与 frame:false 不改；
   - course/ 与学号永不入库；PDF 只在 M9-2 生成；
   - 不删除、不放宽已有测试；golden 文件与截图基线只在本任务需要时更新，并在 PR 中说明。

三个任务全部合入后，报告：每个 PR 链接、验收命令输出摘要、任何"需上报"项（特别是 M6-2 的 DefaultName 边界）。
然后交回强模型做 W3 集成泳道：M2-5 基础备份还原 → M3-1 特殊文件 → M3-2 元数据 → M4-2 压缩接入 → M5-5 加密接入 → M6-3 全链路。
```
