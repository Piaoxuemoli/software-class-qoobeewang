# 交接 prompt（复制到新会话使用）

> 当前节点：M1-3 接口冻结已完成（强模型）。下面这份交给**执行模型**完成第 1 批 🟢/🟡 任务；
> 全部合入后进入下一个强模型节点（M1-5、M2-3/M2-4、M5-1~3、C1）。

```
你是本仓库的开发 agent。仓库根：C:\Users\Qoobeewang\Desktop\软件工程实验
GitHub：Piaoxuemoli/software-class-qoobeewang，主干 master（分支保护：走 PR + CI 必过）。

开工前必读（按顺序）：
AGENTS.md → docs/project/overview.md → docs/project/course-requirements.md
→ docs/roadmap.md（§1 并行设计、§1.4 难度分级）→ docs/architecture/overview.md（§2 包表、§5 ADR）
→ docs/architecture/cli-contract.md（已冻结，协议唯一来源）→ docs/conventions/*.md
→ docs/plans/M1-3-interface-freeze.md（冻结时的语义决策）。
做客户端任务再读 docs/client/ui-design.md 和 docs/client/interaction.md。
后端公开 API 用 `go doc` 看：bash scripts/docker.sh shell 后 cd backend && go doc -all ./internal/<包>。

当前状态：M0、M1-3 已合入。M1-3 冻结了 CLI/JSONL 协议、后端全部包的公开签名（函数体返回"尚未实现"）、
fsmeta.FileInfo、客户端 src/shared/{protocol,api}.ts，并有两端契约测试读取 cli-contract.md 比对。

本批任务（每个任务一个 worktree、一个分支、一个 PR，可并行，同时不超过 5 个）：

| ID | 级别 | 一句话 | 触碰范围 |
|---|---|---|---|
| M1-4 | 🟢 | CLI 框架：全部子命令参数解析/校验/--json 输出/退出码，接到已冻结的 engine/repo/config 签名 | backend/internal/cli/、backend/cmd/、backend/go.mod/go.sum |
| M6-1 | 🟢 | config 包实现 | backend/internal/config/ |
| M2-2 | 🟡 | filter 包实现 | backend/internal/filter/ |
| M4-1 | 🟡 | huffman 包实现 | backend/internal/huffman/ |
| C0   | 🟡 | 客户端底座（Tailwind v4 + shadcn/ui、设计令牌、无边框窗口、侧栏、三页路由、减少动效、截图基线） | client/（除 src/shared/） |
| M1-1 | 🟢 | 需求分析说明书初稿 | docs/deliverables/01-*.md、docs/deliverables/diagrams/ |
| M1-2 | 🟢 | 系统设计文档初稿 | docs/deliverables/02-*.md、docs/deliverables/diagrams/ |

各任务要点（与冻结接口相关，必须遵守）：

M1-4 CLI 框架
- 入口签名已定：cli.Run(ctx, args, stdin, stdout, stderr) int。错误一律 fmt.Errorf("...: %w", cli.ErrUsage)
  或下层哨兵错误，统一经 cli.Classify → cli.ExitCode 输出；--json 时最后一行是 jsonl.Error/某个 *Result。
- 参数语义逐条按 cli-contract.md §1、§2 实现：--flag v 与 --flag=v；--json 可在子命令前或后；时间只写日期时
  from=00:00:00、to=23:59:59.999999999（本地时区）；--encrypt aes256 与 --password-stdin 必须配套；
  --dry-run 不读口令；口令读到第一个 \n 或 EOF、去 \r、1~1024 字节；仓库目录 --repo → 配置 → ~/DataBackup。
- 调用 engine.Backup/DryRun/Restore、repo.*、config.*（目前返回"尚未实现"，会被 Classify 归为 E_INTERNAL，
  这是预期）。把 engine.Progress/Reporter 适配到 jsonl.Progress/Log 的代码写在 cli 包里。
- 引入 testscript（github.com/rogpeppe/go-internal，测试依赖，精确版本），txtar 放
  backend/cmd/databackup/testdata/script/。每个子命令的每类参数错误至少一个 txtar：退出码 2 且 stdout 最后一行
  是 {"type":"error","code":"E_USAGE",...}。用例名带 TC-CLI-xxx。
- cli_test.go 里断言 "(planned)" 的测试会随子命令落地而失效：可以改成断言新的用法文本，PR 中说明原因；
  其他已有测试不得删除或放宽。不得修改 internal/jsonl 的类型。

M6-1 config
- 严格实现 config.go 中已有签名与注释：DefaultPath（XDG_CONFIG_HOME → HOME/.config）、Default（~/DataBackup）、
  Load（文件不存在 → 默认值，缺字段补默认）、Save（校验 + 同目录临时文件 + 原子改名 + 建父目录）、
  Validate（绝对路径；已存在必须是目录；错误包装 ErrInvalid）、Set（未知 KEY → ErrInvalid）。
- 测试用 t.Setenv 设置 HOME/XDG_CONFIG_HOME 到 t.TempDir()；并发 Save 不产生损坏文件（-race）。用例 TC-CFG-xxx。

M2-2 filter
- 语义以 cli-contract.md §2.1"过滤语义"为准：ExcludeDirs 命中 → SkipDir；其他目录：Types 含 d（或 Types 为空）
  → Include，否则 Exclude（仍会被遍历进入）；名字/时间/尺寸/用户组不作用于目录；尺寸只作用于普通文件；
  时间区间闭区间、零值不限；创建时间用 FileInfo.CreateTime()；名字正则只匹配 path.Base(Path)。
- 用户/组名用 os/user 解析为数字 ID（标准库允许）；数字字符串直接当 ID；名字不存在、正则非法、
  下界>上界 → 包装 ErrInvalidCriteria。每个维度命中/不命中/边界各 ≥1 例，用例 TC-FILT-xxx。
- 不需要真实文件系统：直接构造 fsmeta.FileInfo 测试。

M4-1 huffman
- 实现 huffman.go 已有签名：按 BlockSize 分块，每块规范 Huffman 码（码长表写入块头），Close 写结束标记；
  Reader 遇到任何非法输入返回包装 ErrCorrupt 的错误，绝不 panic、不死循环。块格式写在包注释里。
- 测试：空输入、单字节、全同字节（单符号码）、随机、跨块大数据往返；FuzzDecode（go test -fuzz=FuzzDecode
  -fuzztime=30s 不 panic）；Benchmark 吞吐与压缩率。用例 TC-HUFF-xxx。禁止使用 compress/*。

C0 客户端底座
- 不得修改 client/src/shared/。C0 只实现 DataBackupApi 中的 window.*（main 侧 window.ts + IPC，preload 暴露
  window 命名空间），保留现有 getBackendVersion 以免破坏现有界面；其余 API 由后续 C1 实现。
- package.json 一次性加入可预见的全部界面依赖（含 zustand、zod、sonner 等，C1/C2 不再改 package.json）；
  版本精确锁定、发布满 7 天，PR 列出理由。renderer 需要引用 src/shared 时把它加入 tsconfig.web.json 的 include。
- 截图基线只在 Docker 开发镜像中生成（bash scripts/docker.sh make e2e）。

M1-1 / M1-2 文档
- 过滤、还原、错误语义只能引用 cli-contract.md（已冻结），不要另行发明；M1-2 的类图/类描述以
  docs/architecture/overview.md §2 包表和各包 go doc 为准，至少 4 个顺序图（备份、还原加密备份、dry-run 预览、
  删除/列表），一个备份任务状态图（与 interaction.md §3 一致），并说明"无数据库，以 .dbk 文件头代替"。
- 只写 Markdown 与 PlantUML（.puml）源；不运行文档构建、不生成 PDF/PNG（M9-2 统一渲染）。
- 署名只写"组长：王科瑾"，不写学号。

通用工作方式：
1. git fetch && git worktree add ../databackup-wt/<ID> -b <type>/<ID>-<slug> origin/master
2. 先写测试再实现；只改触碰范围内的文件。需要改冻结接口（cli-contract、jsonl 类型、src/shared、
   任何包的公开签名、fsmeta.FileInfo 已有字段）或 engine 时：停下，在 PR 描述"需上报"一节写清原因，不要自行修改。
3. 验收：bash scripts/docker.sh check（后端核心包再跑 bash scripts/docker.sh check-all）。Windows 主机上 bash
   要用 Git Bash（"C:\Program Files\Git\bin\bash.exe"），系统默认的 bash 是 WSL。
4. Conventional Commits，footer 写 Refs: <ID>；更新 CHANGELOG [Unreleased]；只改 roadmap.md 中自己那行状态。
5. 本 prompt 授权：推送任务分支、开 PR、CI 全绿后 squash 合并。禁止推 master、force push master、打 tag、发布。
6. 硬约束：后端生产代码只用 Go 标准库（check-imports 强制）；口令只走 stdin；Electron 安全三件套不改；
   course/ 与学号永不入库；PDF 只在 M9-2 生成。

全部 7 个任务合入后，报告：每个 PR 链接、验收命令输出摘要、任何"需上报"项。然后交回强模型做下一节点：
M1-5 测试夹具、M2-3→M2-4 MyPack、M5-1→M5-2 / M5-3 密码学、C1 客户端调用层。
```
