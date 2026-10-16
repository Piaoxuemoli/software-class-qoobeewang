import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { BackendEvent, BackupResult, ErrorCode } from '@/shared/protocol';
import { useJobStore, initialJobState } from './job';
import { installFakeApi, fakeBackups, type FakeApi } from '../test/fakeApi';

let api: FakeApi;
const store = (): ReturnType<typeof useJobStore.getState> => useJobStore.getState();

const progress = (phase: 'scan' | 'pack', files = 1): BackendEvent => ({
  type: 'progress',
  phase,
  files,
  bytes: files * 10,
  totalFiles: 10,
  totalBytes: 100,
  current: 'a.txt',
});

const backupResult: BackupResult = {
  type: 'result',
  command: 'backup',
  name: 'x',
  path: '/repo/x.dbk',
  files: 2,
  dirs: 1,
  bytes: 100,
  outputBytes: 90,
  compressed: false,
  encrypted: false,
  skipped: 0,
  warnings: 0,
  ms: 50,
};

const err = (code: ErrorCode, message = 'm'): BackendEvent => ({
  type: 'error',
  code,
  message,
});

/** 走完 start 流程进入 running（表单已就绪）。 */
async function toRunning(): Promise<void> {
  store().setSource('/data');
  await store().start();
  expect(store().status).toBe('running');
}

beforeEach(() => {
  api = installFakeApi();
  useJobStore.setState({ ...initialJobState, form: { ...initialJobState.form, filters: {} } });
});

