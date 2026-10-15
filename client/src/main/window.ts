import { BrowserWindow, ipcMain } from 'electron';

/**
 * 无边框窗口的控制 IPC（interaction.md §1 window.ts 职责）。
 * 渲染进程经 `window.databackup.window.*` 触发；最大化状态变化
 * 通过 `window:maximized-changed` 推送（payload: boolean）。
 */
export const windowChannels = {
  minimize: 'window:minimize',
  toggleMaximize: 'window:toggle-maximize',
  close: 'window:close',
  maximizedChanged: 'window:maximized-changed',
} as const;

function isFrom(win: BrowserWindow, sender: Electron.WebContents): boolean {
  return sender === win.webContents;
}

/** 注册窗口控制 IPC 与最大化状态推送。 */
export function registerWindowIpc(win: BrowserWindow): void {
  ipcMain.on(windowChannels.minimize, (e) => {
    if (isFrom(win, e.sender)) win.minimize();
  });
  ipcMain.on(windowChannels.toggleMaximize, (e) => {
    if (!isFrom(win, e.sender)) return;
    if (win.isMaximized()) {
      win.unmaximize();
    } else {
      win.maximize();
    }
  });
  ipcMain.on(windowChannels.close, (e) => {
    if (isFrom(win, e.sender)) win.close();
  });

  const pushMaximized = (): void => {
    if (!win.isDestroyed()) {
      win.webContents.send(windowChannels.maximizedChanged, win.isMaximized());
    }
  };
  win.on('maximize', pushMaximized);
  win.on('unmaximize', pushMaximized);
}
