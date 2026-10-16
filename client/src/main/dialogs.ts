import { BrowserWindow, dialog, type WebContents } from 'electron';

/** 选择目录（interaction.md §1 dialogs.ts）；用户取消返回 null。 */
export async function pickDirectory(
  sender: WebContents,
  title: string | undefined,
): Promise<string | null> {
  const win = BrowserWindow.fromWebContents(sender);
  const options: Electron.OpenDialogOptions = {
    title: title ?? '选择目录',
    properties: ['openDirectory', 'createDirectory'],
  };
  const r = win ? await dialog.showOpenDialog(win, options) : await dialog.showOpenDialog(options);
  return r.canceled ? null : (r.filePaths[0] ?? null);
}

/** 有任务运行时关闭窗口前的确认框（interaction.md §5 退出）；返回 true 表示停止任务并关闭。 */
export async function confirmCloseWithJob(win: BrowserWindow): Promise<boolean> {
  const r = await dialog.showMessageBox(win, {
    type: 'warning',
    title: 'Data Backup',
    message: '备份或还原任务正在运行',
    detail: '关闭窗口会先取消当前任务。还原被取消时已写出的文件不会回滚。',
    buttons: ['继续任务', '停止任务并关闭'],
    defaultId: 0,
    cancelId: 0,
    noLink: true,
  });
  return r.response === 1;
}
