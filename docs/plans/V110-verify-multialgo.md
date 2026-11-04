# V110 临时版本需求：备份验证 + 多算法加密

> 状态：✅ 已完成（待容器环境补跑 `bash scripts/docker.sh check-all` 与 race）。分支 `feat/v110-verify-multialgo`（worktree `../databackup-wt/V110`），不直接落在 master。
> 需求来源：用户 2026-11-04 口头下达的"临时版本需求"，属 post-1.0.1 增量，合入后拟发 v1.1.0。

## 1. 需求与拆解

| 任务 | 内容 |
|---|---|
| V110-1 备份验证 | 新增 `verify` 子命令：不还原的前提下证明一个 .dbk 可完整还原（完整性校验 + 负载全量走查 + 统计交叉核对） |
| V110-2 多算法加密 | 加密算法从单一 AES-256-CBC 扩展为可选：新增自研 ChaCha20（RFC 8439），`backup --encrypt` 增加 `chacha20` 取值 |

两项都涉及冻结接口（CLI 契约、container 格式）与 `engine` 包；按 AGENTS.md 应"停下上报"，本计划即上报产物，改动方式全部为**只增不改**（契约 §5），旧备份文件与旧客户端行为不变。

## 2. 方案设计

### 2.1 多算法加密（V110-2）

**为什么选 ChaCha20**：候选有 AES-CTR（同算法换模式，"多算法"成色不足）、SM4（国密，S 盒实现量大）、Twofish/Serpent（实现量过大）。ChaCha20（RFC 8439）是 TLS 1.3 与 SSH 的标准算法之一，ARX 结构无 S 盒、实现约 150 行且无查表时序泄漏，RFC 自带完整测试向量，流式加密无填充（顺带消掉 CBC 的填充处理分支）。密钥派生、MAC、错误语义全部复用现有 AES 路径。

**格式扩展**（`container-format.md` §2/§3，字节布局不变，只新增枚举值）：

- `encryption` 字段新增 `2 = ChaCha20`；
- `ivLen` 按算法取值：AES-256-CBC 16（不变），ChaCha20 为 12（RFC 8439 nonce），IV 字段复用存 nonce；
- `payloadLen` 约束：CBC 仍要求 16 的正整数倍；ChaCha20 为流密码，密文长 = 明文长，仅要求 > 0（pack 结束标记保证合法备份负载非空）；
- 密钥派生不变：`K = PBKDF2-HMAC-SHA256(口令, salt, iterations, 64)`，`K[0:32]` 加密密钥（AES 或 ChaCha20 通用）、`K[32:64]` HMAC 密钥；校验值仍是 encrypt-then-MAC 的 HMAC-SHA256(头部‖密文‖尾部)。

**代码落点**：

- 新包 `backend/internal/crypto/chacha20`：手写 RFC 8439 §2.3 ChaCha20（256-bit key / 96-bit nonce / 32-bit block counter），对外 `NewWriter(w, key, nonce)` / `NewReader(r, key, nonce)`，接口形态对齐 `aes.CBCWriter/CBCReader`（含 Close、粘滞错误、固定 4 KiB 级冲、ErrKeySize/ErrNonceSize）。
- `container`：`Encryption` 枚举加 `EncryptionChaCha20`；`NewWriter`/`Open`/`parseFixed` 按算法分支（加密层抽象为一个 `io.WriteCloser`/`io.Reader` 局部变量，不引入新接口类型）；`readInfo` 的分组整数倍检查限定到 CBC。
- `engine.BackupOptions`：新增 `Encryption container.Encryption` 字段（`Password` 非 nil 时生效；缺省 0=None 保持旧行为），`backup` 不再硬编码 AES。
- `cli`：`--encrypt` 取值 `none|aes256|chacha20`；`aes256` 名称保持不变（契约已发布）。

**兼容性**：枚举追加不影响旧文件（旧文件 encryption=0/1）；`--encrypt aes256` 语义不变；客户端 `encrypt: boolean` 请求路径不变。

### 2.2 备份验证（V110-1）

**语义**：`verify` = "这个备份现在还原会不会成功"。三层，全过才算通过：

