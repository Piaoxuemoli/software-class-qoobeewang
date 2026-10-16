import type { JobId } from '../../shared/api';
import type { BackendEvent } from '../../shared/protocol';

/**
 * main ↔ preload 的 IPC 通道名（interaction.md §2.1）。preload 也引用本文件，
 * 因此这里只能有常量与类型，不能引入 electron 或 Node 模块。
 */
export const channels = {
  appVersions: 'app:versions',
  pickDirectory: 'dialog:pick-directory',
  backupDryRun: 'backup:dry-run',
  backupStart: 'backup:start',
  restoreStart: 'restore:start',
  jobsCancel: 'jobs:cancel',
  repoList: 'repo:list',
  repoInfo: 'repo:info',
  repoRemove: 'repo:remove',
  configGet: 'config:get',
  configSet: 'config:set',
} as const;

/** main → renderer 推送：任务事件。 */
export const jobEventChannel = 'jobs:event';

export interface JobEventMessage {
  jobId: JobId;
  event: BackendEvent;
}

/** 窗口控制（ipcRenderer.send，单向）与最大化状态推送。 */
export const windowChannels = {
  minimize: 'window:minimize',
  toggleMaximize: 'window:toggle-maximize',
  close: 'window:close',
  maximizedChanged: 'window:maximized-changed',
} as const;
