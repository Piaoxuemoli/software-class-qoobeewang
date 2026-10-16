import path from 'node:path';
import { app, BrowserWindow, ipcMain } from 'electron';
import { JobManager } from './backend/jobs';
import { createMockRunner } from './backend/mock';
import { resolveBackendPath } from './backend/resolve';
import { createSpawnRunner, type BackendRunner } from './backend/runner';
import { confirmCloseWithJob, pickDirectory } from './dialogs';
import { registerIpcHandlers } from './ipc/handlers';
import { guardClose, registerWindowIpc, watchMaximized } from './window';

/** DATABACKUP_MOCK=1 时用 mock 后端回放脚本，不需要真实二进制（interaction.md §2.1）。 */
function createRunner(): BackendRunner {
  if (process.env.DATABACKUP_MOCK === '1') {
    console.warn('[backend] DATABACKUP_MOCK=1：使用 mock 后端');
    return createMockRunner();
  }
  return createSpawnRunner({ resolveBin: resolveBackendPath });
}

const jobs = new JobManager(createRunner());

function createWindow(): void {
  const win = new BrowserWindow({
    width: 1120,
    height: 720,
    minWidth: 960,
    minHeight: 640,
    title: 'Data Backup',
    // 无边框自绘外框（ADR-007，ui-design.md §3）：ready-to-show 后再显示避免白屏闪烁。
    frame: false,
    show: false,
    backgroundColor: '#000000',
    webPreferences: {
      preload: path.join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });

  watchMaximized(win);
  guardClose(win, {
    hasActiveJob: () => jobs.hasActiveJob(),
    confirm: () => confirmCloseWithJob(win),
    cancelActiveJob: () => jobs.cancelActive(),
  });
  win.once('ready-to-show', () => win.show());

  if (process.env.ELECTRON_RENDERER_URL) {
    void win.loadURL(process.env.ELECTRON_RENDERER_URL);
  } else {
    void win.loadFile(path.join(__dirname, '../renderer/index.html'));
  }
}

app.whenReady().then(() => {
  registerWindowIpc(ipcMain, (sender) => BrowserWindow.fromWebContents(sender));
  registerIpcHandlers(ipcMain, {
    jobs,
    clientVersion: app.getVersion(),
    pickDirectory,
  });
  createWindow();
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});

// 退出前取消仍在运行的后端进程（SIGTERM→SIGKILL），让后端有机会删除 .partial。
let quitting = false;
app.on('will-quit', (e) => {
  if (quitting || !jobs.hasRunning()) return;
  quitting = true;
  e.preventDefault();
  void jobs.cancelAll().finally(() => app.quit());
});
