import { contextBridge, ipcRenderer } from 'electron';
import type { DataBackupApi, JobId } from '../shared/api';
import type { BackendEvent } from '../shared/protocol';
import {
  channels,
  jobEventChannel,
  windowChannels,
  type JobEventMessage,
} from '../main/ipc/channels';

/**
 * 渲染进程可见的 window.databackup（interaction.md §2），只经 contextBridge 暴露，
 * 不暴露任何 Node 能力。通道见 interaction.md §2.1。
 */

function invoke<T>(channel: string, ...args: unknown[]): Promise<T> {
  return ipcRenderer.invoke(channel, ...args) as Promise<T>;
}

/** 订阅前到达的任务事件最多缓存多少个任务、每个任务多少条。 */
const MAX_PENDING_JOBS = 4;
const MAX_PENDING_EVENTS = 256;

type JobListener = (e: BackendEvent) => void;
const jobListeners = new Map<JobId, Set<JobListener>>();
/**
 * start() 的返回值与 jobs:event 推送走同一条有序通道，但进程启动后的首批事件
 * 可能先于 jobId 到达渲染进程，所以没有订阅者的事件先缓存，onEvent 时补发。
 */
const pendingJobEvents = new Map<JobId, BackendEvent[]>();

ipcRenderer.on(jobEventChannel, (_e, msg: JobEventMessage) => {
  const listeners = jobListeners.get(msg.jobId);
  if (listeners && listeners.size > 0) {
    for (const cb of listeners) cb(msg.event);
    return;
  }
  let queue = pendingJobEvents.get(msg.jobId);
  if (!queue) {
    if (pendingJobEvents.size >= MAX_PENDING_JOBS) {
      const oldest = pendingJobEvents.keys().next().value;
      if (oldest !== undefined) pendingJobEvents.delete(oldest);
    }
    queue = [];
    pendingJobEvents.set(msg.jobId, queue);
  }
  queue.push(msg.event);
  if (queue.length > MAX_PENDING_EVENTS) queue.shift();
});

function onJobEvent(id: JobId, cb: JobListener): () => void {
  let set = jobListeners.get(id);
  if (!set) {
    set = new Set();
    jobListeners.set(id, set);
  }
  set.add(cb);
  const queued = pendingJobEvents.get(id);
  if (queued) {
    pendingJobEvents.delete(id);
    for (const e of queued) cb(e);
  }
  return () => {
    set.delete(cb);
    if (set.size === 0) jobListeners.delete(id);
  };
}

const api = {
  app: {
    versions: () => invoke(channels.appVersions),
  },
  window: {
    minimize: (): void => {
      ipcRenderer.send(windowChannels.minimize);
    },
    toggleMaximize: (): void => {
      ipcRenderer.send(windowChannels.toggleMaximize);
    },
    close: (): void => {
      ipcRenderer.send(windowChannels.close);
    },
    onMaximizedChange: (cb: (maximized: boolean) => void): (() => void) => {
      const listener = (_e: Electron.IpcRendererEvent, maximized: boolean): void => cb(maximized);
      ipcRenderer.on(windowChannels.maximizedChanged, listener);
      return () => {
        ipcRenderer.removeListener(windowChannels.maximizedChanged, listener);
      };
    },
  },
  dialog: {
    pickDirectory: (opts) => invoke(channels.pickDirectory, opts),
  },
  backup: {
    dryRun: (req) => invoke(channels.backupDryRun, req),
    start: (req, password) => invoke(channels.backupStart, req, password),
  },
  restore: {
    start: (req, password) => invoke(channels.restoreStart, req, password),
  },
  jobs: {
    cancel: (id) => invoke(channels.jobsCancel, id),
    onEvent: onJobEvent,
  },
  repo: {
    list: () => invoke(channels.repoList),
    info: (name) => invoke(channels.repoInfo, name),
    remove: (name) => invoke(channels.repoRemove, name),
  },
  config: {
    get: () => invoke(channels.configGet),
    set: (patch) => invoke(channels.configSet, patch),
  },
} satisfies DataBackupApi;

contextBridge.exposeInMainWorld('databackup', api);
