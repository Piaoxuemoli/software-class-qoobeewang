// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { parseApiError, type JobId } from '../../shared/api';
import type { BackendEvent } from '../../shared/protocol';
import { JobManager } from './jobs';
import { createMockRunner, type MockScript, type MockScriptFn } from './mock';

const backupResult = {
  type: 'result',
  command: 'backup',
  name: 'n',
  path: '/r/n.dbk',
  files: 1,
  dirs: 0,
  bytes: 1,
  outputBytes: 1,
  compressed: false,
  encrypted: false,
  skipped: 0,
  warnings: 0,
  ms: 1,
} as const;
const dryRunResult = {
  type: 'result',
  command: 'dry-run',
  files: 3,
  dirs: 1,
  bytes: 9,
  skipped: 2,
};
const scan = { type: 'progress', phase: 'scan', files: 1, bytes: 1, current: 'a' } as const;

/** 长任务脚本：先发一条进度，长时间后才出结果（期间可取消）。 */
const slowBackup: MockScript = {
  steps: [{ event: scan }, { event: backupResult, delayMs: 60_000 }],
};

function managerWith(script: MockScriptFn) {
  let n = 0;
  const jobs = new JobManager(
    createMockRunner(script, () => {}),
    () => `job-${++n}`,
  );
  const received: { jobId: JobId; event: BackendEvent }[] = [];
  const onEvent = (jobId: JobId, event: BackendEvent) => received.push({ jobId, event });
  return { jobs, received, onEvent };
}

function byCommand(table: Record<string, MockScript>): MockScriptFn {
  return (args) => {
    const key = args.includes('--dry-run') ? 'dry-run' : args[0];
    return table[key] ?? { steps: [], exit: { code: 1 } };
  };
}

async function rejection(p: Promise<unknown>) {
  try {
    await p;
  } catch (err) {
    return parseApiError(err);
  }
  throw new Error('expected rejection');
}

async function waitFor(cond: () => boolean): Promise<void> {
  for (let i = 0; i < 200 && !cond(); i++) await new Promise((r) => setTimeout(r, 1));
  expect(cond()).toBe(true);
}

