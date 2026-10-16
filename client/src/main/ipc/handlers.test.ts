// @vitest-environment node
import type { IpcMainInvokeEvent, WebContents } from 'electron';
import { describe, expect, it, vi } from 'vitest';
import { parseApiError, type BackupRequest } from '../../shared/api';
import { JobManager } from '../backend/jobs';
import {
  createDefaultMockScript,
  FakeChild,
  mockSpawn,
  MOCK_PASSWORD,
  type MockScriptFn,
} from '../backend/mock';
import { createSpawnRunner } from '../backend/runner';
import { channels, jobEventChannel, type JobEventMessage } from './channels';
import { registerIpcHandlers, type IpcHandleTarget } from './handlers';

type Listener = (event: IpcMainInvokeEvent, ...args: unknown[]) => unknown;

function setup(script: MockScriptFn = createDefaultMockScript({ stepMs: 1 })) {
  const handlers = new Map<string, Listener>();
  const ipc: IpcHandleTarget = {
    handle: (channel, listener) => {
      expect(handlers.has(channel), `duplicate handler ${channel}`).toBe(false);
      handlers.set(channel, listener);
    },
  };
  const children: FakeChild[] = [];
  const runner = createSpawnRunner({
    resolveBin: () => 'databackup',
    spawn: mockSpawn(script, (c) => children.push(c)),
    log: () => {},
  });
  const jobs = new JobManager(runner);
  const pickDirectory = vi.fn().mockResolvedValue('/picked');
  registerIpcHandlers(ipc, { jobs, clientVersion: '9.9.9', pickDirectory });

  const sent: JobEventMessage[] = [];
  const sender = {
    isDestroyed: () => false,
    send: (channel: string, msg: JobEventMessage) => {
      expect(channel).toBe(jobEventChannel);
      sent.push(msg);
    },
  } as unknown as WebContents;
  const event = { sender } as IpcMainInvokeEvent;
  const invoke = (channel: string, ...args: unknown[]): Promise<unknown> => {
    const h = handlers.get(channel);
    if (!h) throw new Error(`no handler for ${channel}`);
    return Promise.resolve(h(event, ...args));
  };
  return { handlers, invoke, sent, children, jobs, pickDirectory, sender };
}

async function apiErrorOf(p: Promise<unknown>) {
  try {
    await p;
  } catch (err) {
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toMatch(/^\[E_[A-Z_]+\] /);
    return parseApiError(err);
  }
  throw new Error('expected rejection');
}

async function waitFor(cond: () => boolean): Promise<void> {
  for (let i = 0; i < 500 && !cond(); i++) await new Promise((r) => setTimeout(r, 1));
  expect(cond()).toBe(true);
}

const req: BackupRequest = { source: '/home/u/docs', filters: {}, compress: false, encrypt: false };

