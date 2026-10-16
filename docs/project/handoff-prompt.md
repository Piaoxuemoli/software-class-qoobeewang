# 交接 prompt（复制到新会话使用）

> 节点状态：W2 第 2 批（M2-1、M6-2、C2）已于 2026-10-16 全部合入，W2 批 prompt 不再留存。
> 当前节点：W3 集成泳道（M2-5 → M3-1 → M3-2 → M4-2 → M5-5 → M6-3，严格串行，engine 归单一 agent）。
> 节奏：agent 两日冲刺，全部代码任务目标 10-18 前完成（见 schedule.md）。

```
你是本仓库的开发 agent（强模型，负责 W3 集成泳道全程）。仓库根：C:\Users\Qoobeewang\Desktop\软件工程实验
GitHub：Piaoxuemoli/software-class-qoobeewang，主干 master（分支保护：走 PR + CI 必过）。

开工前必读（按顺序）：
AGENTS.md → backend/AGENTS.md → docs/project/overview.md → docs/roadmap.md（§1 并行设计、W3 任务表）
→ docs/architecture/overview.md（§2 包表、数据流水线、§5 ADR）→ docs/architecture/cli-contract.md
（已冻结，协议唯一来源；§2 参数、§3 事件、§4 错误码）→ docs/conventions/*.md
→ docs/architecture/mypack-format.md（打包格式）→ docs/architecture/container-format.md（容器格式，M5-5 前读）。

当前状态（均已合入 master，CI 绿）：
- 冻结接口：CLI/JSONL 协议、各包公开签名、fsmeta.FileInfo、客户端 src/shared 类型（M1-3）。
- 后端就绪件：cli 框架（M1-4，参数解析/校验/退出码映射已全）、walk 遍历（M2-1）、filter 过滤（M2-2）、
  pack MyPack 读写（M2-3/4）、huffman（M4-1）、手写 SHA/HMAC/PBKDF2/AES-CBC（M5-1~3）、
  container .dbk 容器（M5-4）、repo 仓库管理（M6-2）、config（M6-1）、testutil 目录树夹具（M1-5）。
- engine 还是空壳（M1-3 冻结的签名，实现全部返回 ErrNotImplemented）——W3 就是你的活。
- 客户端：底座 C0、调用层 C1（runner/jobs/zod IPC/mock 后端）、三页面 C2（接 mock）。

泳道规则：六任务严格串行，engine/ 全程只有你一个人改。每任务独立 worktree+分支+PR，
前一个合入后 rebase 再开下一个。顺序与验收：

| # | 任务 | 依赖 | 验收要点 |
|---|---|---|---|
| 1 | M2-5 engine + backup/restore/--dry-run | M1-4, M2-1, M2-2, M2-4 | 只普通文件+目录、无压缩加密；.partial 原子改名；取消处理；testscript 备份→还原五要素一致（FR-01/02）；取消后无残留 |
| 2 | M3-1 特殊文件 | M2-5 | 软/硬链接、FIFO、字符/块设备、套接字往返一致；root 用例非 root 自动跳过 |
| 3 | M3-2 元数据恢复 | M3-1 | 权限、uid/gid（lchown）、atime/mtime（含软链接本身 lutimes）；目录元数据最后设置；非 root chown 跳过并发 warn |
| 4 | M4-2 压缩接入 | M4-1, M3-2 | --compress huffman；全类型往返；记录压缩率 |
| 5 | M5-5 加密接入 | M5-4, M4-2 | --encrypt aes256 --password-stdin；错误口令退出码 3；篡改检测；压缩+加密组合往返 |
| 6 | M6-3 全链路 | M6-1, M6-2, M5-5 | CLI list/info/delete/config 接入 repo/config；进度事件总量+节流；错误码矩阵 testscript 全过；顺手统一 cli.checkName ↔ repo.ValidateName（规则相同，收敛到 repo） |

复审尾巴（接手即知，非阻塞）：
- DefaultName 两种边界已补进契约（"." 开头去前导点、空记 root；超 200 字节 UTF-8 边界截短）；
  engine 生成缺省名直接调 repo.DefaultName。
- 小观察可顺手看：repo.List 对 DT_UNKNOWN+stat 失败的条目是跳过而非 Corrupt；
  客户端 job.ts 在 jobId 回填前 cancel 是空操作（约一次 IPC 窗口）。不急修。
- 已登记的延后契约项：拖拽文件夹需 preload 暴露 webUtils.getPathForFile——本泳道不做，
  留待后续契约 PR（见 docs/plans/C2-client-pages.md）。

通用工作方式：
1. git fetch && git worktree add ../databackup-wt/<ID> -b feat/<ID>-<slug> origin/master
2. 先写测试再实现；验收标准逐条用命令验证贴进 PR。
3. Windows 主机用 Git Bash：bash scripts/docker.sh check（合并前/动 engine 时跑 check-all）。
4. 改冻结接口（cli-contract、src/shared、任何包公开签名、fsmeta.FileInfo 已有字段）→ 停下上报，
   或单独开"契约 PR"（只改契约及两端适配）。
5. Conventional Commits，footer Refs: <ID>；CHANGELOG [Unreleased]；roadmap 只改自己那行。
6. 本 prompt 授权：推送任务分支、开 PR、CI 全绿后 squash 合并；远端分支合并后可删。
   禁止推 master、force push、打 tag、发布 Release。
7. 硬约束：生产代码零第三方依赖（check-imports 强制）；口令只走 stdin；--json 只出 JSONL；
   平台相关代码 //go:build linux；course/ 与学号永不入库；PDF 只在 M9-2 生成。

六任务全部合入后报告：每个 PR 链接、check-all 输出摘要、任何"需上报"项。
然后进入下一个强模型节点：W3 合入复审 → 放行 C3 客户端联调。
```