1. **结构**：头部 + 尾部可解析（`ReadInfo`，E_CORRUPT）；
2. **完整性**：全文件 HMAC（加密，需口令 → E_AUTH / E_PASSWORD_REQUIRED）或 SHA-256（未加密，E_CORRUPT），复用 `container.Open`；
3. **可还原性走查**：解密 → 解压（如有）→ `pack.Reader` 逐条目走完整个归档：头记录合法、每个文件的数据长度走读到底、结尾标记到位；把走查得到的 files/dirs/bytes 与容器尾部 Summary **交叉核对**，不符 → E_CORRUPT。不写任何文件。

**接口**（全部为契约新增，"只增不改"）：

- CLI：`databackup verify NAME|PATH [--repo DIR] [--password-stdin]`（NAME/PATH 解析、口令规则与 restore 完全一致，包括"未加密备份给口令 → warn 并忽略"）；
- jsonl：`command:"verify"` 的 result（name/path/files/dirs/bytes/compressed/encryption/ms）、progress 新 phase `"verify"`、`BackupInfo` 增加可选 `encryption` 字段（`"none"|"aes-256-cbc"|"chacha20"`，损坏项缺省）；
- engine：`engine.Verify(ctx, VerifyOptions{Backup, Password}, rep)`，进度复用现有节流机制，取消语义同 restore。

**客户端边界**：本次只改 `client/src/shared/` 的 TS 类型与契约测试（契约 PR 规则要求两端同步），**不做 UI**（避免动截图基线，UI 集成留作后续任务）。

## 3. 测试与验收

- `chacha20`：RFC 8439 §2.3.2 block function 向量、§2.4.2 加密向量、round-trip（含逐字节/4 KiB 边界/空输入）、错误路径。
- `container`：chacha20 加密的写读 round-trip、头部 ivLen=12 解析、非法参数（错 nonce 长度/口令缺失）、旧 AES golden 文件不受影响；verify 相关：篡改密文/尾部/口令错误各返回既定错误。
- `engine`：Verify 对（未加密、AES、ChaCha20）×（huffman、none）组合的通过用例；统计不符（构造 payload 与尾部不一致）→ E_CORRUPT；取消传播。
- `cli`：verify 命令的 JSON/人类输出、错误码映射（E_AUTH/E_PASSWORD_REQUIRED/E_CORRUPT/E_NOT_FOUND）、`--encrypt chacha20` 全链路（备份→verify→还原数据一致）；`--help` 文本。
- 契约：`jsonl/contract_test.go` 与 `client/src/shared/contract.test.ts` 对新增样例全部通过。
- 验收命令：主机 `go test ./...`（Docker 不可用时的替代门禁，注明）+ `bash scripts/check-imports.sh` + 客户端 `npx vitest run src/shared`；Docker 可用后补 `bash scripts/docker.sh check`。

## 5. 执行结果（2026-11-04）

全部按 §2/§3 落地。门禁情况：

| 门禁 | 结果 |
|---|---|
| `gofmt -l` / `go vet`（含 GOOS=linux、GOOS=windows 交叉） | 通过 |
| `staticcheck 2026.2.1` | 本分支改动文件零告警（主机 Windows 分析另有 3 条预存平台性误报，均在未改动文件，Linux CI 不可见） |
| `check-imports.sh` | 通过 |
| `go test ./...`（Windows 主机全量）+ `go test -tags e2e ./e2e/` | 通过 |
| 客户端 `typecheck` / `eslint` / `vitest src/shared/contract.test.ts` | 通过；`api.test.ts` 在本机 vitest worker 无法启动（环境问题，文件未改动） |
| `go test -race` / Docker `check-all` | 本机无 64 位 gcc 与 Docker，待容器环境补跑 |

ChaCha20 额外做了 OpenSSL（Node crypto）独立交叉验证：§2.3.2 块向量与 §2.4.2 全消息向量均与 OpenSSL 逐字节一致。

## 4. 触碰范围

`backend/internal/crypto/chacha20/`（新增）、`backend/internal/{container,engine,cli,jsonl}/`、`backend/cmd`（如需注册命令）、`client/src/shared/`、`docs/architecture/{cli-contract,container-format,overview}.md`、`docs/roadmap.md`、`CHANGELOG.md`、本文件。不触碰 `fsmeta` 公共类型、`pack`/`huffman`/`filter`/`walk`、`testdata` golden 与截图基线。
