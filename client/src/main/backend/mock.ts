import { EventEmitter } from 'node:events';
import path from 'node:path';
import { PassThrough, Writable } from 'node:stream';
import { EXIT_CODE_OF, type BackupInfo, type ErrorCode } from '../../shared/protocol';
import { createSpawnRunner, type BackendRunner, type SpawnFn } from './runner';

/**
 * mock 后端：用假子进程按脚本回放 JSONL，复用真实 runner 的全部解析、取消与兜底逻辑。
 * 用于单测，以及 C2 在没有真实二进制时开发界面（环境变量 DATABACKUP_MOCK=1，见 interaction.md §2.1）。
 */

export interface MockStep {
  /** 写到 stdout 的一行事件；可以故意不符合协议。 */
  event?: object;
  /** 原样写到 stdout 的内容（用于构造坏行、半行、被切开的多字节字符）。 */
  raw?: string | Buffer;
  stderr?: string;
  /** 本步之前等待的毫秒数，缺省 0。 */
  delayMs?: number;
}

export interface MockScript {
  steps: MockStep[];
  /** 全部步骤回放完后的退出方式，缺省退出码 0。 */
  exit?: { code: number | null; signal?: NodeJS.Signals | null };
  /** 模拟启动失败的 errno（如 'ENOENT'）。 */
  spawnError?: string;
  /** 收到 SIGTERM：'cancel'（缺省）输出 E_CANCELLED 并以 4 退出；'ignore' 不理会，只能被 SIGKILL。 */
  onSigterm?: 'cancel' | 'ignore';
  /** 脚本完整回放（未被取消）后执行，用于更新 mock 状态。 */
  onFinish?: () => void;
}

/** 根据参数（不含 --json）与 stdin 内容（口令）给出要回放的脚本。 */
export type MockScriptFn = (args: readonly string[], stdin: string) => MockScript;

/** 假子进程：读完 stdin 后选定脚本，再依次回放。 */
export class FakeChild extends EventEmitter {
  readonly stdout = new PassThrough();
  readonly stderr = new PassThrough();
  readonly stdin: Writable;
  /** 收到的信号，按顺序。 */
  readonly signals: NodeJS.Signals[] = [];
  /** stdin 的完整内容（只用于断言与脚本选择）。 */
  stdinText = '';
  readonly args: readonly string[];

  private script: MockScript | undefined;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private exited = false;

  constructor(
    args: readonly string[],
    private readonly scriptFn: MockScriptFn,
  ) {
    super();
    this.args = args[0] === '--json' ? args.slice(1) : args;
    const chunks: Buffer[] = [];
    this.stdin = new Writable({
      write: (chunk: Buffer, _enc, cb) => {
        // runner 写完会把缓冲区清零，这里必须拷贝。
        chunks.push(Buffer.from(chunk));
        cb();
      },
      final: (cb) => {
        this.stdinText = Buffer.concat(chunks).toString('utf8');
        cb();
        this.begin();
      },
    });
  }

  kill(signal: NodeJS.Signals): boolean {
    if (this.exited) return false;
    this.signals.push(signal);
    if (signal === 'SIGKILL') {
      this.stop();
      this.exit(null, 'SIGKILL');
    } else if (this.script?.onSigterm !== 'ignore') {
      this.stop();
      this.writeLine({ type: 'error', code: 'E_CANCELLED', message: '已取消' });
      this.exit(EXIT_CODE_OF.E_CANCELLED, null);
    }
    return true;
  }

  private begin(): void {
    if (this.exited) return;
    const script = this.scriptFn(this.args, this.stdinText);
    this.script = script;
    if (script.spawnError) {
      const err: NodeJS.ErrnoException = new Error(`spawn databackup ${script.spawnError}`);
      err.code = script.spawnError;
      this.exited = true;
      this.emit('error', err);
      return;
    }
    this.emit('spawn');
    this.play(0);
  }

  private play(i: number): void {
    const script = this.script;
    if (!script || this.exited) return;
    if (i >= script.steps.length) {
      script.onFinish?.();
      this.exit(script.exit?.code ?? 0, script.exit?.signal ?? null);
      return;
    }
    const step = script.steps[i];
    this.timer = setTimeout(() => {
      if (step.stderr !== undefined) this.stderr.write(step.stderr);
      if (step.raw !== undefined) this.stdout.write(step.raw);
      if (step.event !== undefined) this.writeLine(step.event);
      this.play(i + 1);
    }, step.delayMs ?? 0);
  }

  private writeLine(event: object): void {
    this.stdout.write(`${JSON.stringify(event)}\n`);
  }

