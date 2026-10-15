# client/AGENTS.md

Electron 客户端的补充规则（全局规则见根目录 `AGENTS.md`）。动手前读 `docs/client/ui-design.md` 和 `docs/client/interaction.md`。

## 结构

- `src/main/`：窗口（无边框 + 自绘控制）、后端 runner/jobs、zod 校验的 IPC 处理器、目录对话框。
- `src/preload/`：只通过 `contextBridge` 暴露 `window.databackup`，不暴露 Node 能力。
- `src/renderer/`：React 页面与组件；状态放 zustand store；样式用 Tailwind + 设计令牌。
- `src/shared/`：与 `docs/architecture/cli-contract.md` 一一对应的类型（冻结接口，修改需单独"契约"PR）。
- `src/components/ui/`：shadcn/ui 组件；`src/components/reactbits/`：React Bits 组件（保留来源与许可注释）。

## 必须遵守

- 客户端不实现任何业务逻辑（过滤、打包、压缩、加密、校验），只把表单字段转换成 CLI 参数并展示事件。
- 窗口 `frame: false`，标题栏与窗口按钮自绘；不要改回原生标题栏或使用 `titleBarOverlay`（ADR-007）。
- 视觉只用设计令牌：纯黑画布、白色为唯一强调、细线边框、胶囊按钮；彩色只用于状态。
- 基础组件优先用 shadcn/ui，动效优先用 React Bits；新增 npm 依赖需在 PR 说明理由，版本精确锁定、发布满 7 天。
- 动效必须响应"减少动效"设置与 `prefers-reduced-motion`；WebGL 背景默认关闭。
- 口令只经 stdin 传给后端，提交后立即从渲染进程状态中清除，不写日志。
- 改界面的 PR 必须更新或确认 Playwright 截图基线（只在 Docker 开发镜像中生成）。

## 自检

```bash
make client-lint client-test e2e                      # 容器内
bash scripts/docker.sh make client-install client-lint client-test e2e   # Windows 主机
```
