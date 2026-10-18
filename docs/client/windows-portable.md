# Windows 便携包使用说明（C4b，可选加分项）

> 目标平台仍是 Linux（Ubuntu 22.04/24.04）。本页描述的是可选的 Windows 演示路径：
> 交叉编译的后端 + electron-builder `portable` 单文件，免安装、免管理员权限。
> 最近验证：**2026-10-18**，包含详情 Sheet / 对话框样式修复后的重新构建。

## 构建

**推荐在 Windows 宿主机上直接构建**（electron 本身就是 win32 版本，不必跨平台下载 158 MB 的
win32 运行包，也不受容器网络影响）：

```powershell
# 后端交叉编译：可在容器或宿主 Go 里做
bash scripts/docker.sh make backend-windows   # 或 cd backend && GOOS=windows go build -o bin/databackup.exe ./cmd/databackup

cd client
npm ci
npm run package:win        # electron-vite build + electron-builder --win portable
```

产物：`client/release/DataBackup 1.0.1.exe`（约 116 MB 单文件，已 gitignore）。

或者仍在 Docker 开发镜像里构建——需要先让容器拿到 win32 的 Electron 与 NSIS 工具：

```bash
bash scripts/docker.sh make backend-windows
cd client && npx electron-builder --win portable -c.directories.output=/tmp/eb-out
```

> **镜像**：`electron-builder` 的工具下载用 `ELECTRON_BUILDER_BINARIES_MIRROR`，
> 默认值已改为 `https://registry.npmmirror.com/-/binary/electron-builder-binaries/`；
> 旧的 `npmmirror.com/mirrors/...` 地址在容器内会 TLS 握手失败。
>
> **注意**：在 Docker bind mount（Windows 文件系统）上 electron-builder 的
> `rename` 会 EACCES——把 `-c.directories.output` 指到容器内目录（如 `/tmp/eb-out`）
> 再把产物 `cp` 回 `release/`。详见 `docs/project/pitfalls.md`。

## 运行

双击 `DataBackup 1.0.1.exe` 即可：NSIS 自解压到 `%TEMP%` 后启动，
不写入 Program Files、不需要管理员。首次启动有 SmartScreen 提示
（未签名），选"仍要运行"。便携包内嵌 `resources/bin/databackup.exe`，
后端路径由 `client/src/main/backend/resolve.ts` 按 `process.platform`
自动选择 `.exe` 后缀；仍支持 `DATABACKUP_BIN` 环境变量覆盖。

## 本次验证（2026-10-18）

- `release/win-unpacked/DataBackup.exe`：Playwright 直驱启动，状态栏显示"后端版本: 1.0.1"，
  三个页面可切换；截图确认输入框、开关、卡片圆角与 Sheet 样式均为修复后的版本。
- `DataBackup 1.0.1.exe`（portable）：双击后自解压并出现窗口（标题 `Data Backup`），
  进程随后可正常结束。
- 产物未提交（`client/release/` 已被 gitignore）；v1.0.1 已把构建产物以 `DataBackup-1.0.1-portable.exe` 上传到 Release，供课堂演示直接下载。

## 与 Linux 版的功能差异

Windows 上以下能力平台本身不支持，还原时**静默跳过**（不发 warn 事件、
不打扰界面，只记录到本地日志 `%LOCALAPPDATA%\databackup\windows-skip.log`，
可用 `DATABACKUP_SKIPLOG` 覆盖）：

- uid/gid、POSIX 权限位、ctime/btime（atime/mtime 正常恢复）
- FIFO、socket、字符/块设备节点
- 硬链接身份识别（每个硬链接还原为独立普通文件）
- 符号链接：仅在开启开发者模式或管理员权限时可创建，否则跳过
- junction/其他 reparse point：不递归遍历，避免 `Application Data` 式自指死循环
- OneDrive 类占位文件：Readlink 不可读时按普通文件备份内容

取消（点"取消"按钮或关闭窗口）在 Windows 上是直接结束进程，
没有 SIGTERM 优雅期；残留 `.partial` 属预期，`list` 会自动忽略。

## 验证过的场景

- 备份/还原/list/info/delete 全流程（含 Huffman 压缩、AES-256 加密）
- 错误口令 → `E_AUTH`（exit 3）
- Unicode 文件名与内容、只读文件、400 MiB 大文件流式进度
- junction 不递归、`.partial` 不出现在列表、取消后进程正确终止
- 打包产物真机链路：启动 → 版本号显示 → 加密备份 → 还原一致 →
  `E_AUTH` → `repo.list`，全部通过（Playwright `executablePath` 直驱）
