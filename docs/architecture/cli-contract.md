# 后端 CLI 与 JSONL 协议

> 状态：**已冻结（M1-3）**。这是客户端与后端之间唯一的接口。修改必须单独开"契约"PR：同时改本文、`backend/internal/jsonl`（线上类型）、`client/src/shared/`（TS 类型），并让两端的契约测试通过（见 §6）。

## 1. 调用形式

```
databackup [--json] <子命令> [参数...]
databackup <子命令> --help
databackup --version [--json]
databackup --help
```

- 参数写法：`--flag value` 或 `--flag=value`；可重复的参数多次给出。未知参数、缺少必选参数、多余的位置参数 → `E_USAGE`（退出码 2）。
- `--json`：可以写在子命令前，也可以写在子命令参数中。开启后 stdout **只**输出 JSON Lines（每行一个 JSON 对象，UTF-8），人类可读文本（用法说明、调试日志）一律走 stderr。客户端总是使用 `--json`。
- 不带 `--json` 时：结果以人类可读文本写 stdout；日志、警告与错误（`databackup: <message>`）写 stderr；用法错误时把用法说明写到 stderr。退出码与 JSON 模式相同。具体文本格式不属于契约。
- 口令只能通过 `--password-stdin` 从 stdin 读取：读到第一个 `\n` 或 EOF 为止，去掉结尾的 `\r`；长度 1~1024 字节，空口令 → `E_USAGE`。口令禁止出现在参数、环境变量和任何输出中。
- 时间参数 `T` 的格式为 `YYYY-MM-DD` 或 `YYYY-MM-DDTHH:MM:SS`，按后端进程的本地时区解释。只写日期时：`*-from` 取当天 00:00:00，`*-to` 取当天 23:59:59.999999999。所有区间都是闭区间。
- 仓库目录（备份文件所在目录）的确定顺序：`--repo DIR` → 配置文件中的 `repo` → 默认值 `~/DataBackup`。备份时仓库目录不存在则自动创建。
- 配置文件：`$XDG_CONFIG_HOME/databackup/config.json`，未设置 `XDG_CONFIG_HOME` 时为 `~/.config/databackup/config.json`。文件不存在时使用默认值。

## 2. 子命令

### 2.1 `backup` —— 备份（FR-01、FR-05~13）

```
databackup [--json] backup --source DIR [--repo DIR] [--name NAME]
           [过滤参数...] [--compress none|huffman] [--encrypt none|aes256 --password-stdin]
           [--dry-run]
```

| 参数 | 说明 |
|---|---|
| `--source DIR` | 必选。要备份的目录（相对路径按当前目录解析）。不存在 → `E_NOT_FOUND`；不是目录 → `E_USAGE` |
| `--name NAME` | 备份名，生成 `<repo>/<NAME>.dbk`。缺省为 `<源目录名>-<YYYYMMDD-HHMMSS>`（本地时间；源为 `/` 时源目录名记作 `root`）。NAME 规则：1~200 字节，不含 `/` 与 NUL，不以 `.` 开头；违反 → `E_USAGE`。同名备份已存在 → `E_EXISTS` |
| `--compress` | `none`（缺省）或 `huffman` |
| `--encrypt` | `none`（缺省）或 `aes256`。`aes256` 必须配合 `--password-stdin`，反之亦然；不配套 → `E_USAGE` |
| `--dry-run` | 只遍历 + 过滤，不写任何文件，返回将要备份的数量与大小（客户端实时预览用）。此时 `--name`/`--compress`/`--encrypt` 被接受但忽略，**不需要也不读取**口令 |

过滤参数（FR-05~10，与参考界面 6 个筛选子页一一对应；多个条件同时满足才被备份）：

| 参数 | 含义 |
|---|---|
| `--types LIST` | 只备份所列类型，逗号分隔：`f` 普通文件、`d` 目录、`l` 软链接、`c` 字符设备、`b` 块设备、`s` 套接字、`p` 命名管道。缺省全部；空列表或未知字母 → `E_USAGE` |
| `--exclude-dir PATH` | 排除目录（可重复）。接受相对源目录的路径，或位于源目录内的绝对路径（后端换算为相对路径）；源目录外的绝对路径或含 `..` → `E_USAGE` |
| `--exclude-name REGEX` | 排除**文件名**（不含目录部分）匹配正则的项（可重复，Go RE2 语法；任一匹配即排除）。正则非法 → `E_USAGE` |
| `--mtime-from T` `--mtime-to T` | 修改时间区间 |
| `--ctime-from T` `--ctime-to T` | 创建时间区间：文件系统支持时用 `statx` 的 btime，否则退化为 ctime（状态变更时间） |
| `--size-min N` `--size-max N` | 大小区间（非负整数，字节），只作用于普通文件 |
| `--exclude-user NAME` `--exclude-group NAME` | 排除属主/属组（可重复）；接受名称或数字 ID，名称不存在 → `E_USAGE` |
| — | 区间下界大于上界 → `E_USAGE` |