describe('registerIpcHandlers', () => {
  it('registers every invoke channel exactly once', () => {
    const s = setup();
    expect([...s.handlers.keys()].sort()).toEqual(Object.values(channels).sort());
  });

  it('app.versions returns client and backend versions', async () => {
    const s = setup();
    await expect(s.invoke(channels.appVersions)).resolves.toEqual({
      client: '9.9.9',
      backend: '0.0.0-mock',
    });
  });

  it('maps a missing backend to [E_BACKEND_UNAVAILABLE]', async () => {
    const s = setup(() => ({ steps: [], spawnError: 'ENOENT' }));
    expect((await apiErrorOf(s.invoke(channels.appVersions))).code).toBe('E_BACKEND_UNAVAILABLE');
    expect((await apiErrorOf(s.invoke(channels.backupStart, req))).code).toBe(
      'E_BACKEND_UNAVAILABLE',
    );
  });

  it('dialog.pickDirectory forwards the sender and title', async () => {
    const s = setup();
    await expect(s.invoke(channels.pickDirectory, { title: '选择源目录' })).resolves.toBe(
      '/picked',
    );
    await expect(s.invoke(channels.pickDirectory, undefined)).resolves.toBe('/picked');
    expect(s.pickDirectory).toHaveBeenNthCalledWith(1, s.sender, '选择源目录');
    expect(s.pickDirectory).toHaveBeenNthCalledWith(2, s.sender, undefined);
  });

  it('backup.dryRun returns the summary without type/command', async () => {
    const s = setup();
    await expect(s.invoke(channels.backupDryRun, req)).resolves.toEqual({
      files: 120,
      dirs: 14,
      bytes: 5242880,
      skipped: 7,
    });
    expect(s.children[0].args.slice(0, 2)).toEqual(['backup', '--dry-run']);
    expect(s.children[0].stdinText).toBe('');
  });

  it('backup.start pushes progress and the final result on jobs:event', async () => {
    const s = setup();
    const id = await s.invoke(channels.backupStart, { ...req, name: 'n1', compress: true });
    expect(typeof id).toBe('string');
    await waitFor(() => s.sent.at(-1)?.event.type === 'result');
    expect(s.sent.every((m) => m.jobId === id)).toBe(true);
    expect(s.sent[0].event).toMatchObject({ type: 'progress', phase: 'scan' });
    expect(s.sent.at(-1)?.event).toMatchObject({ command: 'backup', name: 'n1', compressed: true });
    expect(s.children[0].args).toEqual([
      'backup',
      '--source',
      '/home/u/docs',
      '--name',
      'n1',
      '--compress',
      'huffman',
    ]);
  });

  it('encrypting backups pass --encrypt aes256 --password-stdin and write the password to stdin', async () => {
    const s = setup();
    const id = (await s.invoke(channels.backupStart, { ...req, encrypt: true }, 'pw-1')) as string;
    expect(s.children[0].args.slice(-3)).toEqual(['--encrypt', 'aes256', '--password-stdin']);
    await waitFor(() => s.children[0].stdinText !== '');
    expect(s.children[0].stdinText).toBe('pw-1\n');
    await s.invoke(channels.jobsCancel, id);
  });

  it('drops the password when the backup is not encrypted', async () => {
    const s = setup();
    const id = (await s.invoke(channels.backupStart, req, 'unused')) as string;
    expect(s.children[0].args).not.toContain('--password-stdin');
    expect(s.children[0].stdinText).toBe('');
    await s.invoke(channels.jobsCancel, id);
  });

  it('a second backup/restore is rejected with [E_BUSY]; cancel frees the slot', async () => {
    const s = setup(createDefaultMockScript({ stepMs: 50 }));
    const id = (await s.invoke(channels.backupStart, req)) as string;
    expect((await apiErrorOf(s.invoke(channels.backupStart, req))).code).toBe('E_BUSY');
    expect(
      (await apiErrorOf(s.invoke(channels.restoreStart, { backup: 'old', target: '/t' }))).code,
    ).toBe('E_BUSY');
    await expect(s.invoke(channels.repoList)).resolves.toBeInstanceOf(Array);
    await expect(s.invoke(channels.jobsCancel, id)).resolves.toBeUndefined();
    await waitFor(() => s.sent.at(-1)?.event.type === 'error');
    expect(s.sent.at(-1)?.event).toMatchObject({ type: 'error', code: 'E_CANCELLED' });
    const again = await s.invoke(channels.backupStart, { ...req, name: 'again' });
    expect(again).toBeTypeOf('string');
    await s.invoke(channels.jobsCancel, again);
  });

  it('restore.start adds --password-stdin and reports E_AUTH as the final event', async () => {
    const s = setup();
    await s.invoke(channels.restoreStart, { backup: 'docs-20261015-235500', target: '/t' }, 'bad');
    expect(s.children[0].args).toEqual([
      'restore',
      '--backup',
      'docs-20261015-235500',
      '--target',
      '/t',
      '--password-stdin',
    ]);
    await waitFor(() => s.sent.length > 0);
    expect(s.sent.at(-1)?.event).toMatchObject({ type: 'error', code: 'E_AUTH' });
  });

  it('restore.start succeeds with the right password', async () => {
    const s = setup();
    await s.invoke(
      channels.restoreStart,
      { backup: 'docs-20261015-235500', target: '/t' },
      MOCK_PASSWORD,
    );
    await waitFor(() => s.sent.at(-1)?.event.type === 'result');
    expect(s.sent.at(-1)?.event).toMatchObject({ command: 'restore', target: '/t' });
  });

  it('repo.list/info/remove and config.get/set map backend results', async () => {
    const s = setup();
    const list = (await s.invoke(channels.repoList)) as { name: string }[];
    expect(list.map((b) => b.name)).toContain('old');
    const info = await s.invoke(channels.repoInfo, 'old');
    expect(info).toMatchObject({ name: 'old', corrupt: true });
    expect(info).not.toHaveProperty('type');
    expect(info).not.toHaveProperty('command');
    await expect(s.invoke(channels.repoRemove, 'old')).resolves.toBeUndefined();
    expect((await apiErrorOf(s.invoke(channels.repoInfo, 'old'))).code).toBe('E_NOT_FOUND');
    await expect(s.invoke(channels.configGet)).resolves.toEqual({ repo: '/home/mock/DataBackup' });
    await expect(s.invoke(channels.configSet, { repo: '/data/r' })).resolves.toEqual({
      repo: '/data/r',
    });
    await expect(s.invoke(channels.configSet, {})).resolves.toEqual({ repo: '/data/r' });
    expect(s.children.at(-2)?.args).toEqual(['config', 'set', 'repo', '/data/r']);
    expect(s.children.at(-1)?.args).toEqual(['config', 'get']);
  });
});

