import type { BrowserWindow, IpcMainEvent, WebContents } from 'electron';
import { windowChannels } from './ipc/channels';

/**
 * 无边框窗口的控制 IPC（interaction.md §1 window.ts 职责）。
 * 渲染进程经 `window.databackup.window.*` 触发；最大化状态变化
 * 通过 `window:maximized-changed` 推送（payload: boolean）。
 */

/** window.ts 用到的 BrowserWindow 子集，便于测试注入。 */
export type ControlledWindow = Pick<
  BrowserWindow,
  'minimize' | 'maximize' | 'unmaximize' | 'isMaximized' | 'close' | 'isDestroyed'
>;

export interface WindowIpcTarget {
  on(channel: string, listener: (event: IpcMainEvent) => void): unknown;
}

const registered = new WeakSet<WindowIpcTarget>();

/**
 * 全局只注册一次窗口控制 IPC：按发送方 webContents 找到所属窗口再操作，
 * 所以 macOS activate 重建窗口时不会累积处理器。重复调用是空操作。
 */
export function registerWindowIpc(
  ipc: WindowIpcTarget,
  windowOf: (sender: WebContents) => ControlledWindow | null,
): void {
  if (registered.has(ipc)) return;
  registered.add(ipc);
  const withWindow =
    (action: (win: ControlledWindow) => void) =>
    (e: IpcMainEvent): void => {
      const win = windowOf(e.sender);
      if (win && !win.isDestroyed()) action(win);
    };
  ipc.on(
    windowChannels.minimize,
    withWindow((win) => win.minimize()),
  );
  ipc.on(
    windowChannels.toggleMaximize,
    withWindow((win) => (win.isMaximized() ? win.unmaximize() : win.maximize())),
  );
  ipc.on(
    windowChannels.close,
    withWindow((win) => win.close()),
  );
}

/** 窗口级事件只用到的子集。 */
export interface WatchedWindow {
  readonly webContents: Pick<WebContents, 'send'>;
  isMaximized(): boolean;
  isDestroyed(): boolean;
  close(): void;
  on(event: 'maximize' | 'unmaximize', listener: () => void): unknown;
  on(event: 'close', listener: (e: { preventDefault(): void }) => void): unknown;
}

/** 把最大化状态变化推给该窗口的渲染进程。监听挂在窗口自身上，随窗口销毁。 */
export function watchMaximized(win: WatchedWindow): void {
  const push = (): void => {
    if (!win.isDestroyed()) {
      win.webContents.send(windowChannels.maximizedChanged, win.isMaximized());
    }
  };
  win.on('maximize', push);
  win.on('unmaximize', push);
}

export interface CloseGuardDeps {
  hasActiveJob(): boolean;
  /** 询问用户是否停止任务并关闭；true 表示确认。 */
  confirm(): Promise<boolean>;
  /** 取消当前任务并等待其结束。 */
  cancelActiveJob(): Promise<void>;
}

/**
 * 有备份/还原任务运行时拦截关闭（interaction.md §5 退出）：先询问，确认后取消任务、
 * 等后端退出，再真正关闭窗口；询问期间重复点击关闭不会叠加对话框。
 */
export function guardClose(win: WatchedWindow, deps: CloseGuardDeps): void {
  let allowed = false;
  let asking = false;
  win.on('close', (e) => {
    if (allowed || !deps.hasActiveJob()) return;
    e.preventDefault();
    if (asking) return;
    asking = true;
    void (async () => {
      try {
        if (!(await deps.confirm())) return;
        await deps.cancelActiveJob();
        allowed = true;
        if (!win.isDestroyed()) win.close();
      } catch (err) {
        console.warn('[window] 关闭确认失败，窗口保持打开', err);
      } finally {
        asking = false;
      }
    })();
  });
}
