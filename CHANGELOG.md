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
- CLI / JSONL 协议定稿：全部子命令参数、过滤语义、事件与各命令结果字段、错误码与退出码、样例；新增错误码 `E_EXISTS`、`E_INTERNAL`、`E_PASSWORD_REQUIRED`（M1-3）
- 后端各包公开 API 冻结（`fsmeta.FileInfo`、filter、walk、pack、huffman、crypto、container、repo、config、engine）；`cli.Classify`/`ExitCode` 错误映射；协议线上类型 `jsonl.*`（M1-3）
- 客户端共享类型 `src/shared/protocol.ts`、`src/shared/api.ts`；Go 与 TS 两端契约测试直接读取 `cli-contract.md` 比对样例（M1-3）
- `config` 包实现：XDG 配置路径、默认值补齐、校验（绝对路径/已存在须为目录）、原子写入、`config get/set` 所需的全部逻辑（M6-1）

### 变更

- `cli.Run` 增加 `ctx` 与 `stdin` 参数，入口在 SIGINT/SIGTERM 时取消 `ctx`（M1-3）
- 客户端 `src/shared/events.ts` 由 `protocol.ts` 取代（M1-3）

- PDF 改为项目最后（M9-2）一次性生成，移除仓库中的骨架 PDF（M0-6）
