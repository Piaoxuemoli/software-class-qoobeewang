# `.dbk` 容器格式（版本 1）

`.dbk` 是一次备份产出的单个文件（FR-13、FR-17），由 `backend/internal/container` 实现。容器不关心负载内容。负载就是 engine 产生的 MyPack 流（见 `mypack-format.md`），可能经过 Huffman 压缩；容器只负责三件事：

1. **明文头部**：记录备份元信息与 KDF 参数；
2. **明文尾部**：记录统计（ADR-012）；
3. **校验值**：保证完整性，加密时还要认证口令。

格式变化必须同步本文、`container` 包常量与 `backend/internal/container/testdata/*.golden`，并在提交 footer 写 `BREAKING CHANGE:`。

## 1. 总体结构

```
.dbk = 头部 ‖ 负载 ‖ 尾部 ‖ 校验值
头部 = 固定部分（32 字节） ‖ 盐 ‖ IV ‖ 源目录
```

- 头部与尾部**始终是明文**：`list`/`info` 不需要口令，只读文件开头和末尾的 104 字节左右。
- **校验值**（32 字节）覆盖**头部‖负载‖尾部**的全部字节：
  - 加密时为 HMAC-SHA256，先加密后 MAC（encrypt-then-MAC）；
  - 未加密时为 SHA-256。
  
  所以篡改头部的算法标识、迭代次数或尾部统计都会被发现。
- 所有整数为**小端序**。

## 2. 头部

| 偏移 | 长度 | 字段 | 说明 |
|---:|---:|---|---|
| 0 | 4 | `magic` | ASCII `DBAK` |
| 4 | 2 | `version` | 格式版本，当前为 `1`。为 0 视为损坏；大于本程序支持的版本返回 `ErrUnsupportedVersion` |
| 6 | 1 | `compression` | `0` = 无，`1` = Huffman（`huffman` 包分块流） |
| 7 | 1 | `encryption` | `0` = 无，`1` = AES-256-CBC + PKCS#7 |
| 8 | 8 | `createdAt` | 备份创建时间，Unix 秒（有符号） |
| 16 | 4 | `iterations` | PBKDF2 迭代次数。加密时取值 1 ~ 1 000 000（默认 100 000），未加密时为 0 |
| 20 | 1 | `saltLen` | 加密时为 16，否则为 0 |
| 21 | 1 | `ivLen` | 加密时为 16（AES 分组长度），否则为 0 |
| 22 | 2 | 保留 | 必须为 0 |
| 24 | 4 | `sourceLen` | 源目录路径的字节数，不超过 65 536 |
| 28 | 4 | 保留 | 必须为 0 |
| 32 | `saltLen` | `salt` | PBKDF2 盐，由 `crypto/rand` 生成 |
| … | `ivLen` | `iv` | CBC 初始向量，由 `crypto/rand` 生成 |
| … | `sourceLen` | `source` | 备份时源目录的绝对路径（原样字节） |

头部长度为 `32 + saltLen + ivLen + sourceLen`。迭代次数设上限，是为了防止伪造的文件让还原卡在密钥派生上。

## 3. 负载

紧接在头部之后，长度 `payloadLen` 记录在尾部。

- **未加密**：负载就是 engine 写入的字节流本身。
- **加密**：负载是 AES-256-CBC（NIST SP 800-38A §6.2）密文，明文末尾按 PKCS#7 填充到 16 的倍数，所以 `payloadLen` 是 16 的正整数倍（空明文也有一个整块填充）。

**密钥派生**：`K = PBKDF2-HMAC-SHA256(口令, salt, iterations, 64)`（RFC 8018 §5.2）。`K[0:32]` 是 AES-256 密钥，`K[32:64]` 是 HMAC 密钥，两个密钥互相独立。

## 4. 尾部（40 字节）

| 偏移 | 长度 | 字段 | 说明 |
|---:|---:|---|---|
| 0 | 4 | `magic` | ASCII `DBKE` |
| 4 | 4 | 保留 | 必须为 0 |
| 8 | 8 | `payloadLen` | 负载字节数（无符号） |
| 16 | 8 | `files` | 备份的非目录条目数（口径同契约 `backup` 结果） |
| 24 | 8 | `dirs` | 目录数 |
| 32 | 8 | `bytes` | 普通文件数据总字节数（压缩、加密前） |

`files`、`dirs`、`bytes` 必须 ≥ 0。

## 5. 校验值（32 字节）

| 是否加密 | 校验值 |
|---|---|
| 加密 | `HMAC-SHA256(K[32:64], 头部‖负载‖尾部)`（RFC 2104） |
| 未加密 | `SHA-256(头部‖负载‖尾部)`（FIPS 180-4） |

两种校验值都用常量时间比较。

## 6. 读取规则

**`ReadInfo`**：只读头部和文件末尾 72 字节（尾部与校验值），不校验也不解密，供 `list`/`info` 使用。下列任一情况返回 `ErrCorrupt`：

- 魔数不符，或文件长度不足 `32 + 40 + 32` 字节；
- `version` 为 0；
- 算法码未知；
- 保留字段非 0；
- `saltLen`/`ivLen`/`iterations` 与加密标识不一致，或迭代次数超出上限；
- `sourceLen` 超出上限；
- 尾部魔数不符；
- `payloadLen ≠ 文件长度 − 头部长度 − 72`；
- 加密时 `payloadLen` 不是 16 的正整数倍；
- 统计为负。

`version` 大于 1 时返回 `ErrUnsupportedVersion`。这一项在 `version` 字段之后的任何字段被解析之前就判定，所以未来版本可以改变其余布局。

**`Open`**：先完成 `ReadInfo` 的全部检查，再执行下列步骤。

1. 加密且未给口令时返回 `ErrPasswordRequired`，此时还没做任何计算。
2. 从头到尾读一遍文件，计算校验值并与文件末尾 32 字节比较。不符时，加密备份返回 `ErrAuth`（口令错误与篡改无法也不应区分），未加密备份返回 `ErrCorrupt`。
3. 校验通过后才回到负载起点，返回负载明文读取器。所以错误口令或篡改不会让还原写出任何文件。

读取负载期间，负载提前结束或解密后填充非法都返回 `ErrCorrupt`。这只会在校验后文件又被改动时发生；容器不负责防御校验与读取之间的并发修改。

## 7. 示例

未加密、无压缩，`createdAt = 2026-01-02T03:04:05Z`，源目录为 `/src`，负载为 `hello`（5 字节），统计为 1 个文件、1 个目录、5 字节。各段字节如下：

```
头部（36 字节）
  44 42 41 4b                 magic "DBAK"
  01 00                       version 1
  00 00                       compression 0, encryption 0
  a5 35 57 69 00 00 00 00     createdAt 1767323045
  00 00 00 00                 iterations 0
  00 00 00 00                 saltLen 0, ivLen 0, 保留
  04 00 00 00                 sourceLen 4
  00 00 00 00                 保留
  2f 73 72 63                 "/src"
负载（5 字节）
  68 65 6c 6c 6f              "hello"
尾部（40 字节）
  44 42 4b 45 00 00 00 00     magic "DBKE"、保留
  05 00 00 00 00 00 00 00     payloadLen 5
  01 00 … 01 00 … 05 00 …     files 1, dirs 1, bytes 5
校验值（32 字节）
  SHA-256(以上 81 字节)
```

完整字节见 `backend/internal/container/testdata/plain.golden`；加密示例使用固定盐与 IV，见 `encrypted.golden`。