过滤语义（M1-1 需求说明书据此编写）：

1. 被 `--exclude-dir` 命中的目录：本身不备份，也**不进入**。
2. 其他目录：总是进入。目录本身是否记录只取决于 `--types` 是否包含 `d`；时间、尺寸、名字、用户/组过滤不作用于目录。还原时缺失的父目录以权限 0755、当前用户自动创建。
3. 非目录项：必须同时通过类型、名字、时间、尺寸（仅普通文件）、用户/组全部条件。
4. 源目录本身不进入归档；归档只包含其下的内容，路径一律相对源目录。
5. 仓库目录若位于源目录内，自动排除。
6. 硬链接：同一 inode 第一次被备份时存数据，之后的路径记为指向它的硬链接；被过滤掉的路径不参与硬链接识别。
7. 无法读取的项（权限不足等）：输出一条 `warn` 日志并跳过，计入 `warnings`，不使备份失败。

写入：先写 `<NAME>.dbk.partial`，成功后原子改名为 `<NAME>.dbk`；失败或取消时删除 `.partial`。

### 2.2 `restore` —— 还原（FR-02~04、FR-17）

```
databackup [--json] restore --backup NAME|PATH --target DIR [--repo DIR] [--password-stdin]
```

| 参数 | 说明 |
|---|---|
| `--backup` | 必选。含 `/` 视为文件路径，否则视为仓库中的备份名（可带或不带 `.dbk` 后缀）。不存在 → `E_NOT_FOUND` |
| `--target DIR` | 必选。必须不存在（自动创建）或为空目录；否则 → `E_TARGET_NOT_EMPTY` |
| `--password-stdin` | 备份已加密时必须提供，否则 → `E_PASSWORD_REQUIRED`；备份未加密时提供则输出 `warn` 并忽略 |

- 写任何文件之前先完整校验备份：加密备份校验 HMAC（口令错误或被篡改 → `E_AUTH`），未加密备份校验 SHA-256（损坏 → `E_CORRUPT`）。
- 归档内出现绝对路径或 `..` 路径 → `E_CORRUPT`，不写出该项之外的任何文件。
- 非 root 用户还原时跳过属主恢复（`lchown`），输出一条 `warn`，不视为失败。目录的元数据在其子项全部还原后再设置。

### 2.3 `list` —— 列出备份（FR-14）

```
databackup [--json] list [--repo DIR]
```

扫描仓库目录中的 `*.dbk`（忽略 `.partial` 和其他文件），读取每个容器的头部与尾部（不校验、不需要口令）。损坏的文件不影响其他项，以 `corrupt: true` 列出（此时只有 `name`、`path`、`outputBytes`、`createdAt`=文件修改时间有意义）。按 `createdAt` 降序、`name` 升序排列。仓库目录不存在时返回空列表。

### 2.4 `info` —— 查看单个备份（FR-14）

```
databackup [--json] info NAME|PATH [--repo DIR]
```

NAME/PATH 解析同 `restore --backup`。不存在 → `E_NOT_FOUND`；损坏 → `E_CORRUPT`。

### 2.5 `delete` —— 删除备份（FR-14）

```
databackup [--json] delete NAME|PATH [--repo DIR]
```

只删除 `.dbk` 文件（路径不以 `.dbk` 结尾 → `E_USAGE`）。不存在 → `E_NOT_FOUND`。损坏的备份也可删除。

### 2.6 `config` —— 配置（FR-15）

```
databackup [--json] config get [KEY]
databackup [--json] config set KEY VALUE
```

- 目前 KEY 只有 `repo`；未知 KEY → `E_USAGE`。
- `set repo DIR`：DIR 必须是绝对路径；若已存在必须是目录；违反 → `E_USAGE`。写配置采用"写临时文件 + 原子改名"。
- JSON 模式下 `get`/`set` 都返回完整配置。

