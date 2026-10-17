# 发布规范

## 1. 版本号

- 语义化版本 `MAJOR.MINOR.PATCH`；课程交付前为 `0.x`，最终提交版为 `1.0.0`。
- 版本号有两处且必须一致：`backend/internal/version/version.go` 的 `Version` 与 `client/package.json` 的 `version`。`scripts/release/check-version.sh` 校验，Release 工作流强制执行。
- 预发布用 `-rc.N` 后缀（如 `v0.4.0-rc.1`），Release 自动标为 prerelease。

| 版本 | 里程碑 | 内容 |
|---|---|---|
| v0.1.0 | M1 | 需求 + 设计 + 框架代码 + 构建脚本 |
| v0.2.0 | M2 | 遍历、过滤、MyPack、基础备份还原 |
| v0.3.0 | M3 + M4 | 文件类型、元数据、Huffman 压缩 |
| v0.4.0 | M5 + M6 | 加密、备份管理、全链路集成 |
| v0.5.0 | C | Electron 客户端完整可用并打包 |
| v1.0.0 | M9 | 最终提交版 |

## 2. CHANGELOG

- 采用 Keep a Changelog 格式，`CHANGELOG.md` 顶部始终保留 `## [Unreleased]`。
- 每个面向用户的 PR 在 `[Unreleased]` 下的 `新增 / 变更 / 修复 / 移除` 中追加一行，带任务 ID。
- 发布时由 `scripts/release/bump-version.sh` 把 `[Unreleased]` 改名为版本号与日期。

**版本节写给人看**：发布时把该版本的 `[Unreleased]` 累积条目**改写**成一段简短的版本说明（参考下方 §2.1 的规则），不是原样改名了事。开发期的逐条记录留在 git 历史与 PR 里，CHANGELOG 的版本节是给"要不要升级/这是个什么版本"的读者看的。

### 2.1 版本节与 Release Notes 的写法

读者是使用者，不是审查者。结构：一两句话说明这是什么版本，然后按用户可感知的能力分组列要点，最后一句获取方式。约束：

- 全文 ≤15 行；要点 ≤6 条，每条一行，说完一件事。
- 数字只保留最能支撑结论的两三个（如覆盖率下限、关键性能），其余删。
- 不出现任务编号、内部文件路径、过程性词汇（"成文""就绪""落地""配套"）。
- 细节链接到文档或 README，不在正文展开。

`v1.0.0` 的合格示例（也是当前 Release Notes 的实际内容）：

```
DataBackup v1.0.0 —— 首个正式版。

Linux 目录树备份软件：备份为单个 .dbk 文件并原样还原。支持七种文件
类型与完整元数据、六维过滤，MyPack/Huffman/AES-256 全部自研（生产
代码零第三方依赖），配 Electron 桌面客户端。

- 备份/还原全链路：过滤 × 压缩 × 加密 32 组合矩阵验证
- 核心包测试覆盖率 87%~100%，race 与 fuzz 检测干净
- 打包产物：AppImage、deb（内置后端）、独立 CLI
- 全套文档 PDF 与答辩 PPT 随资产发布

安装与使用见 README；WSL 演示环境见 docs/client/wsl-guide.md。
```

## 3. 发布流程（需作者明确同意后执行）

1. `master` 最新且 CI 全绿。
2. `bash scripts/release/bump-version.sh X.Y.Z`（同步两处版本号并整理 CHANGELOG）。
3. `bash scripts/docker.sh check-all` 通过。
4. 仅 v1.0.0：先完成 M9-2（一次性生成并提交全部 PDF）。中间版本不生成 PDF。
5. 通过 PR 合入 `chore(release): vX.Y.Z`。
6. 在合并后的 `master` 上：`git tag -a vX.Y.Z -m "vX.Y.Z"`，`git push origin vX.Y.Z`。
7. `release.yml` 自动：校验版本 → `make check` → 构建后端（`databackup-X.Y.Z-linux-x86_64.tar.gz`）→ 打包客户端（AppImage、deb）→ 构建文档 PDF → `git archive` 源码包 → 生成 `SHA256SUMS` → 用 CHANGELOG 对应段落作为说明创建 GitHub Release 并上传全部资产。
8. 检查 Release 页面资产齐全；若 Notes 不符合 §2.1，`gh release edit vX.Y.Z --notes-file <文件>` 直接改写（Release 说明可以随时修，tag 不动）。
9. 在 `docs/project/schedule.md` 记录。

## 4. 修复已发布版本

从对应 tag 修复后合回 `master`，发 `PATCH` 版本；不要移动或删除已发布的 tag。

## 5. 课程提交包（M9-6）

- 实验平台：《小组实验报告》PDF（官方模板封面，真实身份）+ 答辩 PPT 的 PDF，打包。
- 助教邮箱：源代码（`git archive`）+ 可执行程序（后端二进制 + 客户端安装包）+ 构建脚本（Makefile、Dockerfile、`scripts/`）+ 演示视频链接文本，打包。
- 两个压缩包只输出到本地 `course/07-提交物/`，不进仓库。
