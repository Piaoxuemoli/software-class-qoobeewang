// @vitest-environment node
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FakeChild, mockSpawn, type MockScript, type MockScriptFn } from './mock';
import {
  CANCEL_GRACE_MS,
  createSpawnRunner,
  type RunOutcome,
  type SpawnFn,
  type StreamEvent,
} from './runner';
import { parseEventLine } from './schema';

const BIN = '/opt/databackup/bin/databackup';

const versionLine = { type: 'result', command: 'version', version: '0.1.0' };
const scanLine = { type: 'progress', phase: 'scan', files: 1, bytes: 10, current: 'a.txt' };
const packLine = {
  type: 'progress',
  phase: 'pack',
  files: 1,
  bytes: 10,
  totalFiles: 1,
  totalBytes: 10,
  current: 'a.txt',
};
const warnLine = { type: 'log', level: 'warn', message: '跳过不可读项' };

function setup(script: MockScript | MockScriptFn, spawnOverride?: SpawnFn) {
  const children: FakeChild[] = [];
  const logs: string[] = [];
  const events: StreamEvent[] = [];
  const fn = typeof script === 'function' ? script : () => script;
  const spawnCalls: { command: string; args: readonly string[] }[] = [];
  const inner = spawnOverride ?? mockSpawn(fn, (c) => children.push(c));
  const runner = createSpawnRunner({
    resolveBin: () => BIN,
    spawn: (command, args) => {
      spawnCalls.push({ command, args });
      return inner(command, args);
    },
    log: (m) => logs.push(m),
  });
  const run = (args: string[] = ['list'], password?: string) =>
    runner.run(args, { password, onEvent: (e) => events.push(e) });
  return { runner, run, children, logs, events, spawnCalls };
}

function errorOf(o: RunOutcome) {
  if (o.ok) throw new Error(`expected failure, got ${JSON.stringify(o.result)}`);
  return o.error;
}

afterEach(() => {
  vi.useRealTimers();
});

describe('parseEventLine', () => {
  it('accepts every event kind and strips unknown fields', () => {
    expect(parseEventLine(JSON.stringify({ ...scanLine, extra: 1 }))).toEqual({
      kind: 'progress',
      event: scanLine,
    });
    expect(parseEventLine(JSON.stringify(warnLine))).toEqual({ kind: 'log', event: warnLine });
    expect(parseEventLine('{"type":"error","code":"E_AUTH","message":"x"}').kind).toBe('error');
    expect(parseEventLine(JSON.stringify(versionLine))).toEqual({
      kind: 'result',
      event: versionLine,
    });
  });

  it('reports unknown event types separately (ignored per contract §5)', () => {
    expect(parseEventLine('{"type":"heartbeat"}')).toEqual({ kind: 'unknown', type: 'heartbeat' });
  });

  it('flags malformed and mistyped lines as invalid without throwing', () => {
    for (const line of [
      'not json',
      '[1,2]',
      '"str"',
      '{"no":"type"}',
      '{"type":"progress","phase":"scan","files":"1","bytes":0,"current":""}',
      '{"type":"progress","phase":"zip","files":1,"bytes":0,"current":""}',
      '{"type":"result","command":"teleport"}',
      '{"type":"result","command":"backup","name":"x"}',
      '{"type":"log","level":"debug","message":"x"}',
    ]) {
      expect(parseEventLine(line).kind, line).toBe('invalid');
    }
  });
});