## 3. JSONL 事件

每个事件都有 `type` 字段。`--json` 模式下 stdout 的**最后一行**恰好是一个 `result`（成功，退出码 0）或一个 `error`（失败，退出码非 0）；在它之前可以有任意条 `progress`、`log`。

### 3.1 `progress`

| 字段 | 类型 | 说明 |
|---|---|---|
| `phase` | `"scan"` \| `"pack"` \| `"restore"` | 备份先 `scan`（遍历 + 过滤，得到总量），再 `pack`（打包/压缩/加密一次流式完成）；还原只有 `restore` |
| `files` | 整数 | 已处理的非目录项数（含链接与特殊文件） |
| `bytes` | 整数 | 已处理的普通文件原始数据字节（硬链接只算一次） |
| `totalFiles` `totalBytes` | 整数，可选 | 只在 `pack`、`restore` 阶段出现，即本次的总量（备份取自 scan，还原取自容器尾部） |
| `current` | 字符串 | 当前处理项的相对路径（`/` 分隔） |

计数在同一阶段内单调不减；后端节流为每阶段 ≤10 次/秒，阶段结束时必发一次（此时 `files`、`bytes` 为该阶段最终值）。

```jsonl
{"type":"progress","phase":"scan","files":120,"bytes":5242880,"current":"src/main.go"}
{"type":"progress","phase":"pack","files":60,"bytes":2621440,"totalFiles":120,"totalBytes":5242880,"current":"src/util.go"}
{"type":"progress","phase":"restore","files":120,"bytes":5242880,"totalFiles":120,"totalBytes":5242880,"current":"src/main.go"}
```

### 3.2 `log`

| 字段 | 类型 | 说明 |
|---|---|---|
| `level` | `"info"` \| `"warn"` \| `"error"` | `warn`：跳过不可读项、非 root 跳过 chown、取消后目标目录不完整等 |
| `message` | 字符串 | 中文，面向用户 |

```jsonl
{"type":"log","level":"warn","message":"非 root 用户，已跳过属主恢复（chown）"}
{"type":"log","level":"info","message":"仓库目录不存在，已创建 /home/alice/DataBackup"}
```

### 3.3 `error`

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | 字符串 | 见 §4 |
| `message` | 字符串 | 中文，面向用户，可直接展示 |

```jsonl
{"type":"error","code":"E_AUTH","message":"口令错误或备份已被篡改"}
```

### 3.4 `result`

所有 `result` 都有 `command` 字段区分命令。时间字段为 RFC 3339（秒精度，带本地时区偏移）；`ms` 为耗时毫秒。

| `command` | 字段 |
|---|---|
| `version` | `version` |
| `backup` | `name`、`path`（绝对路径）、`files`、`dirs`、`bytes`（原始数据）、`outputBytes`（备份文件大小）、`compressed`、`encrypted`、`skipped`（被过滤掉的项数，被排除的目录算 1 项且不计其内部）、`warnings`、`ms` |
| `dry-run` | `files`、`dirs`、`bytes`、`skipped` |
| `restore` | `target`（绝对路径）、`files`、`dirs`、`bytes`、`warnings`、`ms` |
| `list` | `repo`（绝对路径）、`backups`：`BackupInfo` 数组 |
| `info` | `BackupInfo` 的全部字段（平铺） |
| `delete` | `name`、`path` |
| `config` | `repo` |

`BackupInfo`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | 字符串 | 文件名去掉 `.dbk` |
| `path` | 字符串 | 绝对路径 |
| `outputBytes` | 整数 | 备份文件大小 |
| `createdAt` | 字符串 | 备份创建时间（容器头）；损坏时为文件修改时间 |
| `source` | 字符串 | 备份时源目录的绝对路径 |
| `compressed` `encrypted` | 布尔 | |
| `files` `dirs` `bytes` | 整数 | 同 `backup` 结果（容器尾部） |
| `formatVersion` | 整数 | 容器格式版本，当前为 1 |
| `corrupt` | 布尔 | 头部或尾部无法解析 |