  private stop(): void {
    clearTimeout(this.timer);
  }

  private exit(code: number | null, signal: NodeJS.Signals | null): void {
    if (this.exited) return;
    this.exited = true;
    this.emit('exit', code, signal);
    let open = 2;
    const onEnd = (): void => {
      open -= 1;
      if (open === 0) this.emit('close', code, signal);
    };
    for (const s of [this.stdout, this.stderr]) {
      s.once('end', onEnd);
      s.end();
      s.resume();
    }
  }
}

/** 生成注入 runner 的 spawn：每次调用创建一个 FakeChild。 */
export function mockSpawn(scriptFn: MockScriptFn, onChild?: (child: FakeChild) => void): SpawnFn {
  return (_command, args) => {
    const child = new FakeChild(args, scriptFn);
    onChild?.(child);
    return child;
  };
}

export function createMockRunner(
  scriptFn: MockScriptFn = createDefaultMockScript(),
  log?: (message: string) => void,
): BackendRunner {
  return createSpawnRunner({
    resolveBin: () => 'databackup-mock',
    spawn: mockSpawn(scriptFn),
    log,
  });
}

// ---- 默认脚本（DATABACKUP_MOCK=1） --------------------------------------------

/** mock 中加密备份的正确口令。 */
export const MOCK_PASSWORD = 'mock';

/** 备份名为此值时模拟后端崩溃（无 result、退出码 1）。 */
export const MOCK_CRASH_NAME = 'crash';

function errorScript(code: ErrorCode, message: string): MockScript {
  return {
    steps: [{ event: { type: 'error', code, message } }],
    exit: { code: EXIT_CODE_OF[code] },
  };
}

function flagValue(args: readonly string[], flag: string): string | undefined {
  const i = args.indexOf(flag);
  return i >= 0 ? args[i + 1] : undefined;
}

function sampleBackups(repo: string): BackupInfo[] {
  return [
    {
      name: 'docs-20261015-235500',
      path: `${repo}/docs-20261015-235500.dbk`,
      outputBytes: 3170304,
      createdAt: '2026-10-15T23:55:00+08:00',
      source: '/home/mock/docs',
      compressed: true,
      encrypted: true,
      files: 120,
      dirs: 14,
      bytes: 5242880,
      formatVersion: 1,
      corrupt: false,
    },
    {
      name: 'photos-20261010-090000',
      path: `${repo}/photos-20261010-090000.dbk`,
      outputBytes: 52428800,
      createdAt: '2026-10-10T09:00:00+08:00',
      source: '/home/mock/photos',
      compressed: false,
      encrypted: false,
      files: 340,
      dirs: 9,
      bytes: 52428800,
      formatVersion: 1,
      corrupt: false,
    },
    {
      name: 'old',
      path: `${repo}/old.dbk`,
      outputBytes: 4096,
      createdAt: '2026-10-01T09:00:00+08:00',
      source: '',
      compressed: false,
      encrypted: false,
      files: 0,
      dirs: 0,
      bytes: 0,
      formatVersion: 0,
      corrupt: true,
    },
  ];
}

function progressSteps(
  stepMs: number,
  phase: 'scan' | 'pack' | 'restore',
  count: number,
  totalFiles: number,
  totalBytes: number,
): MockStep[] {
  const withTotal = phase !== 'scan';
  return Array.from({ length: count }, (_, k) => {
    const n = k + 1;
    return {
      delayMs: stepMs,
      event: {
        type: 'progress',
        phase,
        files: Math.round((totalFiles * n) / count),
        bytes: Math.round((totalBytes * n) / count),
        ...(withTotal ? { totalFiles, totalBytes } : {}),
        current: `dir${n % 4}/file-${n}.txt`,
      },
    };
  });
}

/**
 * 有状态的默认脚本：config set 会改变 repo，delete 会移除列表项，成功的 backup 会加入列表。
 * 约定：备份名 `crash` → 崩溃；还原加密备份必须用口令 `mock`，否则 E_AUTH；其余按契约正常返回。
 * stepMs 为相邻进度事件的间隔（缺省 100ms，与后端节流上限一致）。
 */
