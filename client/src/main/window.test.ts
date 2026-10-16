// @vitest-environment node
import { EventEmitter } from 'node:events';
import type { IpcMainEvent, WebContents } from 'electron';
import { describe, expect, it, vi } from 'vitest';
import { windowChannels } from './ipc/channels';
import {
  guardClose,
  registerWindowIpc,
  watchMaximized,
  type ControlledWindow,
  type WindowIpcTarget,
} from './window';

class FakeIpc implements WindowIpcTarget {
  readonly listeners = new Map<string, ((e: IpcMainEvent) => void)[]>();
  on(channel: string, listener: (e: IpcMainEvent) => void): this {
    this.listeners.set(channel, [...(this.listeners.get(channel) ?? []), listener]);
    return this;
  }
  emit(channel: string, sender: unknown): void {
    for (const l of this.listeners.get(channel) ?? []) l({ sender } as IpcMainEvent);
  }
}

class FakeWindow extends EventEmitter implements ControlledWindow {
  readonly webContents = { send: vi.fn() };
  maximized = false;
  destroyed = false;
  minimize = vi.fn();
  maximize = vi.fn(() => {
    this.maximized = true;
  });
  unmaximize = vi.fn(() => {
    this.maximized = false;
  });
  close = vi.fn(() => {
    let prevented = false;
    this.emit('close', { preventDefault: () => (prevented = true) });
    if (!prevented) this.destroyed = true;
  });
  isMaximized = (): boolean => this.maximized;
  isDestroyed = (): boolean => this.destroyed;
}

function twoWindows() {
  const a = new FakeWindow();
  const b = new FakeWindow();
  const byContents = new Map<unknown, FakeWindow>([
    [a.webContents, a],
    [b.webContents, b],
  ]);
  const windowOf = (sender: WebContents) => byContents.get(sender) ?? null;
  return { a, b, windowOf };
}

describe('registerWindowIpc', () => {
  it('registers each window channel once, even if called again (macOS activate)', () => {
    const ipc = new FakeIpc();
    const { windowOf } = twoWindows();
    registerWindowIpc(ipc, windowOf);
    registerWindowIpc(ipc, windowOf);
    expect([...ipc.listeners.keys()].sort()).toEqual(
      [windowChannels.close, windowChannels.minimize, windowChannels.toggleMaximize].sort(),
    );
    for (const ls of ipc.listeners.values()) expect(ls).toHaveLength(1);
  });

  it('acts only on the window that sent the message', () => {
    const ipc = new FakeIpc();
    const { a, b, windowOf } = twoWindows();
    registerWindowIpc(ipc, windowOf);
    ipc.emit(windowChannels.minimize, b.webContents);
    expect(b.minimize).toHaveBeenCalledTimes(1);
    expect(a.minimize).not.toHaveBeenCalled();

    ipc.emit(windowChannels.toggleMaximize, a.webContents);
    expect(a.maximized).toBe(true);
    ipc.emit(windowChannels.toggleMaximize, a.webContents);
    expect(a.maximized).toBe(false);

    ipc.emit(windowChannels.close, a.webContents);
    expect(a.close).toHaveBeenCalledTimes(1);
    expect(b.close).not.toHaveBeenCalled();
  });

  it('ignores unknown senders and destroyed windows', () => {
    const ipc = new FakeIpc();
    const { a, windowOf } = twoWindows();
    registerWindowIpc(ipc, windowOf);
    ipc.emit(windowChannels.minimize, {});
    a.destroyed = true;
    ipc.emit(windowChannels.minimize, a.webContents);
    expect(a.minimize).not.toHaveBeenCalled();
  });
});

describe('watchMaximized', () => {
  it('pushes the maximized state to the window renderer', () => {
    const w = new FakeWindow();
    watchMaximized(w);
    w.maximized = true;
    w.emit('maximize');
    w.maximized = false;
    w.emit('unmaximize');
    expect(w.webContents.send.mock.calls).toEqual([
      [windowChannels.maximizedChanged, true],
      [windowChannels.maximizedChanged, false],
    ]);
  });
});

describe('guardClose', () => {
  function guarded(active: boolean, answer: boolean) {
    const w = new FakeWindow();
    let running = active;
    let resolveConfirm: (v: boolean) => void = () => {};
    const deps = {
      hasActiveJob: () => running,
      confirm: vi.fn(
        () =>
          new Promise<boolean>((r) => {
            resolveConfirm = r;
          }),
      ),
      cancelActiveJob: vi.fn(async () => {
        running = false;
      }),
    };
    guardClose(w, deps);
    return { w, deps, answer: () => resolveConfirm(answer) };
  }

  const flush = () => new Promise((r) => setTimeout(r, 0));

  it('closes immediately when no job is running', () => {
    const g = guarded(false, true);
    g.w.close();
    expect(g.w.destroyed).toBe(true);
    expect(g.deps.confirm).not.toHaveBeenCalled();
  });

  it('keeps the window open when the user chooses to continue the job', async () => {
    const g = guarded(true, false);
    g.w.close();
    expect(g.w.destroyed).toBe(false);
    g.answer();
    await flush();
    expect(g.deps.cancelActiveJob).not.toHaveBeenCalled();
    expect(g.w.destroyed).toBe(false);
  });

  it('cancels the job first, then closes after confirmation', async () => {
    const g = guarded(true, true);
    g.w.close();
    g.w.close();
    expect(g.deps.confirm).toHaveBeenCalledTimes(1);
    g.answer();
    await flush();
    expect(g.deps.cancelActiveJob).toHaveBeenCalledTimes(1);
    expect(g.w.destroyed).toBe(true);
  });
});
