// @vitest-environment node
import { beforeAll, describe, expect, it, vi } from 'vitest';
import type { DataBackupApi } from '../shared/api';
import type { BackendEvent } from '../shared/protocol';
import { channels, jobEventChannel, windowChannels } from '../main/ipc/channels';

const electron = vi.hoisted(() => ({
  contextBridge: { exposeInMainWorld: vi.fn() },
  ipcRenderer: {
    invoke: vi.fn().mockResolvedValue('ok'),
    send: vi.fn(),
    on: vi.fn(),
    removeListener: vi.fn(),
  },
}));
vi.mock('electron', () => electron);

let api: DataBackupApi;
let pushJobEvent: (jobId: string, event: BackendEvent) => void;

beforeAll(async () => {
  await import('./index');
  const [name, exposed] = electron.contextBridge.exposeInMainWorld.mock.calls[0];
  expect(name).toBe('databackup');
  api = exposed as DataBackupApi;
  const call = electron.ipcRenderer.on.mock.calls.find(([ch]) => ch === jobEventChannel);
  if (!call) throw new Error('preload did not subscribe to jobs:event');
  const listener = call[1] as (e: unknown, msg: unknown) => void;
  pushJobEvent = (jobId, event) => listener({}, { jobId, event });
});

const progress = (files: number): BackendEvent => ({
  type: 'progress',
  phase: 'scan',
  files,
  bytes: 0,
  current: 'a',
});

describe('preload window.databackup', () => {
  it('exposes every DataBackupApi namespace', () => {
    expect(Object.keys(api).sort()).toEqual(
      ['app', 'backup', 'config', 'dialog', 'jobs', 'repo', 'restore', 'window'].sort(),
    );
  });

  it('maps each method to its invoke channel and arguments', async () => {
    const req = { source: '/s', filters: {}, compress: false, encrypt: true };
    const cases: [() => Promise<unknown>, string, unknown[]][] = [
      [() => api.app.versions(), channels.appVersions, []],
      [() => api.dialog.pickDirectory({ title: 't' }), channels.pickDirectory, [{ title: 't' }]],
      [() => api.backup.dryRun(req), channels.backupDryRun, [req]],
      [() => api.backup.start(req, 'pw'), channels.backupStart, [req, 'pw']],
      [
        () => api.restore.start({ backup: 'b', target: '/t' }),
        channels.restoreStart,
        [{ backup: 'b', target: '/t' }, undefined],
      ],
      [() => api.jobs.cancel('j1'), channels.jobsCancel, ['j1']],
      [() => api.repo.list(), channels.repoList, []],
      [() => api.repo.info('n'), channels.repoInfo, ['n']],
      [() => api.repo.remove('n'), channels.repoRemove, ['n']],
      [() => api.config.get(), channels.configGet, []],
      [() => api.config.set({ repo: '/r' }), channels.configSet, [{ repo: '/r' }]],
    ];
    for (const [call, channel, args] of cases) {
      electron.ipcRenderer.invoke.mockClear();
      await call();
      expect(electron.ipcRenderer.invoke).toHaveBeenCalledWith(channel, ...args);
    }
  });

  it('sends window controls on their channels', () => {
    api.window.minimize();
    api.window.toggleMaximize();
    api.window.close();
    expect(electron.ipcRenderer.send.mock.calls).toEqual([
      [windowChannels.minimize],
      [windowChannels.toggleMaximize],
      [windowChannels.close],
    ]);
  });

  it('delivers job events only to subscribers of that job and stops after unsubscribe', () => {
    const got: BackendEvent[] = [];
    const off = api.jobs.onEvent('job-a', (e) => got.push(e));
    pushJobEvent('job-a', progress(1));
    pushJobEvent('job-other', progress(99));
    off();
    pushJobEvent('job-a', progress(2));
    expect(got).toEqual([progress(1)]);
  });

  it('replays events that arrived before the renderer subscribed', () => {
    pushJobEvent('job-early', progress(1));
    pushJobEvent('job-early', progress(2));
    const got: BackendEvent[] = [];
    api.jobs.onEvent('job-early', (e) => got.push(e));
    pushJobEvent('job-early', progress(3));
    expect(got).toEqual([progress(1), progress(2), progress(3)]);
  });
});
