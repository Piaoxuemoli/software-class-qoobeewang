# UML 图源与复核记录

> 交付文档与答辩用图。源是 PlantUML（`*.puml`），PNG 由 `bash scripts/docker.sh docs` 统一渲染。
> 课程要求里"用 StarUML 复核"一项，按本人指示不安装 StarUML，改用同一模型重绘并导出图集：
> `course/05-UML/UML图集.pdf`（本地，gitignore）+ 本目录 PNG。

| 图 | 源 | 类型 | 用途 | 复核要点 |
|---|---|---|---|---|
| 用例图 | `use-cases.puml` | 用例图 | 需求文档 §3、PPT P3 | UC-01 ~ 07 与 FR 对应；参与者只有用户与后端 CLI |
| 后端类图 | `class-backend.puml` | 类图 | 设计文档 §3 | 与 `backend/internal/*` 公开签名一致；无数据库类 |
| 顶层构件图 | `component-top.puml` | 构件图 | 设计文档 §2、PPT P4 | 客户端三进程 + CLI 的边界；JSONL 走 stdout |
| 后端构件图 | `component-backend.puml` | 构件图 | 设计文档 §2 | CLI → engine → 各包的单向依赖 |
| 客户端构件图 | `component-client.puml` | 构件图 | 设计文档 §2 | contextBridge 白名单；main 才 spawn 后端 |
| 备份顺序图 | `seq-backup.puml` | 顺序图 | 设计文档 §4 | scan → pack → compress → encrypt → 原子发布 |
| 还原顺序图 | `seq-restore-encrypted.puml` | 顺序图 | 设计文档 §4 | 先校验后落盘；E_AUTH 路径 |
| dry-run 顺序图 | `seq-dry-run.puml` | 顺序图 | 设计文档 §4 | 只遍历与过滤，不写仓库 |
| 列表/删除顺序图 | `seq-list-delete.puml` | 顺序图 | 设计文档 §4 | 只读头尾；损坏项标记 |
| 备份任务状态图 | `state-backup-job.puml` | 状态图 | 设计文档 §5、交互规范 | 9 个状态与 interaction.md §3 一致 |

图集：`course/05-UML/UML图集.pdf`（A4，10 图 + 图注）。重新生成：

```bash
# PNG：docs 流水线会顺带重渲染全部 *.puml
bash scripts/docker.sh docs
# 图集：用 Edge/Chrome 打印 docs/images/uml/atlas.html
msedge --headless=new --no-pdf-header-footer --print-to-pdf=course/05-UML/UML图集.pdf \
  "file:///<repo>/docs/images/uml/atlas.html"
```
