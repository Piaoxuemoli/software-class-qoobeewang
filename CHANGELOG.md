# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

（v1.0.0 发布后的新增条目记在这里）

- 交接与沉淀文档：`docs/project/pitfalls.md` 踩坑手册（环境/CI 发布/协作三类，现象→根因→对策→预防，含发布三连坑与截图基线环境锁定等根因分析）；`docs/project/handover.md` 交接文档（状态快照、操作速查、剩余任务）；AGENTS.md 必读表挂入两文档入口

## [1.0.0] - 2026-10-17

首个正式版。Linux 目录树备份软件：备份为单个 .dbk 文件并原样还原。支持七种文件类型与完整元数据、六维过滤，MyPack/Huffman/AES-256 全部自研（生产代码零第三方依赖），配 Electron 桌面客户端。

- 备份/还原全链路：过滤 × 压缩 × 加密 32 组合矩阵验证
- 核心包测试覆盖率 87%~100%，race 与 fuzz 检测干净
- 打包产物：AppImage、deb（内置后端）、独立 CLI
- 全套文档 PDF 与答辩 PPT 随 Release 资产发布

安装与使用见 README；WSL 演示环境见 docs/client/wsl-guide.md。开发过程的逐条变更见 git 历史与各 PR。
