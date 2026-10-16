// @vitest-environment node
import { describe, expect, it } from 'vitest';
import type { StreamEvent } from './runner';
import { createDefaultMockScript, createMockRunner, MOCK_CRASH_NAME, MOCK_PASSWORD } from './mock';

/** 默认 mock 脚本（DATABACKUP_MOCK=1）覆盖 C2 需要的主要场景。 */
function run(args: string[], password?: string) {
  const runner = createMockRunner(createDefaultMockScript({ stepMs: 1 }), () => {});
  const events: StreamEvent[] = [];
  const handle = runner.run(args, { password, onEvent: (e) => events.push(e) });
  return { handle, events, runner };
}

describe('default mock backend', () => {
  it('answers version, config and list', async () => {
    const runner = createMockRunner(createDefaultMockScript({ stepMs: 1 }), () => {});
    expect(await runner.run(['--version']).done).toMatchObject({
      ok: true,
      result: { command: 'version' },
    });
    expect(await runner.run(['config', 'set', 'repo', '/data/r']).done).toEqual({
      ok: true,
      result: { type: 'result', command: 'config', repo: '/data/r' },
    });
    const list = await runner.run(['list']).done;
    expect(list.ok && list.result.command === 'list' && list.result.repo).toBe('/data/r');
  });

  it('streams scan → pack progress and a backup result, then lists the new backup', async () => {
    const r = run([
      'backup',
      '--source',
      '/home/u/docs',
      '--name',
      'new-one',
      '--compress',
      'huffman',
    ]);
    const outcome = await r.handle.done;
    expect(outcome).toMatchObject({ ok: true, result: { command: 'backup', name: 'new-one' } });
    const phases = r.events.flatMap((e) => (e.type === 'progress' ? [e.phase] : []));
    expect(phases[0]).toBe('scan');
    expect(phases.at(-1)).toBe('pack');
    expect(r.events.some((e) => e.type === 'log')).toBe(true);
    const list = await r.runner.run(['list']).done;
    expect(list.ok && list.result.command === 'list' && list.result.backups[0].name).toBe(
      'new-one',
    );
  });

  it('answers dry-run without a password', async () => {
    const r = run(['backup', '--dry-run', '--source', '/x']);
    expect(await r.handle.done).toMatchObject({ ok: true, result: { command: 'dry-run' } });
  });

  it('fails restore of an encrypted backup with a wrong password (E_AUTH)', async () => {
    const r = run(
      ['restore', '--backup', 'docs-20261015-235500', '--target', '/t', '--password-stdin'],
      'wrong',
    );
    expect(await r.handle.done).toMatchObject({ ok: false, error: { code: 'E_AUTH' } });
  });

  it('requires a password for encrypted backups (E_PASSWORD_REQUIRED)', async () => {
    const r = run(['restore', '--backup', 'docs-20261015-235500', '--target', '/t']);
    expect(await r.handle.done).toMatchObject({
      ok: false,
      error: { code: 'E_PASSWORD_REQUIRED' },
    });
  });

  it('restores with the mock password', async () => {
    const r = run(
      ['restore', '--backup', 'docs-20261015-235500', '--target', '/t', '--password-stdin'],
      MOCK_PASSWORD,
    );
    expect(await r.handle.done).toMatchObject({
      ok: true,
      result: { command: 'restore', target: '/t' },
    });
    expect(r.events.every((e) => e.type !== 'progress' || e.phase === 'restore')).toBe(true);
  });

  it('crashes without a result for the crash name', async () => {
    const r = run(['backup', '--source', '/x', '--name', MOCK_CRASH_NAME]);
    expect(await r.handle.done).toMatchObject({ ok: false, error: { code: 'E_INTERNAL' } });
  });

  it('honours cancellation with E_CANCELLED', async () => {
    const r = run(['backup', '--source', '/x', '--name', 'slow']);
    await r.handle.started;
    r.handle.cancel();
    expect(await r.handle.done).toMatchObject({ ok: false, error: { code: 'E_CANCELLED' } });
  });

  it('reports E_NOT_FOUND for unknown backups and deletes known ones', async () => {
    const runner = createMockRunner(createDefaultMockScript({ stepMs: 1 }), () => {});
    expect(await runner.run(['info', 'missing']).done).toMatchObject({
      ok: false,
      error: { code: 'E_NOT_FOUND' },
    });
    expect(await runner.run(['delete', 'old']).done).toMatchObject({
      ok: true,
      result: { command: 'delete', name: 'old' },
    });
    expect(await runner.run(['info', 'old']).done).toMatchObject({ ok: false });
  });
});
