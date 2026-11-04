# UML 图源与复核记录

> 交付文档与答辩用图。源是 PlantUML（`*.puml`），PNG 由 `bash scripts/docker.sh docs` 统一渲染。
> 课程要求里"用 StarUML 复核"一项，按本人指示不安装 StarUML，改用同一模型重绘并导出图集：
> `course/05-UML/UML图集.pdf`（本地，gitignore）+ 本目录 PNG。

| 图 | 源 | 类型 | 用途 | 复核要点 |
|---|---|---|---|---|
| 用例图 | `use-cases.puml` | 用例图 | 需求文档 §2.2 | UC-01 ~ 07；过滤是 include；口令与 dry-run 预览是 extend |
| 后端类图 | `class-backend.puml` | 类图 | 设计文档 §3 | 基数和组合关系在源里；只加大了 nodesep/ranksep。A4 上是否认得出，要等重新渲染后看 |
| 顶层构件图 | `component-top.puml` | 构件图 | 设计文档 §2、PPT P4 | 客户端三进程 + CLI 的边界；JSONL 走 stdout |
| 后端构件图 | `component-backend.puml` | 构件图 | 设计文档 §2 | CLI → engine → 各包的单向依赖 |
| 客户端构件图 | `component-client.puml` | 构件图 | 设计文档 §2 | contextBridge 白名单；main 才 spawn 后端 |
| 备份顺序图 | `seq-backup.puml` | 顺序图 | 设计文档 §4 | 进度和结果经 CLI stdout JSONL，再由 main 解析 |
| 还原顺序图 | `seq-restore-encrypted.puml` | 顺序图 | 设计文档 §4 | 非法路径在 alt 里中止；不是每条都拒绝 |
| dry-run 顺序图 | `seq-dry-run.puml` | 顺序图 | 设计文档 §4 | 回程同样经 CLI stdout |
| 列表/删除顺序图 | `seq-list-delete.puml` | 顺序图 | 设计文档 §4 | info 读头尾；删除为长按约 1.2 秒 |
| 任务状态图 | `state-backup-job.puml` | 状态图 | 设计文档 §4.2、交互规范 | 含还原的 AwaitingPassword，与 jobStore 一致 |

图集：`course/05-UML/UML图集.pdf`（A4，10 图 + 图注）。重新生成：

```bash
# PNG：docs 流水线会顺带重渲染全部 *.puml
bash scripts/docker.sh docs
# 图集：用 Edge/Chrome 打印 docs/images/uml/atlas.html
msedge --headless=new --no-pdf-header-footer --print-to-pdf=course/05-UML/UML图集.pdf \
  "file:///<repo>/docs/images/uml/atlas.html"
```
