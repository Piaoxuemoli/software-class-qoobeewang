import { contextBridge, ipcRenderer } from 'electron';

/**
 * 渲染进程可见的 window.databackup（interaction.md §2）。
 * C0 只实现 window.* 命名空间并保留既有 getBackendVersion；
 * app/dialog/backup/restore/jobs/repo/config 由 C1 补齐。
 */
const api = {
  getBackendVersion: (): Promise<string> => ipcRenderer.invoke('backend:getVersion'),
  window: {
    minimize: (): void => {
      ipcRenderer.send('window:minimize');
    },
    toggleMaximize: (): void => {
      ipcRenderer.send('window:toggle-maximize');
    },
    close: (): void => {
      ipcRenderer.send('window:close');
    },
    onMaximizedChange: (cb: (maximized: boolean) => void): (() => void) => {
      const listener = (_e: Electron.IpcRendererEvent, maximized: boolean): void => cb(maximized);
      ipcRenderer.on('window:maximized-changed', listener);
      return () => {
        ipcRenderer.removeListener('window:maximized-changed', listener);
      };
    },
  },
};

export type DataBackupApi = typeof api;

contextBridge.exposeInMainWorld('databackup', api);
