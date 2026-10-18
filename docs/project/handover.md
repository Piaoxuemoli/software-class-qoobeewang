# 交接文档（Handover）

> 面向：接手本项目的人（未来的本人、或任何新 agent）。回答四个问题：项目到哪了、东西都在哪、怎么继续干活、还差什么。最后更新：**2026-10-18（演示视频完成后）**。
> 深读顺序：[AGENTS.md](../../AGENTS.md)（全局规则）→ 本文档 → [README](../../README.md) → 按需查[文档索引](../../README.md#文档索引)。

## 1. 项目状态快照

- **一句话**：Linux 目录树备份软件（Go 后端 + Electron 客户端，格式/压缩/加密全自研），《软件开发综合实验》课程项目，组长王科瑾单人组 + AI 辅助开发。
- **里程碑**：v1.0.1 已发布（[Release](https://github.com/Piaoxuemoli/software-class-qoobeewang/releases/tag/v1.0.1)，资产见 Releases 页：PDF×6 / AppImage / deb / CLI 包 / 源码包 / sha256，另附 Windows 便携包）。v1.0.0 是首版；v1.0.1 修复了详情 Sheet/对话框的样式缺陷，并把截图基线改为自动审计。FR-01~18 全部实现并有自动化测试背书。
- **开发模型**：波次+泳道并行（roadmap §1），agent 强度分级调度（🔴🟡🟢），review-agent 循环 + `make check`/`check-all` + CI 四 job 门禁。2026-10-16~17 两日冲刺完成 W3→W5 全部 agent 侧工作（PR #27~#43）。
- **演示视频（H6，2026-10-18 完成）**：P1 配音方案（IndexTTS-2.5 零样本 + AISHELL-1 开源男声）→ P2 分镜固化与 7 段 Playwright 录制 → P3 ASS 字幕/注解合成 + QC。成片 `course/04-视频/DataBackup-demo-1.0.1.mp4`：2:57.97 / 1920×1080 / 30 fps / H.264+AAC / −16.5 LUFS；QC 全绿，**只剩本人上传取链接**（M9-6 需要）。制作文档见 `docs/plans/video-{phase1,phase2,qc}.md`。
- **代码冻结**：10-17 已冻结。10-18 在重拍截图时发现并修复了一个发行版可见的界面缺陷（Tailwind v4 扫描范围 → Sheet/对话框样式缺失），e2e 基线已重建；这带来一个新的决策项 H11（是否发 v1.0.1 重出产物），与 H12（提交版 PDF/PPT 重出）。两个可选项（C5 UI 打磨 🟡、C4b Windows 便携包 近🔴）仍需本人拍板。

## 2. 东西都在哪

| 要找什么 | 位置 |
|---|---|
| 任务与验收标准 | `docs/roadmap.md`（任务 ID 全仓通用：分支/PR/footer） |
| 剩余人工任务（逐项步骤） | `docs/project/human-tasks.md` |
| **踩坑手册（先读，防再犯）** | `docs/project/pitfalls.md` |
| 交付文档（md 源 + PDF 成品） | `docs/deliverables/` + `docs/pdf/` |
| 答辩 PPT（pptx + PDF） | `docs/ppt/答辩PPT.pptx`、`docs/pdf/答辩PPT.pdf` |
| 演示视频脚本 | `docs/deliverables/06-演示视频脚本.md`、`07-演示视频分镜与操作脚本.md` |
| **演示视频制作文档** | `docs/plans/video-phase1.md`（配音方案/试听）、`video-phase2.md`（分镜固化与分段录制）、`video-qc.md`（合成与 QC） |
| **演示视频成片与素材** | 本地 `course/04-视频/`（成片、`voice/` 参考音与旁白、`work/` 录制段与 QC；视频/音频不入库） |
| **视频工具链** | `scripts/video/`（配音合成、语速测量、ASS 生成、整轨混音、合成、QC）、`client/e2e/record-demo/`（Playwright 分段录制） |
| **课程最终提交包（本地）** | `course/07-提交物/最终提交包/`（平台包/邮箱包/演示视频/文档素材/校验） |
| 架构 / CLI 协议 / 格式规范 | `docs/architecture/`（cli-contract.md 是冻结契约） |
| WSL 安装运行指南 | `docs/client/wsl-guide.md` |
| 测试规范与 TC 编号约定 | `docs/conventions/testing.md`（400+ 用例编号内嵌代码可检索） |
| 复审报告留档 | `docs/plans/*-review.md` |
| 性能/测试原始数据 | 测试报告 §3/§4 已汇总；原始输出在 WSL `~/m8-evidence.txt` |

## 3. 怎么继续干活（操作速查）

- **标准任务流**：roadmap 找任务 → worktree + 分支 → 先写测试 → `bash scripts/docker.sh check`（Windows 主机）→ PR → CI 绿 → squash 合并 → 只改自己那行状态。细节见 AGENTS.md §4。
- **再发一版**：改代码 → 常规 PR 合并 → 版本 bump（**按 `pitfalls.md` §2.6 清单**：常量/lock/断言/基线连锁）→ `git tag vX.Y.Z && git push origin vX.Y.Z`。发布流水线已修好可直接用（历史三坑见 pitfalls §2.1~2.3）。
- **跑测试/构建**：`make check` / `check-all` / `package` / `coverage`；Windows 主机一律 `bash scripts/docker.sh <...>`。
- **演示环境**：WSL2 Ubuntu 24.04 已装齐（工具链 + Electron 运行库 + `/opt/DataBackup` 的 deb），按 `wsl-guide.md` 走；`/tmp` 易失、WSL 偶发 E_FAIL 等坑见 pitfalls §1。
- **改交付文档**：改 md 源 → `bash scripts/docker.sh docs` 重新出 PDF（仓库版占位学号）；真实学号版命令在 human-tasks.md §4。

## 4. 还差什么（全部为本人任务）

详见 **`docs/project/human-tasks.md`**（每项含步骤/验收）。**已完成**：H3 看板、H5 UML、H7 AI 截图、H10 WSL 环境、H11 发布、H12 提交版 PDF/PPT，以及 **H6 成片（仅剩上传）**；**待本人**：H6 上传取链接 → H2 证书（24h 审核，最先启动）→ H8 答辩彩排（11-16 前）→ H9 签到；**M9-6 两个提交包已由 agent 整理到 `course/07-提交物/最终提交包/`**，填链接/上传/发邮件即可。

## 5. 给未来 agent 的提醒

1. AGENTS.md 的硬性约束依然全部有效（零第三方依赖、无学号、PDF 只在发布型节点生成、master 保护）。push/合并曾在 2026-10-17 获本人一次性授权（"你来合并/发布"）；**新会话如无明确指示，先确认该授权是否延续**。
2. 开工前读 `pitfalls.md`——尤其反斜杠转义、截图基线环境锁定、版本 bump 连锁、WSL 三坑，都是真实代价换来的。
3. 冻结后改动要克制：任何 UI 改动会连锁截图基线，任何契约改动要走独立契约 PR。
4. 本仓库文档即事实：改行为必须同 PR 改文档（AGENTS.md §4.5），这条纪律是两日冲刺不失控的根本。