describe('createSpawnRunner', () => {
  it('spawns the resolved binary with --json first and an argument array', async () => {
    const s = setup({ steps: [{ event: versionLine }] });
    await s.run(['backup', '--source', '/a b; rm -rf /']).done;
    expect(s.spawnCalls).toEqual([
      { command: BIN, args: ['--json', 'backup', '--source', '/a b; rm -rf /'] },
    ]);
  });

  it('streams progress/log events in order and resolves with the final result', async () => {
    const s = setup({
      steps: [
        { event: scanLine },
        { event: warnLine },
        { event: packLine },
        { event: versionLine },
      ],
    });
    const h = s.run();
    expect(await h.started).toBe(true);
    expect(await h.done).toEqual({ ok: true, result: versionLine });
    expect(s.events).toEqual([scanLine, warnLine, packLine]);
  });

  it('reassembles lines split across chunks, including a split multi-byte character', async () => {
    const line = Buffer.from(
      `${JSON.stringify({ type: 'log', level: 'info', message: '仓库目录不存在，已创建' })}\n`,
    );
    const cut = line.indexOf(Buffer.from('仓')) + 1;
    const s = setup({
      steps: [
        { raw: line.subarray(0, cut) },
        { raw: line.subarray(cut) },
        { raw: '{"type":"result","command":"version",' },
        { raw: '"version":"0.1.0"}' },
      ],
    });
    expect(await s.run().done).toEqual({ ok: true, result: versionLine });
    expect(s.events).toEqual([{ type: 'log', level: 'info', message: '仓库目录不存在，已创建' }]);
  });

  it('ignores unknown event types and unknown fields', async () => {
    const s = setup({
      steps: [{ raw: '{"type":"heartbeat","n":1}\n' }, { event: { ...versionLine, future: true } }],
    });
    expect(await s.run().done).toEqual({ ok: true, result: versionLine });
  });

  it('treats a malformed line as a protocol error even if a result follows', async () => {
    const s = setup({ steps: [{ raw: 'garbage\n' }, { event: versionLine }] });
    const err = errorOf(await s.run().done);
    expect(err.code).toBe('E_INTERNAL');
    expect(err.message).toContain('不符合协议');
    expect(err.message).toContain('第 1 行');
  });

  it('treats a known event with invalid fields as a protocol error', async () => {
    const s = setup({
      steps: [{ event: { type: 'progress', phase: 'scan' } }, { event: versionLine }],
    });
    expect(errorOf(await s.run().done).code).toBe('E_INTERNAL');
  });

  it('requires the result/error to be the last line', async () => {
    const s = setup({ steps: [{ event: versionLine }, { event: scanLine }] });
    const err = errorOf(await s.run().done);
    expect(err.code).toBe('E_INTERNAL');
    expect(err.message).toContain('之后仍有输出');
  });

  it('rejects two terminal events', async () => {
    const s = setup({ steps: [{ event: versionLine }, { event: versionLine }] });
    expect(errorOf(await s.run().done).code).toBe('E_INTERNAL');
  });

  it('passes a final error event through with its message', async () => {
    const s = setup({
      steps: [{ event: { type: 'error', code: 'E_AUTH', message: '口令错误或备份已被篡改' } }],
      exit: { code: 3 },
    });
    expect(errorOf(await s.run().done)).toEqual({
      code: 'E_AUTH',
      message: '口令错误或备份已被篡改',
    });
  });

  it('reports the final error event even after an earlier malformed line', async () => {
    const s = setup({
      steps: [{ raw: '{oops\n' }, { event: { type: 'error', code: 'E_IO', message: '磁盘已满' } }],
      exit: { code: 1 },
    });
    expect(errorOf(await s.run().done).code).toBe('E_IO');
  });

  it.each([
    [3, 'E_AUTH'],
    [1, 'E_INTERNAL'],
    [7, 'E_INTERNAL'],
  ])('normalizes an unknown error code by exit code %i → %s', async (exit, code) => {
    const s = setup({
      steps: [{ event: { type: 'error', code: 'E_FUTURE', message: '新错误' } }],
      exit: { code: exit },
    });
    expect(errorOf(await s.run().done)).toEqual({ code, message: '新错误' });
  });

  it.each([
    [1, 'E_INTERNAL'],
    [2, 'E_USAGE'],
    [3, 'E_AUTH'],
    [4, 'E_CANCELLED'],
    [0, 'E_INTERNAL'],
    [99, 'E_INTERNAL'],
  ])('maps exit code %i without an error event to %s', async (exit, code) => {
    const s = setup({ steps: [{ event: scanLine }], exit: { code: exit } });
    expect(errorOf(await s.run().done).code).toBe(code);
  });

  it('maps termination by signal to E_CANCELLED', async () => {
    const s = setup({ steps: [], exit: { code: null, signal: 'SIGKILL' } });
    const err = errorOf(await s.run().done);
    expect(err.code).toBe('E_CANCELLED');
    expect(err.message).toContain('SIGKILL');
  });

  it('does not accept a result paired with a non-zero exit code', async () => {
    const s = setup({ steps: [{ event: versionLine }], exit: { code: 2 } });
    expect(errorOf(await s.run().done).code).toBe('E_USAGE');
  });

  it('appends a bounded stderr tail to E_INTERNAL crashes and logs it', async () => {
    const noise = 'x'.repeat(20_000);
    const s = setup({ steps: [{ stderr: noise }, { stderr: '\npanic: boom' }], exit: { code: 1 } });
    const err = errorOf(await s.run().done);
    expect(err.code).toBe('E_INTERNAL');
    expect(err.message).toContain('panic: boom');
    expect(err.message.length).toBeLessThan(500);
    expect(s.logs).toHaveLength(1);
    expect(s.logs[0]).toContain('panic: boom');
    expect(s.logs[0].length).toBeLessThan(9_000);
  });

  it('maps a missing binary (ENOENT) to E_BACKEND_UNAVAILABLE', async () => {
    const s = setup({ steps: [], spawnError: 'ENOENT' });
    const h = s.run();
    expect(await h.started).toBe(false);
    const err = errorOf(await h.done);
    expect(err.code).toBe('E_BACKEND_UNAVAILABLE');
    expect(err.message).toContain(BIN);
  });

  it('maps a synchronous spawn failure to E_BACKEND_UNAVAILABLE', async () => {
    const s = setup({ steps: [] }, () => {
      throw Object.assign(new Error('EACCES'), { code: 'EACCES' });
    });
    const h = s.run();
    expect(await h.started).toBe(false);
    expect(errorOf(await h.done).code).toBe('E_BACKEND_UNAVAILABLE');
    h.cancel();
  });
});

