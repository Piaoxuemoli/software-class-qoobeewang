import { spawn as nodeSpawn } from 'node:child_process';
import type { Readable, Writable } from 'node:stream';
import type { ApiError } from '../../shared/api';
import {
  FALLBACK_ERROR_CODE_OF_EXIT,
  type LogEvent,
  type ProgressEvent,
  type ResultEvent,
} from '../../shared/protocol';
import { createLineSplitter } from './jsonl';
import { isErrorCode, parseEventLine, type RawErrorEvent } from './schema';

/** 取消时 SIGTERM 之后等待多久再 SIGKILL（契约 §3.5）。 */
export const CANCEL_GRACE_MS = 3000;

/** 保留的 stderr 尾部长度（字符），只用于诊断。 */
export const STDERR_TAIL_LIMIT = 8 * 1024;

/** 附在 E_INTERNAL 说明后面的 stderr 片段长度上限。 */
const STDERR_IN_MESSAGE_LIMIT = 300;

/** 结果之前的流式事件。 */
export type StreamEvent = ProgressEvent | LogEvent;

export type RunOutcome = { ok: true; result: ResultEvent } | { ok: false; error: ApiError };

export interface RunOptions {
  /** 有值时追加 '\n' 写入 stdin 后立即关闭 stdin；调用方需自行在参数里给出 --password-stdin。 */
  password?: string;
  onEvent?: (event: StreamEvent) => void;
}

export interface RunHandle {
  /** 进程成功启动为 true；二进制缺失或无法启动为 false（此时 done 为 E_BACKEND_UNAVAILABLE）。 */
  readonly started: Promise<boolean>;
  /** 终态，永不 reject。 */
  readonly done: Promise<RunOutcome>;
  /** 请求取消：SIGTERM，CANCEL_GRACE_MS 后仍未退出则 SIGKILL。可重复调用。 */
  cancel(): void;
}

/** 后端调用的可注入接口：真实实现见 createSpawnRunner，测试与 C2 开发用 mock.ts。 */
export interface BackendRunner {
  /** args 为子命令及其参数，不含 --json（由 runner 加在最前）。 */
  run(args: readonly string[], options?: RunOptions): RunHandle;
}

/** runner 用到的 ChildProcess 子集，便于测试注入假进程。 */
export interface ChildLike {
  readonly stdin: Writable | null;
  readonly stdout: Readable | null;
  readonly stderr: Readable | null;
  kill(signal: NodeJS.Signals): boolean;
  on(event: 'error', listener: (err: NodeJS.ErrnoException) => void): unknown;
  once(event: 'spawn', listener: () => void): unknown;
  once(event: 'exit', listener: () => void): unknown;
  once(
    event: 'close',
    listener: (code: number | null, signal: NodeJS.Signals | null) => void,
  ): unknown;
}

export type SpawnFn = (command: string, args: readonly string[]) => ChildLike;

export interface SpawnRunnerOptions {
  resolveBin: () => string;
  spawn?: SpawnFn;
  /** 诊断日志（不含口令）；缺省 console.warn。 */
  log?: (message: string) => void;
}

/** 参数数组直接交给 execve，不经 shell。 */
const spawnBackend: SpawnFn = (command, args) =>
  nodeSpawn(command, [...args], { stdio: 'pipe', shell: false, windowsHide: true });

export function createSpawnRunner(options: SpawnRunnerOptions): BackendRunner {
  return { run: (args, runOptions = {}) => startRun(options, args, runOptions) };
}

interface ExitInfo {
  code: number | null;
  signal: NodeJS.Signals | null;
}

/**
 * 收集一次运行的 stdout 行并判定终态（契约 §3 "最后一行恰好是一个 result 或 error"、§4 退出码兜底）。
 * 独立于进程，便于单测。
 */
export class OutputCollector {
  private terminal: ResultEvent | RawErrorEvent | null = null;
  private badLine: string | null = null;
  private trailing = false;
  private lineNo = 0;

  constructor(private readonly onEvent?: (event: StreamEvent) => void) {}

  line(line: string): void {
    this.lineNo += 1;
    const parsed = parseEventLine(line);
    if (parsed.kind === 'unknown') return;
    if (this.terminal) {
      this.trailing = true;
      return;
    }
    switch (parsed.kind) {
      case 'invalid':
        this.badLine ??= `第 ${this.lineNo} 行${parsed.reason}`;
        return;
      case 'progress':
      case 'log':
        this.onEvent?.(parsed.event);
        return;
      case 'error':
      case 'result':
        this.terminal = parsed.event;
        return;
    }
  }

  finish(exit: ExitInfo, cancelRequested: boolean, stderrTail: string): RunOutcome {
    const t = this.terminal;
    const clean = !this.badLine && !this.trailing;
    if (t?.type === 'result' && clean && exit.code === 0 && exit.signal === null) {
      return { ok: true, result: t };
    }
    if (cancelRequested) {
      const message = t?.type === 'error' && t.code === 'E_CANCELLED' ? t.message : '已取消';
      return fail('E_CANCELLED', message);
    }
    if (t?.type === 'error' && !this.trailing) {
      const code = isErrorCode(t.code) ? t.code : fallbackCode(exit);
      return fail(code, t.message);
    }
    if (exit.signal !== null) {
      return fail('E_CANCELLED', `已取消（后端进程被 ${exit.signal} 终止）`);
    }
    const code = fallbackCode(exit);
    let message = this.describeViolation(exit);
    const diag = stderrTail.trim().slice(-STDERR_IN_MESSAGE_LIMIT);
    if (code === 'E_INTERNAL' && diag) message += `\n后端诊断：${diag}`;
    return fail(code, message);
  }