describe('jobStore 状态机（interaction.md §3）', () => {
  it('TC-UI-001 初始为 idle，源目录为空', () => {
    expect(store().status).toBe('idle');
    expect(store().form.source).toBe('');
  });

  it('TC-UI-002 idle → editing：开加密后选择源目录（口令未填，表单不完整）', () => {
    store().setEncrypt(true);
    expect(store().status).toBe('idle');
    store().setSource('/data');
    expect(store().status).toBe('editing');
  });

  it('TC-UI-003 editing → editing：修改筛选条件仍在 editing', () => {
    store().setEncrypt(true);
    store().setSource('/data');
    store().setFilters({ types: ['f', 'd'] });
    expect(store().status).toBe('editing');
    expect(store().form.filters.types).toEqual(['f', 'd']);
  });

  it('TC-UI-004 editing → ready：口令补齐后表单校验通过', () => {
    store().setEncrypt(true);
    store().setSource('/data');
    store().setPassword('pw');
    expect(store().status).toBe('editing');
    store().setConfirm('pw');
    expect(store().status).toBe('ready');
  });

  it('TC-UI-005 ready → editing：修改使校验失败', () => {
    store().setEncrypt(true);
    store().setSource('/data');
    store().setPassword('pw');
    store().setConfirm('pw');
    expect(store().status).toBe('ready');
    store().setConfirm('different');
    expect(store().status).toBe('editing');
  });

  it('TC-UI-006 ready → running：start 调 backup.start 并清空口令', async () => {
    store().setEncrypt(true);
    store().setSource('/data');
    store().setName('my-backup');
    store().setFilters({ types: ['f'] });
    store().setCompress(true);
    store().setPassword('secret');
    store().setConfirm('secret');
    await store().start();

    expect(api.backup.start).toHaveBeenCalledWith(
      {
        source: '/data',
        name: 'my-backup',
        filters: { types: ['f'] },
        compress: true,
        encrypt: true,
      },
      'secret',
    );
    expect(store().status).toBe('running');
    // 口令提交后立即从状态中清除
    expect(store().form.password).toBe('');
    expect(store().form.confirm).toBe('');
  });

  it('TC-UI-007 running 内部：scan → pack 阶段更新', async () => {
    await toRunning();
    store().handleEvent(progress('scan'));
    expect(store().phase).toBe('scan');
    store().handleEvent(progress('pack', 5));
    expect(store().phase).toBe('pack');
    expect(store().progress?.files).toBe(5);
  });

  it('TC-UI-008 running → cancelling：cancel 调 jobs.cancel', async () => {
    await toRunning();
    await store().cancel();
    expect(api.jobs.cancel).toHaveBeenCalledWith('job-1');
    expect(store().status).toBe('cancelling');
  });

  it('TC-UI-009 cancelling → cancelled：收到 E_CANCELLED', async () => {
    await toRunning();
    await store().cancel();
    store().handleEvent(err('E_CANCELLED', '已取消'));
    expect(store().status).toBe('cancelled');
  });

  it('TC-UI-010 running → succeeded：收到 result', async () => {
    await toRunning();
    store().handleEvent(backupResult);
    expect(store().status).toBe('succeeded');
    expect(store().result).toEqual(backupResult);
  });

  it('TC-UI-011 running → failed：收到非取消错误', async () => {
    await toRunning();
    store().handleEvent(err('E_IO', '磁盘满'));
    expect(store().status).toBe('failed');
    expect(store().error).toEqual({ code: 'E_IO', message: '磁盘满' });
  });

  it('TC-UI-012 succeeded → idle：再次备份重置表单', async () => {
    await toRunning();
    store().handleEvent(backupResult);
    store().again();
    expect(store().status).toBe('idle');
    expect(store().form.source).toBe('');
    expect(store().result).toBeNull();
  });

  it('TC-UI-013 failed → ready：重试保留表单', async () => {
    await toRunning();
    store().handleEvent(err('E_IO'));
    store().retry();
    expect(store().status).toBe('ready');
    expect(store().form.source).toBe('/data');
    expect(store().error).toBeNull();
  });

  it('TC-UI-014 cancelled → ready：返回编辑', async () => {
    await toRunning();
    await store().cancel();
    store().handleEvent(err('E_CANCELLED'));
    store().backToReady();
    expect(store().status).toBe('ready');
  });

  it('TC-UI-015 还原：editing → ready → awaitingPassword → running', async () => {
    const enc = fakeBackups.find((b) => b.encrypted)!;
    store().beginRestore(enc);
    expect(store().status).toBe('editing');
    store().setRestoreTarget('/out');
    expect(store().status).toBe('ready');
    await store().requestRestore();
    expect(store().status).toBe('awaitingPassword');
    store().setRestorePassword('mock');
    await store().submitRestorePassword();
    expect(api.restore.start).toHaveBeenCalledWith({ backup: enc.name, target: '/out' }, 'mock');
    expect(store().status).toBe('running');
    expect(store().restore?.password).toBe('');
  });

  it('TC-UI-016 还原口令连续 3 次 E_AUTH 回到 ready', async () => {
    const enc = fakeBackups.find((b) => b.encrypted)!;
    store().beginRestore(enc);
    store().setRestoreTarget('/out');
    store().setRestorePassword('x');
    await store().requestRestore();
    await store().submitRestorePassword();

    for (const n of [1, 2]) {
      store().handleEvent(err('E_AUTH', '口令错误'));
      expect(store().status).toBe('awaitingPassword');
      expect(store().authFailures).toBe(n);
      store().setRestorePassword('x');
      await store().submitRestorePassword();
    }
    store().handleEvent(err('E_AUTH', '口令错误'));
    expect(store().status).toBe('ready');
    expect(store().authFailures).toBe(0);
  });

  it('TC-UI-017 E_PASSWORD_REQUIRED 回到 awaitingPassword 且不计失败次数', async () => {
    const enc = fakeBackups.find((b) => b.encrypted)!;
    store().beginRestore(enc);
    store().setRestoreTarget('/out');
    // 模拟加密备份未提供口令直接启动（防御：后端主动要求口令）。
    useJobStore.setState({ status: 'running', kind: 'restore' });
    store().handleEvent(err('E_PASSWORD_REQUIRED', '需要口令'));
    expect(store().status).toBe('awaitingPassword');
    expect(store().authFailures).toBe(0);
  });

  it('TC-UI-018 未加密备份直接 running，不传口令', async () => {
    const plain = fakeBackups.find((b) => !b.encrypted)!;
    store().beginRestore(plain);
    store().setRestoreTarget('/out');
    await store().requestRestore();
    expect(api.restore.start).toHaveBeenCalledWith(
      { backup: plain.name, target: '/out' },
      undefined,
    );
    expect(store().status).toBe('running');
  });

  it('TC-UI-019 口令只在提交时传递：ready 编辑阶段不调用 start', async () => {
    store().setEncrypt(true);
    store().setSource('/data');
    store().setPassword('p');
    store().setConfirm('p');
    expect(api.backup.start).not.toHaveBeenCalled();
  });

  it('TC-UI-020 dry-run 预览：成功写入 summary，E_USAGE 写入 error', async () => {
    store().setSource('/data');
    await store().refreshPreview();
    expect(store().preview.summary?.files).toBe(2);
    vi.mocked(api.backup.dryRun).mockRejectedValueOnce(new Error('[E_USAGE] 类型列表为空'));
    await store().refreshPreview();
    expect(store().preview.error?.code).toBe('E_USAGE');
  });
});
