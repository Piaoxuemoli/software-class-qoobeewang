# WSL2 + WSLg 安装运行说明（C4）

> 目标：在本机 WSL2 Ubuntu（22.04/24.04）中安装并运行 DataBackup 桌面应用，用于开发验证与课堂演示。
> 以下全部步骤已于 2026-10-17 在 Windows 11 + WSL2 Ubuntu 24.04 + WSLg 环境实测通过。

## 1. 前置条件

- Windows 11（或支持 WSLg 的 Windows 10 21H2+），WSL2 已启用。确认 WSLg 可用：WSL 内 `echo $DISPLAY` 应输出 `:0`（或类似）。
- Ubuntu 22.04 / 24.04 发行版。**开发/构建**需要完整工具链（root 或 sudo）：

  ```bash
  bash scripts/install-deps.sh   # Go/Node/staticcheck 锁定版 + Electron 运行库，幂等
  ```

- 仅**安装运行**打包产物不需要 Go/Node，但需要 Electron 运行库。最小集（24.04 包名）：

  ```bash
  sudo apt-get install -y libgtk-3-0t64 libnss3 libasound2t64 libgbm1 \
    libxss1 libxtst6 libxkbcommon0 libxcomposite1 libxdamage1 \
    libxrandr2 libxfixes3 libdrm2 libnotify4 xdg-utils
  ```

  22.04 对应去掉 `t64` 后缀（`libgtk-3-0`、`libasound2` 等）。跑一遍 `install-deps.sh` 是最省事的做法。

## 2. 构建（在 WSL 内，Linux 原生文件系统上）

```bash
git clone <repo> ~/databackup && cd ~/databackup
make package          # 后端 release 构建 + 客户端 electron-vite build + electron-builder
ls -la dist/
# DataBackup-0.1.0.AppImage          # AppImage（单文件，免安装）
# databackup-client_0.1.0_amd64.deb  # deb 包
# databackup                          # 后端 CLI 单独可用
```

网络受限时先设镜像（与 `scripts/docker.sh` 同源）：

```bash
export npm_config_registry=https://registry.npmmirror.com \
       ELECTRON_MIRROR=https://npmmirror.com/mirrors/electron/ \
       ELECTRON_BUILDER_BINARIES_MIRROR=https://npmmirror.com/mirrors/electron-builder-binaries/
```

注意：不要在 `/mnt/c`（Windows 挂载盘）上构建——9P 文件系统慢且偶发权限问题；克隆到 WSL 原生目录（如 `~/databackup`）。

## 3. 安装与运行

### 方式 A：deb（推荐演示用）

```bash
sudo dpkg -i dist/databackup-client_0.1.0_amd64.deb
sudo apt-get -f install -y   # 补 deb 声明的依赖（libsecret-1-0）
```

- GUI：直接运行 `/opt/DataBackup/databackup-client`（或终端 `databackup-client`，已注册 `/usr/bin` 软链与应用菜单项）。
- 内置后端 CLI：`/opt/DataBackup/resources/bin/databackup --help`（GUI 与 CLI 共用同一二进制，`resolve.ts` 在打包态从 `resources/bin/` 解析）。

### 方式 B：AppImage（免 root、免安装）

```bash
chmod +x dist/DataBackup-0.1.0.AppImage
./dist/DataBackup-0.1.0.AppImage --appimage-extract-and-run   # 无 libfuse2 时
# 或先安装 FUSE 后直跑：
sudo apt-get install -y libfuse2t64   # 22.04: libfuse2
./dist/DataBackup-0.1.0.AppImage
```

### 验证

```bash
B=/opt/DataBackup/resources/bin/databackup
mkdir -p ~/demo/src/sub && echo hello > ~/demo/src/a.txt && echo world > ~/demo/src/sub/b.txt
$B backup --source ~/demo/src --repo ~/demo/repo --name demo
$B restore --backup demo --repo ~/demo/repo --target ~/demo/dst
diff -r ~/demo/src ~/demo/dst && echo OK
```

GUI 在 WSLg 下直接出窗口（无需手动配置 X server）。

## 4. 已知坑（实测记录）

| 坑 | 说明与对策 |
|---|---|
| WSL `/tmp` 易失 | WSL 空闲自动停机，下次调用 systemd 清 `/tmp`——跨 `wsl.exe` 调用的中间产物放 `~` 下，别放 `/tmp` |
| AppImage 直跑报 `libfuse.so.2` | 24.04 默认无 libfuse2：`apt install libfuse2t64` 或加 `--appimage-extract-and-run` |
| deb 首次 `dpkg -i` 报缺 `libsecret-1-0` | 正常，`apt-get -f install` 补齐即可 |
| 打包配置 | electron-builder 26 无 `linux.arch` 键（宿主默认 x64）；deb 目标要求 author email 与 maintainer |
| 屏幕录制/演示 | WSLg 窗口即普通 Windows 窗口，可直接用 Win+G / OBS 录制 |