describe('IPC payload validation', () => {
  const bad: [string, string, unknown[]][] = [
    ['missing source', channels.backupStart, [{ filters: {}, compress: false, encrypt: false }]],
    ['empty source', channels.backupDryRun, [{ ...req, source: '' }]],
    ['NUL in path', channels.backupDryRun, [{ ...req, source: '/a\0b' }]],
    ['unknown request key', channels.backupStart, [{ ...req, shell: 'rm -rf /' }]],
    ['unknown filter key', channels.backupDryRun, [{ ...req, filters: { foo: 1 } }]],
    ['bad type letter', channels.backupDryRun, [{ ...req, filters: { types: ['x'] } }]],
    ['negative size', channels.backupDryRun, [{ ...req, filters: { sizeMin: -1 } }]],
    ['fractional size', channels.backupDryRun, [{ ...req, filters: { sizeMax: 1.5 } }]],
    ['string boolean', channels.backupStart, [{ ...req, compress: 'yes' }]],
    ['password not string', channels.backupStart, [{ ...req, encrypt: true }, 42]],
    ['extra argument', channels.backupDryRun, [req, 'extra']],
    ['missing request', channels.backupStart, []],
    ['restore missing target', channels.restoreStart, [{ backup: 'x' }]],
    ['name starting with dash', channels.repoInfo, ['--repo=/etc']],
    ['name not string', channels.repoRemove, [1]],
    ['config unknown key', channels.configSet, [{ theme: 'dark' }]],
    ['config relative-looking flag', channels.configSet, [{ repo: '-x' }]],
    ['job id not string', channels.jobsCancel, [7]],
    ['dialog opts unknown key', channels.pickDirectory, [{ title: 'x', defaultPath: '/' }]],
    ['args to no-arg channel', channels.repoList, ['x']],
  ];

  it.each(bad)('rejects %s with [E_USAGE]', async (_name, channel, args) => {
    const s = setup();
    const err = await apiErrorOf(s.invoke(channel, ...args));
    expect(err.code).toBe('E_USAGE');
    expect(err.message).toContain(channel);
    expect(s.children).toHaveLength(0);
  });

  it('rejects passwords containing a newline without echoing them', async () => {
    const s = setup();
    const pw = 'top\nsecret-value';
    const err = await apiErrorOf(s.invoke(channels.backupStart, { ...req, encrypt: true }, pw));
    expect(err.code).toBe('E_USAGE');
    expect(err.message).not.toContain('secret-value');
    expect(s.children).toHaveLength(0);
  });
});
