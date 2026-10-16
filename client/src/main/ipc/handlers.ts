import type { IpcMainInvokeEvent, WebContents } from 'electron';
import type { z } from 'zod';
import type { BackupInfo, Config, DryRunSummary, JobId, Versions } from '../../shared/api';
import type { BackendEvent, InfoResult } from '../../shared/protocol';
import {
  backupArgs,
  configGetArgs,
  configSetArgs,
  deleteArgs,
  dryRunArgs,
  infoArgs,
  listArgs,
  restoreArgs,
  versionArgs,
} from '../backend/args';
import type { JobManager } from '../backend/jobs';
import { ApiFailure, apiFailure } from '../errors';
import { channels, jobEventChannel, type JobEventMessage } from './channels';
import { argsSchemas } from './schemas';

/** ipcMain 的可注入子集。 */
export interface IpcHandleTarget {
  handle(
    channel: string,
    listener: (event: IpcMainInvokeEvent, ...args: unknown[]) => unknown,
  ): void;
}

export interface IpcDeps {
  jobs: JobManager;
  clientVersion: string;
  pickDirectory(sender: WebContents, title: string | undefined): Promise<string | null>;
}

type Handler<S extends z.ZodTuple> = (event: IpcMainInvokeEvent, args: z.infer<S>) => unknown;

function toApiFailure(err: unknown): ApiFailure {
  if (err instanceof ApiFailure) return err;
  return apiFailure('E_INTERNAL', err instanceof Error ? err.message : String(err));
}

/**
 * 注册一个 invoke 通道：入参整体按元组校验，失败 reject `[E_USAGE] ...`；
 * 处理器抛出的错误统一编码为 `[<code>] <说明>`（formatApiError）。
 * 刻意不写成 async：校验后的入参（可能含口令）不跨 await 保留。
 */
function register<S extends z.ZodTuple>(
  ipc: IpcHandleTarget,
  channel: string,
  schema: S,
  handler: Handler<S>,
): void {
  ipc.handle(channel, (event, ...raw) => {
    const parsed = schema.safeParse(raw);
    if (!parsed.success) {
      const where = parsed.error.issues
        .map((i) => `${i.path.join('.') || '(参数)'}: ${i.message}`)
        .join('；');
      return Promise.reject(apiFailure('E_USAGE', `IPC 参数非法（${channel}）：${where}`));
    }
    try {
      return Promise.resolve(handler(event, parsed.data)).catch((err: unknown) => {
        throw toApiFailure(err);
      });
    } catch (err) {
      return Promise.reject(toApiFailure(err));
    }
  });
}

/** 把任务事件推给发起任务的渲染进程（interaction.md §2.1 jobs:event）。 */
function eventSink(sender: WebContents): (jobId: JobId, event: BackendEvent) => void {
  return (jobId, event) => {
    if (sender.isDestroyed()) return;
    const msg: JobEventMessage = { jobId, event };
    sender.send(jobEventChannel, msg);
  };
}

function toBackupInfo(r: InfoResult): BackupInfo {
  return {
    name: r.name,
    path: r.path,
    outputBytes: r.outputBytes,
    createdAt: r.createdAt,
    source: r.source,
    compressed: r.compressed,
    encrypted: r.encrypted,
    files: r.files,
    dirs: r.dirs,
    bytes: r.bytes,
    formatVersion: r.formatVersion,
    corrupt: r.corrupt,
  };
}

/** 注册 window.databackup 除 window.* 以外的全部 invoke 通道（window.* 见 window.ts）。 */
export function registerIpcHandlers(ipc: IpcHandleTarget, deps: IpcDeps): void {
  const { jobs } = deps;
  const backendVersion = async (): Promise<string> =>
    (await jobs.runOnce(versionArgs(), 'version')).version;
  const readConfig = async (args: string[]): Promise<Config> => ({
    repo: (await jobs.runOnce(args, 'config')).repo,
  });

  register(ipc, channels.appVersions, argsSchemas.none, async (): Promise<Versions> => ({
    client: deps.clientVersion,
    backend: await backendVersion(),
  }));
  register(ipc, channels.backendVersion, argsSchemas.none, backendVersion);
  register(ipc, channels.pickDirectory, argsSchemas.pickDirectory, (e, [opts]) =>
    deps.pickDirectory(e.sender, opts?.title),
  );
  register(ipc, channels.backupDryRun, argsSchemas.dryRun, (_e, [req]): Promise<DryRunSummary> =>
    jobs.runDryRun(dryRunArgs(req)),
  );
  register(ipc, channels.backupStart, argsSchemas.backupStart, (e, [req, password]) =>
    jobs.start('backup', backupArgs(req), {
      // 不加密时 CLI 不读 stdin（也不接受 --password-stdin），口令直接丢弃。
      password: req.encrypt ? password : undefined,
      onEvent: eventSink(e.sender),
    }),
  );
  register(ipc, channels.restoreStart, argsSchemas.restoreStart, (e, [req, password]) =>
    jobs.start('restore', restoreArgs(req, password !== undefined), {
      password,
      onEvent: eventSink(e.sender),
    }),
  );
  register(ipc, channels.jobsCancel, argsSchemas.jobId, (_e, [id]) => jobs.cancel(id));
  register(
    ipc,
    channels.repoList,
    argsSchemas.none,
    async (): Promise<BackupInfo[]> => (await jobs.runOnce(listArgs(), 'list')).backups,
  );
  register(
    ipc,
    channels.repoInfo,
    argsSchemas.backupName,
    async (_e, [name]): Promise<BackupInfo> =>
      toBackupInfo(await jobs.runOnce(infoArgs(name), 'info')),
  );
  register(ipc, channels.repoRemove, argsSchemas.backupName, async (_e, [name]): Promise<void> => {
    await jobs.runOnce(deleteArgs(name), 'delete');
  });
  register(ipc, channels.configGet, argsSchemas.none, () => readConfig(configGetArgs()));
  register(ipc, channels.configSet, argsSchemas.configPatch, (_e, [patch]) =>
    readConfig(patch.repo === undefined ? configGetArgs() : configSetArgs('repo', patch.repo)),
  );
}
