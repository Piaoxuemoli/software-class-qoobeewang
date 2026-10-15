# 后端 CLI 与 JSONL 协议

> 状态：**草案**。M0 已实现 `--help`、`--version [--json]`；其余子命令在 M1-3 定稿（定稿后修改需走 PR 并同时更新客户端 `client/src/shared/` 类型与契约测试）。

## 1. 调用形式

```
databackup [--json] <子命令> [参数...]
databackup --version [--json]
databackup --help
```

- `--json`：stdout 只输出 JSON Lines（每行一个 JSON 对象），人类可读日志一律走 stderr。客户端总是使用 `--json`。
- 口令只能通过 `--password-stdin` 从 stdin 读取（读到第一个换行或 EOF），禁止出现在参数中。

## 2. 子命令（草案）

| 子命令 | 参数 | 说明 |
|---|---|---|
| `backup` | `--source DIR` `[--repo DIR]` `[--name NAME]` `[过滤参数]` `[--compress huffman]` `[--encrypt aes256 --password-stdin]` | 备份为 `<repo>/<name>.dbk`；name 缺省为 `<源目录名>-<YYYYMMDD-HHMMSS>` |
| `restore` | `--backup NAME或路径` `--target DIR` `[--repo DIR]` `[--password-stdin]` | target 必须不存在或为空 |
| `list` | `[--repo DIR]` | 列出备份 |
| `info` | `NAME或路径` `[--repo DIR]` | 显示容器头信息 |
| `delete` | `NAME` `[--repo DIR]` | 删除备份 |
| `config` | `get [KEY]` / `set KEY VALUE` | 目前 KEY 只有 `repo` |

过滤参数（对应 FR-05~10，与参考界面 6 个筛选子页一一对应）：

| 参数 | 含义 |
|---|---|
| `--types f,d,l,c,b,s,p` | 只备份所列类型（缺省全部；`f` 表示普通文件） |
| `--exclude-dir PATH` | 排除目录（相对源目录，可重复） |
| `--exclude-name REGEX` | 排除文件名匹配正则的项（Go RE2 语法） |
| `--mtime-from T` `--mtime-to T` | 修改时间区间，`T` 为 `YYYY-MM-DD` 或 `YYYY-MM-DDTHH:MM:SS`（本地时区） |
| `--ctime-from T` `--ctime-to T` | 创建时间区间（btime，不支持时退化为 ctime） |
| `--size-min N` `--size-max N` | 大小区间（字节，闭区间，仅作用于普通文件） |
| `--exclude-user NAME` `--exclude-group NAME` | 排除属主/属组（可重复） |

目录本身不因时间/尺寸/名字过滤被剔除（否则其下文件无处安放），只受类型和排除目录影响——M1-3 定稿时在需求说明书中写明。

## 3. JSONL 事件

所有事件都有 `type` 字段：

| type | 字段 | 说明 |
|---|---|---|
| `progress` | `phase`（`scan`/`pack`/`restore`）、`files`、`bytes`、`current`（相对路径） | 计数单调递增；频率由后端节流（如每 100ms 一次） |
| `log` | `level`（`info`/`warn`/`error`）、`message` | 例如非 root 跳过 chown 时发 warn |
| `result` | `command`，以及命令相关字段 | 成功时最后一行 |
| `error` | `code`、`message` | 失败时最后一行 |

已实现的示例（M0）：

```json
{"type":"result","command":"version","version":"0.1.0"}
```

## 4. 退出码与错误码

| 退出码 | 含义 | 典型 `error.code` |
|---|---|---|
| 0 | 成功 | — |
| 1 | 运行时错误 | `E_IO`、`E_NOT_FOUND`、`E_TARGET_NOT_EMPTY`、`E_CORRUPT` |
| 2 | 用法错误（参数缺失/非法） | `E_USAGE` |
| 3 | 认证失败（口令错误或数据被篡改，HMAC 不通过） | `E_AUTH` |

## 5. 兼容性规则

- 只增不改：新增字段/事件不算破坏；改名、删字段、改语义必须同步修改客户端并在 CHANGELOG 标注。
- 客户端必须忽略未知字段和未知事件类型。
