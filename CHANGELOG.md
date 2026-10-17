# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

（v1.0.0 发布后的新增条目记在这里）

- fix(client)：Tailwind v4 只扫描 `src/renderer`，`src/components/ui` 独占的工具类没有被生成，备份详情 Sheet 落进文档流、口令对话框错位；`styles.css` 增加 `@source '../../'`，筛选胶囊清除按钮与 Sheet 内边距一并修正；8 张页面基线重建
- test(client)：截图基线改为"结构契约 + 范围快照 + 清单"三层（`docs/client/visual-regression.md`）：新增 `layout.spec.ts` 的几何/计算样式断言与 3 张浮层元素截图，整页只保留空闲态；新增 `client/e2e/baselines.md` 清单与 `scripts/check-baseline-manifest.mjs` 门禁，更新基线只走 `make e2e-update BASELINE_REASON=...`
- fix(build)：`electron-builder` 二进制镜像默认值改用 `https://registry.npmmirror.com/-/binary/electron-builder-binaries/`（旧 `npmmirror.com/mirrors` 地址在容器内 TLS 握手失败）
- docs(client)：Windows 便携包按修复后的界面重新构建并验证（推荐在 Windows 宿主机 `npm run package:win`；镜像与验证步骤见 `docs/client/windows-portable.md`）
- 交接与沉淀文档：`docs/project/pitfalls.md` 踩坑手册（环境/CI 发布/协作三类，现象→根因→对策→预防，含发布三连坑与截图基线环境锁定等根因分析）；`docs/project/handover.md` 交接文档（状态快照、操作速查、剩余任务）；AGENTS.md 必读表挂入两文档入口
- C4b（可选）Windows 便携包：`GOOS=windows` 交叉编译后端为 `databackup.exe`（新增 `fsmeta_windows.go` 与 `engine/platform_{linux,windows}.go`，无 uid/gid/btime/设备节点；reparse point 不递归遍历）；平台不支持的元数据/特殊文件静默跳过、仅写本地日志 `%LOCALAPPDATA%\databackup\windows-skip.log`；客户端按平台解析 `bin/databackup[.exe]`，`electron-builder` `portable` 目标产出免安装单文件 exe；使用说明见 `docs/client/windows-portable.md`

## [1.0.0] - 2026-10-17

首个正式版。Linux 目录树备份软件：备份为单个 .dbk 文件并原样还原。支持七种文件类型与完整元数据、六维过滤，MyPack/Huffman/AES-256 全部自研（生产代码零第三方依赖），配 Electron 桌面客户端。

- 备份/还原全链路：过滤 × 压缩 × 加密 32 组合矩阵验证
- 核心包测试覆盖率 87%~100%，race 与 fuzz 检测干净
- 打包产物：AppImage、deb（内置后端）、独立 CLI
- 全套文档 PDF 与答辩 PPT 随 Release 资产发布

安装与使用见 README；WSL 演示环境见 docs/client/wsl-guide.md。开发过程的逐条变更见 git 历史与各 PR。