  private describeViolation(exit: ExitInfo): string {
    if (this.trailing) return '后端输出不符合协议：result/error 之后仍有输出';
    if (this.badLine) return `后端输出不符合协议：${this.badLine}`;
    if (this.terminal?.type === 'result') {
      return `后端输出不符合协议：返回了结果但退出码为 ${exit.code}`;
    }
    return `后端异常退出（退出码 ${exit.code}），未返回结果`;
  }
}

function fallbackCode(exit: ExitInfo): ApiError['code'] {
  if (exit.signal !== null) return 'E_CANCELLED';
  return (exit.code !== null && FALLBACK_ERROR_CODE_OF_EXIT[exit.code]) || 'E_INTERNAL';
}

function fail(code: ApiError['code'], message: string): RunOutcome {
  return { ok: false, error: { code, message } };
}

function startRun(
  opts: SpawnRunnerOptions,
  args: readonly string[],
  runOpts: RunOptions,
): RunHandle {
  const log = opts.log ?? ((m: string) => console.warn(m));
  const spawnFn = opts.spawn ?? spawnBackend;
  const label = args.join(' ');
  const collector = new OutputCollector(runOpts.onEvent);
  const splitter = createLineSplitter((line) => collector.line(line));

  let resolveStarted: (ok: boolean) => void = () => {};
  let resolveDone: (o: RunOutcome) => void = () => {};
  const started = new Promise<boolean>((r) => (resolveStarted = r));
  const done = new Promise<RunOutcome>((r) => (resolveDone = r));

  let child: ChildLike | undefined;
  let spawned = false;
  let exited = false;
  let settled = false;
  let cancelRequested = false;
  let killTimer: ReturnType<typeof setTimeout> | undefined;
  let stderrTail = '';

  const settle = (outcome: RunOutcome): void => {
    if (settled) return;
    settled = true;
    clearTimeout(killTimer);
    resolveStarted(false);
    if (!outcome.ok && outcome.error.code !== 'E_CANCELLED') {
      const diag = stderrTail.trim() ? `；stderr: ${stderrTail.trim()}` : '';
      log(`[backend] ${label} 失败 [${outcome.error.code}] ${outcome.error.message}${diag}`);
    }
    resolveDone(outcome);
  };

  const unavailable = (bin: string, err: NodeJS.ErrnoException): RunOutcome =>
    fail(
      'E_BACKEND_UNAVAILABLE',
      err.code === 'ENOENT' ? `找不到后端程序：${bin}` : `无法启动后端程序 ${bin}：${err.message}`,
    );

  let bin = '';
  try {
    bin = opts.resolveBin();
    child = spawnFn(bin, ['--json', ...args]);
  } catch (err) {
    settle(unavailable(bin, err as NodeJS.ErrnoException));
    return { started, done, cancel: () => {} };
  }
  const proc = child;

  proc.once('spawn', () => {
    spawned = true;
    resolveStarted(true);
  });
  // 用 on 而不是 once：未监听的第二个 'error'（如 kill 失败）会让主进程崩溃。
  proc.on('error', (err) => {
    if (!spawned) {
      settle(unavailable(bin, err));
    } else {
      log(`[backend] ${label} 进程错误：${err.message}`);
    }
  });
  proc.once('exit', () => {
    exited = true;
    clearTimeout(killTimer);
  });
  proc.once('close', (code, signal) => {
    exited = true;
    splitter.flush();
    settle(collector.finish({ code, signal }, cancelRequested, stderrTail));
  });

  proc.stdout?.setEncoding('utf8');
  proc.stdout?.on('data', (chunk: string) => splitter.feed(chunk));
  proc.stderr?.setEncoding('utf8');
  proc.stderr?.on('data', (chunk: string) => {
    stderrTail = (stderrTail + chunk).slice(-STDERR_TAIL_LIMIT);
  });

  // 子进程提前退出时写 stdin 会 EPIPE；终态以退出码和 stdout 为准。
  proc.stdin?.on('error', () => {});
  writePassword(proc.stdin, runOpts.password);

  return {
    started,
    done,
    cancel(): void {
      if (settled || cancelRequested) return;
      cancelRequested = true;
      if (exited) return;
      proc.kill('SIGTERM');
      killTimer = setTimeout(() => {
        if (!exited) proc.kill('SIGKILL');
      }, CANCEL_GRACE_MS);
    },
  };
}

/**
 * 口令经 stdin 传给后端（契约 §1 --password-stdin）：写入 "口令\n" 后立即关闭 stdin，
 * 写完把缓冲区清零。JS 字符串无法清零，调用链上只保留局部引用、不落日志。
 */
function writePassword(stdin: Writable | null, password: string | undefined): void {
  if (!stdin) return;
  if (password === undefined) {
    stdin.end();
    return;
  }
  const buf = Buffer.from(`${password}\n`, 'utf8');
  stdin.end(buf, () => buf.fill(0));
}