describe('password via stdin', () => {
  it('writes the password plus newline to stdin and closes it', async () => {
    const s = setup({ steps: [{ event: versionLine }] });
    await s.run(['restore', '--backup', 'x', '--target', '/t', '--password-stdin'], 's3cret').done;
    expect(s.children[0].stdinText).toBe('s3cret\n');
    expect(s.spawnCalls[0].args).not.toContain('s3cret');
  });

  it('closes stdin without writing anything when no password is given', async () => {
    const s = setup({ steps: [{ event: versionLine }] });
    await s.run().done;
    expect(s.children[0].stdinText).toBe('');
  });

  it('never puts the password into logs, outcomes or spawn arguments', async () => {
    const pw = 'Pa55-unique-password';
    const scripts: MockScript[] = [
      { steps: [{ stderr: 'panic\n' }], exit: { code: 1 } },
      { steps: [{ raw: 'bad\n' }], exit: { code: 0 } },
      { steps: [{ event: { type: 'error', code: 'E_AUTH', message: 'x' } }], exit: { code: 3 } },
      { steps: [], spawnError: 'ENOENT' },
    ];
    for (const script of scripts) {
      const s = setup(script);
      const outcome = await s.run(['restore', '--password-stdin'], pw).done;
      const visible = [JSON.stringify(outcome), ...s.logs, JSON.stringify(s.spawnCalls)];
      for (const text of visible) expect(text).not.toContain(pw);
    }
  });
});

describe('cancellation (contract §3.5)', () => {
  it('sends SIGTERM and resolves E_CANCELLED when the backend exits gracefully', async () => {
    const s = setup({ steps: [{ event: scanLine }, { event: versionLine, delayMs: 60_000 }] });
    const h = s.run();
    await h.started;
    h.cancel();
    h.cancel();
    const err = errorOf(await h.done);
    expect(err.code).toBe('E_CANCELLED');
    expect(s.children[0].signals).toEqual(['SIGTERM']);
  });

  it('escalates to SIGKILL after the grace period if SIGTERM is ignored', async () => {
    vi.useFakeTimers();
    const s = setup({
      steps: [{ event: versionLine, delayMs: 60_000 }],
      onSigterm: 'ignore',
    });
    const h = s.run();
    await h.started;
    h.cancel();
    expect(s.children[0].signals).toEqual(['SIGTERM']);
    await vi.advanceTimersByTimeAsync(CANCEL_GRACE_MS - 1);
    expect(s.children[0].signals).toEqual(['SIGTERM']);
    await vi.advanceTimersByTimeAsync(1);
    expect(s.children[0].signals).toEqual(['SIGTERM', 'SIGKILL']);
    expect(errorOf(await h.done).code).toBe('E_CANCELLED');
  });

  it('does not SIGKILL a process that exited within the grace period', async () => {
    vi.useFakeTimers();
    const s = setup({ steps: [{ event: versionLine, delayMs: 60_000 }] });
    const h = s.run();
    await h.started;
    h.cancel();
    await vi.advanceTimersByTimeAsync(CANCEL_GRACE_MS * 2);
    expect(s.children[0].signals).toEqual(['SIGTERM']);
    expect(errorOf(await h.done).code).toBe('E_CANCELLED');
  });

  it('is a no-op after the process has finished', async () => {
    const s = setup({ steps: [{ event: versionLine }] });
    const h = s.run();
    expect((await h.done).ok).toBe(true);
    h.cancel();
    expect(s.children[0].signals).toEqual([]);
  });
});