export function createDefaultMockScript({ stepMs = 100 }: { stepMs?: number } = {}): MockScriptFn {
  let repo = '/home/mock/DataBackup';
  let backups = sampleBackups(repo);
  const find = (name: string): BackupInfo | undefined =>
    backups.find((b) => b.name === name.replace(/\.dbk$/, '') || b.path === name);

  const backup = (args: readonly string[], stdin: string): MockScript => {
    const source = flagValue(args, '--source') ?? '';
    const files = 120;
    const bytes = 5242880;
    if (args.includes('--dry-run')) {
      return {
        steps: [
          ...progressSteps(stepMs, 'scan', 3, files, bytes),
          { event: { type: 'result', command: 'dry-run', files, dirs: 14, bytes, skipped: 7 } },
        ],
      };
    }
    const encrypted = args.includes('--encrypt');
    if (encrypted && stdin.trim() === '') return errorScript('E_USAGE', '口令不能为空');
    const name = flagValue(args, '--name') ?? `${path.posix.basename(source) || 'root'}-mock`;
    const scan = progressSteps(stepMs, 'scan', 5, files, bytes);
    const pack = progressSteps(stepMs, 'pack', 20, files, bytes);
    if (name === MOCK_CRASH_NAME) {
      return {
        steps: [...scan, ...pack.slice(0, 5), { stderr: 'panic: mock crash\n' }],
        exit: { code: 1 },
      };
    }
    if (find(name)) return errorScript('E_EXISTS', `同名备份已存在：${name}`);
    const compressed = args.includes('--compress');
    const info: BackupInfo = {
      name,
      path: `${repo}/${name}.dbk`,
      outputBytes: compressed ? 3170304 : bytes,
      createdAt: new Date().toISOString().replace(/\.\d+Z$/, 'Z'),
      source,
      compressed,
      encrypted,
      files,
      dirs: 14,
      bytes,
      formatVersion: 1,
      corrupt: false,
    };
    return {
      steps: [
        ...scan,
        ...pack.slice(0, 10),
        { event: { type: 'log', level: 'warn', message: '跳过不可读项：dir1/secret.txt' } },
        ...pack.slice(10),
        {
          event: {
            type: 'result',
            command: 'backup',
            name,
            path: info.path,
            files,
            dirs: 14,
            bytes,
            outputBytes: info.outputBytes,
            compressed,
            encrypted,
            skipped: 7,
            warnings: 1,
            ms: 2500,
          },
        },
      ],
      onFinish: () => {
        backups = [info, ...backups];
      },
    };
  };

  const restore = (args: readonly string[], stdin: string): MockScript => {
    const name = flagValue(args, '--backup') ?? '';
    const target = flagValue(args, '--target') ?? '';
    const b = find(name);
    if (!b) return errorScript('E_NOT_FOUND', `备份不存在：${name}`);
    if (b.corrupt) return errorScript('E_CORRUPT', '备份文件已损坏');
    if (b.encrypted) {
      if (!args.includes('--password-stdin')) {
        return errorScript('E_PASSWORD_REQUIRED', '该备份已加密，需要口令');
      }
      if (stdin.replace(/\r?\n$/, '') !== MOCK_PASSWORD) {
        return errorScript('E_AUTH', '口令错误或备份已被篡改');
      }
    }
    return {
      steps: [
        ...progressSteps(stepMs, 'restore', 20, b.files, b.bytes),
        {
          event: {
            type: 'result',
            command: 'restore',
            target,
            files: b.files,
            dirs: b.dirs,
            bytes: b.bytes,
            warnings: 0,
            ms: 2000,
          },
        },
      ],
    };
  };

  return (args, stdin) => {
    const [cmd, ...rest] = args;
    switch (cmd) {
      case '--version':
        return {
          steps: [{ event: { type: 'result', command: 'version', version: '0.0.0-mock' } }],
        };
      case 'config':
        if (rest[0] === 'set' && rest[1] === 'repo' && rest[2]) {
          repo = rest[2];
        }
        return { steps: [{ event: { type: 'result', command: 'config', repo } }] };
      case 'list':
        return { steps: [{ event: { type: 'result', command: 'list', repo, backups } }] };
      case 'info': {
        const b = find(rest[0] ?? '');
        if (!b) return errorScript('E_NOT_FOUND', `备份不存在：${rest[0]}`);
        return { steps: [{ event: { type: 'result', command: 'info', ...b } }] };
      }
      case 'delete': {
        const b = find(rest[0] ?? '');
        if (!b) return errorScript('E_NOT_FOUND', `备份不存在：${rest[0]}`);
        backups = backups.filter((x) => x !== b);
        return {
          steps: [{ event: { type: 'result', command: 'delete', name: b.name, path: b.path } }],
        };
      }
      case 'backup':
        return backup(rest, stdin);
      case 'restore':
        return restore(rest, stdin);
      default:
        return errorScript('E_USAGE', `未知子命令：${cmd}`);
    }
  };
}
