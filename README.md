# DataBackup · 数据备份软件

电子科技大学《软件开发综合实验》课程项目。在 Linux 上把目录树备份成一个文件，并能原样还原——文件类型、权限属主时间、软硬链接都不丢；支持按路径/类型/名字/时间/尺寸/用户筛选，自研 MyPack 打包格式、Huffman 压缩与 AES-256 加密，配有 Electron 桌面客户端。

> 状态：M0 仓库基建完成，进入 M1 需求与设计。进度见 [docs/project/schedule.md](docs/project/schedule.md)。

## 功能

| 功能 | 状态 |
|---|---|
| 备份 / 还原目录树（FR-01、02） | 计划中（M2） |
| 特殊文件：软/硬链接、管道、设备、套接字（FR-03） | 计划中（M3） |
| 元数据：权限、属主、时间（FR-04） | 计划中（M3） |
| 自定义备份：路径、类型、名字、时间、尺寸、用户（FR-05~10） | 计划中（M2） |
| 打包解包：自研 MyPack（FR-11） | 计划中（M2） |
| 压缩解压：自研 Huffman（FR-12） | 计划中（M4） |
| 加密解密：自研 AES-256 + PBKDF2 + HMAC（FR-13） | 计划中（M5） |
| 备份管理与设置（FR-14、15） | 计划中（M6） |
| Electron 图形客户端（FR-16） | 骨架已就绪（C） |

完整需求见 [docs/project/overview.md](docs/project/overview.md)。

## 技术栈

- **后端**：Go，仅标准库（扩展功能全部自研，由 `scripts/check-imports.sh` 强制）
- **客户端**：Electron + React + TypeScript，通过 CLI + JSON Lines 调用后端
- **工程**：Makefile、Docker 开发镜像、GitHub Actions CI / Release
- **文档**：Markdown + PlantUML，自动生成 PDF（`docs/pdf/`）

## 快速开始

Windows 主机（Git Bash + Docker Desktop，无需安装 make/Go/Node）：

```bash
bash scripts/docker.sh check     # 在开发镜像里跑格式、lint、构建、测试
bash scripts/docker.sh shell     # 进入开发容器
bash scripts/docker.sh docs      # 重新生成 docs/pdf/ 下的 PDF
```

Linux / 开发容器内：

```bash
bash scripts/install-deps.sh   # 首次：安装 Go、Node 等工具链（容器内已装好）
make check                     # 快速门禁
make backend && ./backend/bin/databackup --help
make client-install && make client
make help                      # 全部目标
```

演示环境推荐 WSL2 Ubuntu（WSLg 可直接显示 Electron 窗口），说明见 [docs/project/overview.md](docs/project/overview.md#5-运行与开发环境)。

## 目录

```
backend/   Go 后端          client/   Electron 客户端
docs/      文档与 PDF        scripts/  构建/lint/文档/发布脚本
docker/    开发与文档镜像    .github/  CI 与模板
```

## 参与开发

人和 agent 都从 [AGENTS.md](AGENTS.md) 开始；规范在 [docs/conventions/](docs/conventions/)，任务在 [docs/roadmap.md](docs/roadmap.md)。

## 许可

[MIT](LICENSE)
