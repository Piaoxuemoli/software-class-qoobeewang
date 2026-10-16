# W3 engine 集成（M2-5 → M3-1 → M3-2 → M4-2 → M5-5 → M6-3）

## 目标

roadmap W3 集成泳道：把已就绪的原语（walk、filter、pack、huffman、crypto、container、repo、config）
串成 `engine.Backup` / `DryRun` / `Restore`，并让 CLI 全部子命令走通真实链路（FR-01~18）。

## 交付切分（与 roadmap 的对应）

原计划六个串行 PR。实施时发现前五项共用同一条流水线，拆开会产生只为中间态存在的
临时代码（如"M2-5 先拒绝 `--compress huffman`、M4-2 再放开"；M2-5 的"五要素一致"验收本身
就需要 M3-2 的元数据恢复），故合并为两个 PR，每个 roadmap 任务仍有独立的验收用例：

| PR | 覆盖任务 | 内容 |
|---|---|---|
| PR-1 `feat/M2-5-engine-pipeline` | M2-5、M3-1、M3-2、M4-2、M5-5 | engine 全流水线、`fsmeta.CreateNode/ApplyMetadata`、CLI 备份/还原 testscript、信号取消 e2e |
| PR-2 `feat/M6-3-full-chain` | M6-3 | list/info/delete/config 全链路、错误码矩阵、过滤 × 压缩 × 加密组合矩阵、`cli.checkName` 收敛到 `repo.ValidateName` |

先例：M2-3/M2-4、M5-1/M5-2 都是一行多 ID 一个 PR。

## 设计

### 备份

```
prepare（校验源、换算排除目录、自动排除仓库、编译过滤器）
  → scan：walk + filter，条目列表保存在内存，发 scan 进度
  → 仓库目录不存在则创建（info 日志）
  → pack：<name>.dbk.partial ← bufio ← container.Writer[AES+HMAC] ← [huffman.Writer] ← pack.Writer
  → fsync → 再次确认同名不存在 → rename → fsync 仓库目录
```

- **scan 结果保存在内存**（每条约 200 字节元数据，不含文件数据）：pack 阶段的总量与归档内容
  一致，硬链接识别也只做一次。两阶段之间文件变化的处理：被删除/不可读/换了类型 → warn 跳过；
  指向被跳过文件的硬链接一并 warn 跳过；变短 → 以零补齐到扫描时的大小并 warn；变长 → 只写到
  扫描时的大小。
- 打开源文件用 `O_NOFOLLOW|O_NONBLOCK` 并 fstat 复核类型：扫描后被换成软链接不会跟随，被换成
  FIFO 不会阻塞。
- `.partial` 权限 0600（备份可能含他人不可读的数据）；任何失败或取消都删除 `.partial`。
- 仓库位于源目录内 → 作为排除目录（契约 §2.1 过滤语义 5）；仓库**就是**源目录 → `E_USAGE`
  （否则会把正在写的 `.partial` 打进自己）。
- 源目录可以是指向目录的软链接：遍历从 `EvalSymlinks` 的结果开始，容器头记录用户给出的绝对路径。

### 还原

```
检查目标（不存在或空目录） → 打开备份 → container.Open（整文件校验，可取消）
  → 创建目标 → [huffman.Reader] → pack.Reader → 逐条重建 → 目录元数据逆序设置
```

- 校验通过前不创建、不写任何文件（错误口令、篡改、缺口令都不会留下目标目录）。
- 按类型重建：目录先以 0700 创建（保证可写入子项），元数据全部还原后**逆先序**设置（子目录先于
  父目录）；普通文件 `O_CREAT|O_EXCL`（不写穿已存在的软链接）；软链接 `symlink`；硬链接 `link`；
  FIFO/套接字/设备 `fsmeta.CreateNode`。
- 元数据：`lchown`（仅 root）→ `chmod`（软链接跳过，chown 之后以保留 setuid/setgid）→
  `utimensat(AT_SYMLINK_NOFOLLOW)`（纳秒，软链接设置自身）。
- 非 root：跳过 chown；**遇到第一个属主/属组与当前用户不同的条目时**发一条
  `非 root 用户，已跳过属主恢复（chown）`。属主全部是自己时结果与源一致，不发无意义的警告。
- 非 root 还原设备文件（mknod 返回 EPERM）→ warn 跳过，不使还原失败。
- 安全：`pack.Reader` 已拒绝绝对路径、`..` 与位于软链接之下的路径；engine 另外拒绝
  "目标不是普通文件的硬链接"（否则可借"硬链接到软链接"在目标外写文件）；同一路径出现两次
  （EEXIST）按 `E_CORRUPT` 处理。
- 取消或中途失败：不回滚，先发 `还原已取消/还原失败，目标目录内容不完整：<target>` 再报错。

### 进度与取消

- 每阶段一个节流器：距上次报告不足 100ms 不发，阶段结束必发一次（ADR-013）。scan 不带总量；
  pack 总量取自 scan、restore 总量取自容器尾部。
- 取消检查点：walk 每个条目、pack/restore 每个条目与每 256 KiB 数据、容器校验的每次读。
  取消时一律返回 `cancelError`（`errors.Is(err, context.Canceled)`，消息"操作已取消"），
  即使下层先报出了 I/O 错误。

### 顺带修复

- `walk`：只对普通文件做硬链接识别。此前 FIFO、软链接等的多个硬链接会被标成 `HardlinkTo`，
  而 MyPack/`FileInfo` 只允许普通文件为硬链接，打包时 `WriteHeader` 报错导致整个备份失败。
  现在它们各自作为独立条目备份（"链接方式"要素对这类罕见情形不保留）。

## 测试计划

| 任务 | 用例 |
|---|---|
| M2-5 | TC-ENG-001/002/009/011~014/020~028/030~036/050~053/060~062，TC-CLI-210~213/224/300/302，TC-E2E-101/102 |
| M3-1 | TC-ENG-003/004，TC-FSM-020~024，TC-WALK-016，TC-ENG-040~043，TC-CLI-300 |
| M3-2 | TC-ENG-005/010/012，TC-FSM-030~035 |
| M4-2 | TC-ENG-006（全类型往返 + 文本压缩率 ≈0.67），TC-ENG-008 |
| M5-5 | TC-ENG-007/008/033/034，TC-CLI-301 |
| M6-3 | TC-CLI-400（过滤 8 种 × 压缩 2 × 加密 2 = 32 组合，经 `cli.Run` 备份→还原，与按过滤语义裁剪后的声明式目录树五要素比较，并逐行校验 JSONL、进度单调与总量）、TC-CLI-310（list/info/delete 全链路，含损坏文件、`.partial` 忽略）、TC-CLI-311（config 仓库全链路）、TC-CLI-312（错误码矩阵）、TC-CLI-313（进度事件形状）、TC-CLI-120~126（名称规则收敛到 `repo.ValidateName`） |

root 才能跑的用例（设备、他人属主）在 Docker 开发镜像（root）中执行；非 root 专属用例
（chown 警告、不可读文件）用 `docker run --user 1000:1000` 与 GitHub CI（非 root）执行。

## 文档同步

CHANGELOG、roadmap W3 行、`architecture/overview.md` ADR-014/015、设计文档 walk 硬链接说明；
契约中实现确定的边界（仓库即源目录、非 root 设备跳过、chown 警告触发条件）另开契约 PR 补写。