describe('JobManager long jobs', () => {
  it('returns a job id, forwards progress and ends with exactly one result event', async () => {
    const m = managerWith(
      byCommand({ backup: { steps: [{ event: scan }, { event: backupResult }] } }),
    );
    const id = await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    expect(id).toBe('job-1');
    await waitFor(() => !m.jobs.hasActiveJob() && m.received.length === 2);
    expect(m.received).toEqual([
      { jobId: id, event: scan },
      { jobId: id, event: backupResult },
    ]);
  });

  it('rejects a second backup/restore while one is running (E_BUSY)', async () => {
    const m = managerWith(byCommand({ backup: slowBackup, restore: slowBackup }));
    const id = await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    expect(m.jobs.hasActiveJob()).toBe(true);
    expect((await rejection(m.jobs.start('backup', ['backup'], { onEvent: m.onEvent }))).code).toBe(
      'E_BUSY',
    );
    expect(
      (await rejection(m.jobs.start('restore', ['restore'], { onEvent: m.onEvent }))).code,
    ).toBe('E_BUSY');
    await m.jobs.cancel(id);
  });

  it('allows a new job after the previous one finished', async () => {
    const m = managerWith(byCommand({ backup: { steps: [{ event: backupResult }] } }));
    await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    await waitFor(() => !m.jobs.hasActiveJob());
    await expect(m.jobs.start('backup', ['backup'], { onEvent: m.onEvent })).resolves.toBe('job-2');
  });

  it('does not restrict dry-run and queries while a job is running', async () => {
    const m = managerWith(
      byCommand({
        backup: slowBackup,
        'dry-run': { steps: [{ event: dryRunResult }] },
        list: { steps: [{ event: { type: 'result', command: 'list', repo: '/r', backups: [] } }] },
      }),
    );
    const id = await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    await expect(m.jobs.runDryRun(['backup', '--dry-run'])).resolves.toEqual({
      files: 3,
      dirs: 1,
      bytes: 9,
      skipped: 2,
    });
    await expect(m.jobs.runOnce(['list'], 'list')).resolves.toMatchObject({ backups: [] });
    await m.jobs.cancel(id);
  });

  it('rejects start with E_BACKEND_UNAVAILABLE when the binary is missing and frees the slot', async () => {
    const m = managerWith(() => ({ steps: [], spawnError: 'ENOENT' }));
    const err = await rejection(m.jobs.start('backup', ['backup'], { onEvent: m.onEvent }));
    expect(err.code).toBe('E_BACKEND_UNAVAILABLE');
    expect(m.jobs.hasActiveJob()).toBe(false);
    expect(m.received).toEqual([]);
  });

  it('cancel() waits for the backend to exit and the job ends with E_CANCELLED', async () => {
    const m = managerWith(byCommand({ backup: slowBackup }));
    const id = await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    await m.jobs.cancel(id);
    expect(m.jobs.hasActiveJob()).toBe(false);
    await waitFor(() => m.received.at(-1)?.event.type === 'error');
    expect(m.received.at(-1)?.event).toEqual({
      type: 'error',
      code: 'E_CANCELLED',
      message: '已取消',
    });
  });

  it('cancel() of an unknown or finished job is a no-op', async () => {
    const m = managerWith(byCommand({}));
    await expect(m.jobs.cancel('nope')).resolves.toBeUndefined();
  });

  it('reports a result of the wrong command as an E_INTERNAL error event', async () => {
    const m = managerWith(byCommand({ backup: { steps: [{ event: dryRunResult }] } }));
    await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    await waitFor(() => m.received.length === 1);
    expect(m.received[0].event).toMatchObject({ type: 'error', code: 'E_INTERNAL' });
  });

  it('cancelActive() and cancelAll() stop running processes', async () => {
    const m = managerWith(byCommand({ backup: slowBackup, 'dry-run': slowBackup }));
    await m.jobs.start('backup', ['backup'], { onEvent: m.onEvent });
    await m.jobs.cancelActive();
    expect(m.jobs.hasActiveJob()).toBe(false);
    const dry = m.jobs.runDryRun(['backup', '--dry-run']);
    expect(m.jobs.hasRunning()).toBe(true);
    await m.jobs.cancelAll();
    expect((await rejection(dry)).code).toBe('E_CANCELLED');
    expect(m.jobs.hasRunning()).toBe(false);
  });
});

describe('JobManager dry-run and queries', () => {
  it('a new dry-run cancels the unfinished previous one', async () => {
    let calls = 0;
    const m = managerWith(() =>
      ++calls === 1
        ? { steps: [{ event: dryRunResult, delayMs: 60_000 }] }
        : { steps: [{ event: dryRunResult }] },
    );
    const first = m.jobs.runDryRun(['backup', '--dry-run']);
    const second = m.jobs.runDryRun(['backup', '--dry-run']);
    expect((await rejection(first)).code).toBe('E_CANCELLED');
    await expect(second).resolves.toEqual({ files: 3, dirs: 1, bytes: 9, skipped: 2 });
  });

  it('rejects with the backend error code', async () => {
    const m = managerWith(() => ({
      steps: [{ event: { type: 'error', code: 'E_NOT_FOUND', message: '备份不存在' } }],
      exit: { code: 1 },
    }));
    expect(await rejection(m.jobs.runOnce(['info', 'x'], 'info'))).toEqual({
      code: 'E_NOT_FOUND',
      message: '备份不存在',
    });
  });

  it('rejects a result of an unexpected command with E_INTERNAL', async () => {
    const m = managerWith(() => ({ steps: [{ event: dryRunResult }] }));
    expect((await rejection(m.jobs.runOnce(['list'], 'list'))).code).toBe('E_INTERNAL');
  });
});
