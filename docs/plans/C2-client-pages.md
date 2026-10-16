# C2 实施记录：三个页面接 mock 后端

## 结构

- `stores/job.ts`：interaction.md §3 状态机 + 备份编排表单 + 还原上下文。`idle/editing/ready`
  三个 pre-run 态按"表单是否可提交"连续判定（源目录为空 → idle；不可提交 → editing；可提交 → ready）。
  还原在 `ready → running` 之间插入 `awaitingPassword`（加密备份）；`E_AUTH` 回到该态并累计失败次数，
  连续 3 次回 `ready`；`E_PASSWORD_REQUIRED` 同样回 `awaitingPassword` 但不计失败。
- `stores/backups.ts`：仓库路径（config.get）+ 列表缓存，启动预取、备份完成 / 删除 / 改仓库后刷新。
- `components/`：`StatusMark`（状态图标）、`Counter`（300ms 数字滚动）、`HoldButton`（长按 1200ms 删除）、
  `JobCard`（进度/完成/失败/取消四张卡，备份页与还原 Sheet 共用）、`FilterCapsules`（六个筛选维度）、
  `PasswordDialog`（口令对话框，开合完全由 jobStore.status 驱动）。
- `components/ui/checkbox.tsx`、`textarea.tsx`：按 shadcn 源码新增（radix checkbox 依赖已在 package.json）。

## 与设计文档的偏差（已核对约束，不影响契约）

- **时间区间用文本输入**而非 react-day-picker 的 Calendar：值原样透传给后端校验（E_USAGE）。
  组件精简取舍，后续可平滑替换。
- **无拖入文件夹**：拖入文件取真实路径需要 preload 暴露 `webUtils.getPathForFile`，
  属于 main/preload 契约改动（本任务只允许删 getBackendVersion），留待后续契约 PR。
- 侧栏仓库路径与迷你进度接真实数据（config + jobStore）。

## 口令安全

- `backup.start(req, pw)` / `restore.start(req, pw)` 提交后立即从 store 清空（password/confirm 字段），
  组件不缓存、不写日志；IPC 层经 stdin 传给后端（C1 已实现）。

## 截图基线

- 新增：backup-progress.png（进度卡）、backup-done.png（完成态）、restore-sheet.png（详情 Sheet）、
  password-dialog.png（口令对话框）。
- 更新：page-backup / page-manage / page-settings 三张空闲态（骨架页换为真实页面）。
- 进度卡截图对动态元素（已用时间、当前路径、进度条、百分比、侧栏迷你进度）加 mask。
