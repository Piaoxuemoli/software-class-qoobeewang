# M1-3 接口冻结

## 目标

roadmap M1-3：定稿 `docs/architecture/cli-contract.md`；建立后端各包的公开类型与函数签名（实现返回未实现错误）；`fsmeta.FileInfo` 定稿；客户端 `src/shared/` 类型与契约一致。验收：`make check` 通过；`go doc` 可浏览全部公开 API；契约文档、Go 类型、TS 类型三者字段一致（测试比对 JSON 样例）。覆盖 FR-01~18 的接口面。

## 设计

### 1. 三方一致性

- `cli-contract.md` 是唯一来源。文中所有 ` ```jsonl ` 代码块里的每一行都是**契约样例**。
- Go：`internal/jsonl` 定义全部线上类型（事件 + 各命令 result）与错误码常量。`contract_test.go` 读取契约文档，把每条样例按 `type`/`command` 解码到对应 Go 类型（`DisallowUnknownFields`），再编码回去与原文逐字节比较（同时锁定字段名、字段顺序、必选/可选），并检查每个线上类型都至少有一条样例、§4 错误码表与 Go 常量集合相同。
- TS：`src/shared/protocol.ts` 定义同样的类型。`contract.test.ts` 读同一文档，用编译期约束的键表（`{[K in keyof Required<T>]: 'req'|'opt'}`，漏键/多键都编译失败）校验每条样例的键集合与枚举取值，并比对错误码集合。
- 这是测试规范"只读 testdata"的唯一例外（读仓库内契约文档），在 `testing.md` 中注明。

### 2. 关键语义决策（写入契约）

| 决策 | 内容 | 理由 |
|---|---|---|
| 过滤与目录 | 排除目录 → 不记录、不深入；`--types` 不含 `d` → 目录不记录但仍深入；时间/尺寸/名字/用户过滤不作用于目录；还原时缺失的父目录以 0755 自动创建 | 文件必须有处安放；`--types f` 语义直观 |
| 排除目录路径 | 接受相对源目录或位于源目录内的绝对路径（后端换算），源目录外的绝对路径 → `E_USAGE` | 客户端只传对话框选出的绝对路径，不做业务换算 |
| 仓库在源内 | 自动排除仓库目录 | 防止备份自身的 `.partial` |
| 根目录 | 遍历不输出源根本身；归档只含其下内容 | 还原目标目录由用户指定 |
| 不可读项 | 遍历/读取时发 `warn` 跳过，计入 `warnings`，不视为失败 | 健壮性 |
| 还原前校验 | 容器先完整校验（加密：HMAC；不加密：SHA-256），通过后才写任何文件 | 先校验后解密；坏备份不留下半成品 |
| 统计 | `files` = 非目录项数（含链接与特殊文件）；`bytes` = 普通文件原始数据字节（硬链接只算一次） | 进度与结果口径统一 |
| 容器统计放尾部 | 头部（版本、算法、时间、源路径、KDF 参数）+ 负载 + 尾部（files/dirs/bytes）+ 校验值；`list` 读头尾，不校验 | 流式写入时统计数在最后才知道 |
| `list` 含损坏项 | 损坏文件以 `corrupt:true` 列出，便于删除 | FR-14、FR-17 |
| dry-run | `command:"dry-run"`；忽略压缩/加密/名称，不读口令 | 客户端预览无需口令 |
| 新增错误码 | `E_EXISTS`（重名，1）、`E_INTERNAL`（未分类，1）、`E_PASSWORD_REQUIRED`（加密备份未给口令，3） | 错误必须可区分 |
| 进度节流 | 在 `engine` 统一节流（≤10 次/秒，阶段结束必发）；`walk` 只做遍历 | 一处实现 |
| 入口签名 | `cli.Run(ctx, args, stdin, stdout, stderr) int`；`main` 用 `signal.NotifyContext` 处理 SIGINT/SIGTERM | 取消与口令 stdin 需要 |

### 3. 包 API（详见各包 `go doc`）

`fsmeta`（FileInfo/FileType/Lstat/CreateNode/ApplyMetadata）、`filter`（Criteria/Filter/Decision/New）、`walk`（Walk/Options/Stats）、`pack`（Writer/Reader）、`huffman`（Writer/Reader）、`crypto/{sha256,hmac,pbkdf2,aes}`（仿标准库签名 + CBC 流式封装）、`container`（Header/Summary/Info/Writer/Open/ReadInfo）、`repo`（Repo/Backup/List/Info/Delete/Resolve）、`config`（Config/Load/Save/DefaultPath）、`engine`（Backup/DryRun/Restore + Options/Result/Reporter）、`jsonl`（线上类型 + 错误码）、`cli`（Run + ExitCode 映射）。

未实现的函数返回包内 `errNotImplemented`；各任务实现时删除。

## 子任务

- [x] 契约文档定稿（参数、事件、result、错误码、样例）
- [x] Go 各包签名与文档注释
- [x] `jsonl` 线上类型 + 契约测试
- [x] `cli.Run` 新签名、`main` 信号处理
- [x] TS `shared/` 类型 + 契约测试
- [x] 架构文档、testing.md 例外、CHANGELOG

## 测试计划

- `TestContractSamples`（Go）、`contract.test.ts`（TS）：样例 ↔ 类型 ↔ 错误码。
- 原有 cli 测试随签名调整。

## 文档同步

`architecture/cli-contract.md`、`architecture/overview.md`（包表与依赖）、`conventions/testing.md`（契约测试例外）、`CHANGELOG.md`。需求说明书（M1-1）引用契约中"过滤与目录"语义。
