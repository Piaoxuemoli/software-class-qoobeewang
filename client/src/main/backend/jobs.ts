import { randomUUID } from 'node:crypto';
import type { ApiError, DryRunSummary, JobId } from '../../shared/api';
import type { BackendEvent, ErrorEvent, ResultEvent } from '../../shared/protocol';
import { ApiFailure, apiFailure } from '../errors';
import type { BackendRunner, RunHandle, RunOutcome } from './runner';
import { isErrorCode } from './schema';

/** 长任务的种类：同一时刻只允许一个（interaction.md §5 并发）。 */
export type JobKind = 'backup' | 'restore';

/** 接收任务的全部事件：progress/log，最后恰好一个 result 或 error。 */
export type JobEventSink = (jobId: JobId, event: BackendEvent) => void;

export interface StartJobOptions {
  password?: string;
  onEvent: JobEventSink;
}

interface ActiveJob {
  id: JobId;
  kind: JobKind;
  handle: RunHandle;
  finished: Promise<void>;
}

type ResultOf<C extends ResultEvent['command']> = Extract<ResultEvent, { command: C }>;

/**
 * 任务注册表（interaction.md §1 backend/jobs.ts）：
 * - backup/restore 是带 jobId 的长任务，同时只允许一个，第二个返回 E_BUSY；
 * - dry-run 新调用取消上一次未完成的调用；
 * - list/info/delete/config/version 不受限。
 */
export class JobManager {
  private active: ActiveJob | null = null;
  private dryRun: RunHandle | null = null;
  private readonly running = new Set<RunHandle>();

  constructor(
    private readonly runner: BackendRunner,
    private readonly newId: () => JobId = randomUUID,
  ) {}

  hasActiveJob(): boolean {
    return this.active !== null;
  }

  /** 是否还有任何后端进程（含 dry-run 与查询）未结束。 */
  hasRunning(): boolean {
    return this.running.size > 0;
  }

  /**
   * 启动长任务；进程成功启动后才返回 jobId，启动失败 reject（E_BACKEND_UNAVAILABLE）。
   * 同步部分把口令交给 runner 后就不再持有，异步等待阶段的闭包里没有口令。
   */
  start(kind: JobKind, args: readonly string[], opts: StartJobOptions): Promise<JobId> {
    if (this.active) {
      return Promise.reject(apiFailure('E_BUSY', '已有备份或还原任务在运行'));
    }
    const id = this.newId();
    const onEvent = opts.onEvent;
    const handle = this.track(
      this.runner.run(args, { password: opts.password, onEvent: (e) => onEvent(id, e) }),
    );
    const finished = handle.done.then((outcome) => {
      if (this.active?.id === id) this.active = null;
      return outcome;
    });
    this.active = { id, kind, handle, finished: finished.then(() => {}) };
    return this.confirmStarted(id, kind, handle, finished, onEvent);
  }

  private async confirmStarted(
    id: JobId,
    kind: JobKind,
    handle: RunHandle,
    finished: Promise<RunOutcome>,
    onEvent: JobEventSink,
  ): Promise<JobId> {
    if (!(await handle.started)) {
      throw new ApiFailure(failureOf(await finished));
    }
    void finished.then((outcome) => onEvent(id, terminalEvent(outcome, kind)));
    return id;
  }

  /** 取消任务并等待其结束；任务不存在或已结束时直接返回。 */
  async cancel(id: JobId): Promise<void> {
    const job = this.active;
    if (!job || job.id !== id) return;
    job.handle.cancel();
    await job.finished;
  }

  /** 取消当前长任务（关闭窗口确认后调用）。 */
  async cancelActive(): Promise<void> {
    if (this.active) await this.cancel(this.active.id);
  }

  /** 取消所有仍在运行的后端进程（退出应用前调用）。 */
  async cancelAll(): Promise<void> {
    const handles = [...this.running];
    for (const h of handles) h.cancel();
    await Promise.all(handles.map((h) => h.done));
  }

  /** backup --dry-run；新调用会取消尚未完成的上一次（上一次以 E_CANCELLED 失败）。 */
  async runDryRun(args: readonly string[]): Promise<DryRunSummary> {
    this.dryRun?.cancel();
    const handle = this.track(this.runner.run(args));
    this.dryRun = handle;
    try {
      const r = await expectResult(handle, 'dry-run');
      return { files: r.files, dirs: r.dirs, bytes: r.bytes, skipped: r.skipped };
    } finally {
      if (this.dryRun === handle) this.dryRun = null;
    }
  }

  /** 运行一次不受并发限制的查询类命令，返回指定 command 的 result。 */
  runOnce<C extends ResultEvent['command']>(
    args: readonly string[],
    command: C,
  ): Promise<ResultOf<C>> {
    return expectResult(this.track(this.runner.run(args)), command);
  }

  private track(handle: RunHandle): RunHandle {
    this.running.add(handle);
    void handle.done.then(() => this.running.delete(handle));
    return handle;
  }
}

async function expectResult<C extends ResultEvent['command']>(
  handle: RunHandle,
  command: C,
): Promise<ResultOf<C>> {
  const outcome = await handle.done;
  if (!outcome.ok) throw new ApiFailure(outcome.error);
  if (outcome.result.command !== command) {
    throw apiFailure('E_INTERNAL', `后端返回了 ${outcome.result.command} 结果，期望 ${command}`);
  }
  return outcome.result as ResultOf<C>;
}

function failureOf(outcome: RunOutcome): ApiError {
  return outcome.ok ? { code: 'E_INTERNAL', message: '后端未能启动' } : outcome.error;
}

/** 任务终态 → 推给渲染进程的最后一个事件（result 或 error）。 */
function terminalEvent(outcome: RunOutcome, kind: JobKind): ResultEvent | ErrorEvent {
  if (outcome.ok) {
    if (outcome.result.command === kind) return outcome.result;
    return {
      type: 'error',
      code: 'E_INTERNAL',
      message: `后端返回了 ${outcome.result.command} 结果，期望 ${kind}`,
    };
  }
  const { code, message } = outcome.error;
  return { type: 'error', code: isErrorCode(code) ? code : 'E_INTERNAL', message };
}
