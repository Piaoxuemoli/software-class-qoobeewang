# Changelog

本文件记录所有面向用户的变更，格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

- feat(video)：新增演示视频制作工具链 —— `scripts/video/`（IndexTTS-2.5 批量配音、语速测量、ASS 字幕/注解生成、整片音轨、ffmpeg 合成、成片 QC）与 `client/e2e/record-demo/`（Playwright 分段录制、页面内光标层、真实产物捕获）
- docs(video)：完成演示视频 P1~P3 并固化分镜时间轴 —— 分镜/字幕/注解按真实录制动作重排，成片 **2:57.97 / 1920×1080 / 30fps / −16.5 LUFS**，QC 全绿；记录见 `docs/plans/video-{phase1,phase2,qc}.md`；修正 `07-…` 里 S6 的 `diff` 口径（FIFO 还原用 `test -p` 单独校验）
- docs(project)：README/交接/人工任务/进度同步演示视频完成状态；课程最终提交包整理到本地 `course/07-提交物/最终提交包/`（视频与音频仍不入库）

## [1.0.1] - 2026-10-18

界面样式修复、截图基线自动审计与交付文档刷新。建议所有 v1.0.0 使用者升级；Windows 便携包随 Release 提供。

- fix(client)：Tailwind v4 只扫描 `src/renderer`，`src/components/ui` 独占的工具类没有被生成，备份详情 Sheet 落进文档流、口令对话框错位；`styles.css` 增加 `@source '../../'`，筛选胶囊清除按钮与 Sheet 内边距一并修正；8 张页面基线重建
- test(client)：截图基线改为"结构契约 + 范围快照 + 自动审计"三层（`docs/client/visual-regression.md`）：`layout.spec.ts` 断言几何/计算样式，`scripts/visual-audit.mjs` 解 PNG 检查尺寸/背景/内容占比/盖章哈希，`baseline-stamp.mjs` 记录更新原因；不再要求人工逐张核对
- fix(build)：`electron-builder` 二进制镜像默认值改用 `https://registry.npmmirror.com/-/binary/electron-builder-binaries/`（旧 `npmmirror.com/mirrors` 地址在容器内 TLS 握手失败）
- docs(client)：Windows 便携包按修复后的界面重新构建并验证（推荐在 Windows 宿主机 `npm run package:win`；镜像与验证步骤见 `docs/client/windows-portable.md`）
- docs(readme)：用真实后端截图重写「界面」一节（7 张，含备份进行中与设置页），补基线规则入口
- docs(ppt)：答辩 PPT 重做为 14 页（`scripts/docs/pptx-build.mjs` 生成，`docs/deliverables/05-答辩PPT.md` 同步）：白底黑字、图表与实测数据为主，去掉宣传式表述；补充已知限制与代码规模
- docs(report)：实验报告 §7 插入 4 张 AI 工具会话截图（已隐去余额与无关项目名），工具表同步；全部 6 份 PDF 重出
- docs(project)：H7 完成、H11/H12 更新；新增截图基线标准与 Windows 验证记录；H3（看板快照）与 H5（UML 图集）由 agent 完成
- 交接与沉淀文档：`docs/project/pitfalls.md` 踩坑手册（环境/CI 发布/协作三类，现象→根因→对策→预防，含发布三连坑与截图基线环境锁定等根因分析）；`docs/project/handover.md` 交接文档（状态快照、操作速查、剩余任务）；AGENTS.md 必读表挂入两文档入口
- C4b（可选）Windows 便携包：`GOOS=windows` 交叉编译后端为 `databackup.exe`（新增 `fsmeta_windows.go` 与 `engine/platform_{linux,windows}.go`，无 uid/gid/btime/设备节点；reparse point 不递归遍历）；平台不支持的元数据/特殊文件静默跳过、仅写本地日志 `%LOCALAPPDATA%\databackup\windows-skip.log`；客户端按平台解析 `bin/databackup[.exe]`，`electron-builder` `portable` 目标产出免安装单文件 exe；使用说明见 `docs/client/windows-portable.md`

## [1.0.0] - 2026-10-17

首个正式版。Linux 目录树备份软件：备份为单个 .dbk 文件并原样还原。支持七种文件类型与完整元数据、六维过滤，MyPack/Huffman/AES-256 全部自研（生产代码零第三方依赖），配 Electron 桌面客户端。

- 备份/还原全链路：过滤 × 压缩 × 加密 32 组合矩阵验证
- 核心包测试覆盖率 87%~100%，race 与 fuzz 检测干净
- 打包产物：AppImage、deb（内置后端）、独立 CLI
- 全套文档 PDF 与答辩 PPT 随 Release 资产发布

安装与使用见 README；WSL 演示环境见 docs/client/wsl-guide.md。开发过程的逐条变更见 git 历史与各 PR。