```jsonl
{"type":"result","command":"version","version":"0.1.0"}
{"type":"result","command":"backup","name":"docs-20261015-235500","path":"/home/alice/DataBackup/docs-20261015-235500.dbk","files":120,"dirs":14,"bytes":5242880,"outputBytes":3170304,"compressed":true,"encrypted":true,"skipped":7,"warnings":0,"ms":1834}
{"type":"result","command":"dry-run","files":120,"dirs":14,"bytes":5242880,"skipped":7}
{"type":"result","command":"restore","target":"/home/alice/restore","files":120,"dirs":14,"bytes":5242880,"warnings":1,"ms":1502}
{"type":"result","command":"list","repo":"/home/alice/DataBackup","backups":[{"name":"docs-20261015-235500","path":"/home/alice/DataBackup/docs-20261015-235500.dbk","outputBytes":3170304,"createdAt":"2026-10-15T23:55:00+08:00","source":"/home/alice/docs","compressed":true,"encrypted":true,"files":120,"dirs":14,"bytes":5242880,"formatVersion":1,"corrupt":false},{"name":"old","path":"/home/alice/DataBackup/old.dbk","outputBytes":4096,"createdAt":"2026-10-01T09:00:00+08:00","source":"","compressed":false,"encrypted":false,"files":0,"dirs":0,"bytes":0,"formatVersion":0,"corrupt":true}]}
{"type":"result","command":"info","name":"docs-20261015-235500","path":"/home/alice/DataBackup/docs-20261015-235500.dbk","outputBytes":3170304,"createdAt":"2026-10-15T23:55:00+08:00","source":"/home/alice/docs","compressed":true,"encrypted":true,"files":120,"dirs":14,"bytes":5242880,"formatVersion":1,"corrupt":false}
{"type":"result","command":"delete","name":"old","path":"/home/alice/DataBackup/old.dbk"}
{"type":"result","command":"config","repo":"/home/alice/DataBackup"}
```

### 3.5 取消

- 后端捕获 SIGINT/SIGTERM：停止处理，删除 `.partial` 文件，输出 `error{code:"E_CANCELLED"}`，退出码 4。
- 还原被取消时已写出的文件不回滚，先输出一条 `warn` 日志说明目标目录内容不完整。
- 客户端发送 SIGTERM 后 3 秒仍未退出则 SIGKILL（此时没有 `error` 事件，客户端按退出码/信号判定为已取消）。

## 4. 退出码与错误码

| 退出码 | 含义 | `error.code` |
|---|---|---|
| 0 | 成功 | — |
| 1 | 运行时错误 | `E_IO`（读写失败）、`E_NOT_FOUND`（源目录或备份不存在）、`E_TARGET_NOT_EMPTY`（还原目录不为空或不是目录）、`E_CORRUPT`（备份格式错误、损坏或含非法路径）、`E_EXISTS`（同名备份已存在）、`E_INTERNAL`（未分类的内部错误） |
| 2 | 用法错误（参数缺失/非法） | `E_USAGE` |
| 3 | 认证失败 | `E_AUTH`（口令错误或数据被篡改，HMAC 不通过）、`E_PASSWORD_REQUIRED`（备份已加密但未提供口令） |
| 4 | 已取消（收到 SIGINT/SIGTERM） | `E_CANCELLED` |

客户端若收到非零退出码却没有 `error` 事件（如被 SIGKILL、后端崩溃），按退出码映射：1 → `E_INTERNAL`，2 → `E_USAGE`，3 → `E_AUTH`，4 或被信号终止 → `E_CANCELLED`。

## 5. 兼容性规则

- 只增不改：新增字段/事件/错误码不算破坏；改名、删字段、改语义必须同步修改客户端并在 CHANGELOG 标注。
- 客户端必须忽略未知字段和未知事件类型；未知错误码按退出码映射处理。

## 6. 契约测试

本文所有 ` ```jsonl ` 代码块中的每一行都是契约样例，由两端测试直接读取本文：

- Go：`backend/internal/jsonl/contract_test.go`——每条样例解码到对应 Go 类型（拒绝未知字段）后重新编码，必须与原文逐字节相同（锁定字段名、顺序与可选性）；每个线上类型至少一条样例；§4 错误码集合与 Go 常量相同。
- TS：`client/src/shared/contract.test.ts`——每条样例的键集合与 TS 类型一致（必选键齐全、无多余键），枚举取值合法；§4 错误码集合与 TS 类型相同。

新增事件或字段时：先改本文样例，再改两端类型，两端测试都通过才算完成。
