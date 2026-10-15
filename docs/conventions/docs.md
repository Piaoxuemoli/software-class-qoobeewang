# 文档规范

## 1. 文档分类

| 类别 | 位置 | 说明 |
|---|---|---|
| 项目文档 | `docs/project/` | 简介、课程要求基线、进度计划 |
| 架构文档 | `docs/architecture/` | 架构概览、CLI 协议、MyPack/容器格式规范、ADR |
| 规范 | `docs/conventions/` | git、测试、发布、编码、文档 |
| 计划 | `docs/plans/` | 大任务开工前的计划文档 `<任务ID>-<slug>.md` |
| **交付文档** | `docs/deliverables/` | 课程要交的报告源文件（见 `deliverables/README.md`） |
| PDF 备份 | `docs/pdf/` | 由流水线生成并提交 |

## 2. 写作风格

- 中文为主，术语首次出现附英文；先结论后细节，能用表格不用长段落。
- 需求编号 `FR-xx`、用例编号 `UC-xx`、测试编号 `TC-<模块>-xxx`、任务编号 `Mx-y` 全仓库统一，跨文档引用时直接写编号。
- 交付文档严格按官方模板的章节编号与标题（见 `docs/project/course-requirements.md` §5），不得增删一级章节。

## 3. 图

- 一律用 PlantUML 源文件：放在文档同级的 `diagrams/` 目录，`<名称>.puml`。
- 文档构建（`bash scripts/docker.sh docs`）把每个 `.puml` 渲染为同名 `.png`（提交入库，GitHub 上可直接显示）；Markdown 中引用 `.png`。
- 图与文字必须一致：改代码导致设计变化时，同一 PR 内更新图。

## 4. Markdown → PDF

- 清单：`docs/pdf-manifest.tsv`（每行：输出 PDF 名 + 一个或多个源 md）。新增需要 PDF 的文档时在此登记。
- 构建：Windows 主机 `bash scripts/docker.sh docs`；有文档工具链的 Linux 上 `make docs`。流水线：PlantUML → PNG，pandoc → docx（`docs/templates/reference.docx` 中文样式）→ LibreOffice → PDF，输出到 `docs/pdf/`。
- 何时重建并提交 PDF：每个里程碑发布前、交付文档有实质修改的 PR 合入前。CI 的 docs 任务会验证可构建并上传产物。
- 仓库内 PDF 使用占位身份（`docs/deliverables/profile.example.yaml`）。**带真实姓名学号的提交版**：

  ```bash
  DOCS_PROFILE=course/profile.yaml DOCS_OUT=course/07-提交物 bash scripts/docker.sh docs
  ```

  `DOCS_OUT` 必须位于仓库之外或被 gitignore（如 `course/`），脚本会强制检查。

## 5. 文档与代码同步

| 改动 | 必须同步的文档 |
|---|---|
| 新增/修改功能行为 | 需求说明书对应用例、`CHANGELOG.md` |
| 新增/修改模块、类、接口 | 系统设计文档（构件图/类图/类描述）、`architecture/overview.md` |
| CLI 参数或事件变化 | `architecture/cli-contract.md`、客户端共享类型 |
| 归档/容器格式变化 | 对应格式规范文档 + golden 文件 |
| 新增/修改测试用例 | 软件测试报告用例表与追踪表 |
| 计划或状态变化 | `roadmap.md`、`project/schedule.md` |
