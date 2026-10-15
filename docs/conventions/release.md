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

## 3. 发布流程（需作者明确同意后执行）

1. `master` 最新且 CI 全绿。
2. `bash scripts/release/bump-version.sh X.Y.Z`（同步两处版本号并整理 CHANGELOG）。
3. `bash scripts/docker.sh check-all` 通过。
4. 仅 v1.0.0：先完成 M9-2（一次性生成并提交全部 PDF）。中间版本不生成 PDF。
5. 通过 PR 合入 `chore(release): vX.Y.Z`。
6. 在合并后的 `master` 上：`git tag -a vX.Y.Z -m "vX.Y.Z"`，`git push origin vX.Y.Z`。
7. `release.yml` 自动：校验版本 → `make check` → 构建后端（`databackup-X.Y.Z-linux-x86_64.tar.gz`）→ 打包客户端（AppImage、deb）→ 构建文档 PDF → `git archive` 源码包 → 生成 `SHA256SUMS` → 用 CHANGELOG 对应段落作为说明创建 GitHub Release 并上传全部资产。
8. 检查 Release 页面资产齐全，在 `docs/project/schedule.md` 记录。

## 4. 修复已发布版本

从对应 tag 修复后合回 `master`，发 `PATCH` 版本；不要移动或删除已发布的 tag。

## 5. 课程提交包（M9-6）

- 实验平台：《小组实验报告》PDF（官方模板封面，真实身份）+ 答辩 PPT 的 PDF，打包。
- 助教邮箱：源代码（`git archive`）+ 可执行程序（后端二进制 + 客户端安装包）+ 构建脚本（Makefile、Dockerfile、`scripts/`）+ 演示视频链接文本，打包。
- 两个压缩包只输出到本地 `course/07-提交物/`，不进仓库。
