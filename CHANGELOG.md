# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增

- 仓库基建：Go 后端骨架（`databackup --help`、`--version [--json]`）、Electron 客户端骨架（显示后端版本）、Makefile、Docker 开发镜像与文档镜像（M0-2）
- CI（后端 / 客户端 / e2e / 文档）与打 tag 自动发布工作流，PR / Issue 模板（M0-4）
- Markdown + PlantUML → PDF 文档流水线，交付文档按官方模板建立骨架（M0-4）
- AGENTS.md、项目简介、课程要求基线、架构与 CLI 协议草案、路线图、进度计划、git / 测试 / 发布 / 编码 / 文档规范（M0-3）
- 调研报告、波次 + 泳道并行开发设计、客户端视觉规范与交互时序、分区 AGENTS.md、开工前检查清单（M0-6）
- CLI 协议草案补充 `backup --dry-run`、带总量的进度事件、取消语义与退出码 4（M0-6）
- 任务难度分级（🔴/🟡/🟢）与可选任务 C4b：Windows 便携演示包，平台不支持项静默跳过（M0-6）

### 变更

- PDF 改为项目最后（M9-2）一次性生成，移除仓库中的骨架 PDF（M0-6）
