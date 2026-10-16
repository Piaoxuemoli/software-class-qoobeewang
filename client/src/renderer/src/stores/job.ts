import { create } from 'zustand';
import {
  parseApiError,
  type ApiError,
  type BackupFilters,
  type BackupInfo,
  type BackupRequest,
  type DryRunSummary,
  type JobId,
} from '@/shared/api';
import type {
  BackendEvent,
  BackupResult,
  LogEvent,
  Phase,
  ProgressEvent,
  RestoreResult,
} from '@/shared/protocol';

/**
 * 任务状态机（docs/client/interaction.md §3）。
 * idle/editing/ready 是"尚未开始"的三个态：无源目录为 idle，
 * 表单未通过校验为 editing，可提交为 ready。
 * 还原在 running 之前多一个 awaitingPassword（加密备份等口令输入）。
 */
export type JobStatus =
  | 'idle'
  | 'editing'
  | 'ready'
  | 'awaitingPassword'
  | 'running'
  | 'cancelling'
  | 'cancelled'
  | 'succeeded'
  | 'failed';

export type JobKind = 'backup' | 'restore';

/** 备份编排卡的表单；filters 与 BackupRequest.filters 同构，字段原样传给后端。 */
export interface BackupForm {
  source: string;
  name: string;
  filters: BackupFilters;
  compress: boolean;
  encrypt: boolean;
  password: string;
  confirm: string;
}

/** 还原任务的上下文（目标目录与一次性口令；备份名与是否加密）。 */
export interface RestoreCtx {
  name: string;
  encrypted: boolean;
  target: string;
  password: string;
}

export interface Preview {
  /** 正在请求 dry-run（或已超过 2s 仍在等待，界面上显示"计算中…"）。 */
  pending: boolean;
  summary: DryRunSummary | null;
  error: ApiError | null;
}

/** 日志最多保留的条数，防止长任务撑爆内存。 */
const MAX_LOGS = 200;

const initialForm: BackupForm = {
  source: '',
  name: '',
  filters: {},
  compress: false,
  encrypt: false,
  password: '',
  confirm: '',
};

const initialJob = {
  status: 'idle' as JobStatus,
  kind: null as JobKind | null,
  jobId: null as JobId | null,
  phase: null as Phase | null,
  progress: null as ProgressEvent | null,
  logs: [] as LogEvent[],
  result: null as BackupResult | RestoreResult | null,
  error: null as ApiError | null,
  authFailures: 0,
  startedAt: null as number | null,
  form: initialForm,
  restore: null as RestoreCtx | null,
  preview: { pending: false, summary: null, error: null } as Preview,
};

/** 测试用：把 store 重置为初始状态。 */
export const initialJobState = initialJob;

type JobFields = typeof initialJob;

interface JobStore extends JobFields {
  /** 表单变化后的重新判定：idle（无源）/ editing（不可提交）/ ready（可提交）。 */
  setSource: (v: string) => void;
  setName: (v: string) => void;
  setFilters: (patch: Partial<BackupFilters>) => void;
  setCompress: (v: boolean) => void;
  setEncrypt: (v: boolean) => void;
  setPassword: (v: string) => void;
  setConfirm: (v: string) => void;
  /** 防抖后由组件调用；用当前表单发 dry-run 预览。 */
  refreshPreview: () => Promise<void>;
  /** ready → running：启动备份并订阅任务事件。 */
  start: () => Promise<void>;
  cancel: () => Promise<void>;
  /** failed → ready（保留表单与错误提示由界面决定是否展示）。 */
  retry: () => void;
  /** succeeded → idle：清空任务与表单，开始下一次备份。 */
  again: () => void;
  /** cancelled → ready（保留表单）。 */
  backToReady: () => void;
  /** 由订阅回调喂入的后端事件（progress/log/result/error）。 */
  handleEvent: (e: BackendEvent) => void;

  /** 打开某备份的还原流程（备份管理页 Sheet 内）。 */
  beginRestore: (b: BackupInfo) => void;
  setRestoreTarget: (v: string) => void;
  setRestorePassword: (v: string) => void;
  /** ready → awaitingPassword（加密）或直接启动（未加密）。 */
  requestRestore: () => Promise<void>;
  /** awaitingPassword → running：提交口令启动还原。 */
  submitRestorePassword: () => Promise<void>;
  /** 放弃当前还原上下文（关闭 Sheet 时）。 */
  endRestore: () => void;
}

/** 表单是否可提交：源目录非空；加密时要求口令非空且两次输入一致。 */
function formReady(f: BackupForm): boolean {
  if (f.source.trim() === '') return false;
  return !f.encrypt || (f.password !== '' && f.password === f.confirm);
}

/** 表单修改后重算 idle/editing/ready；任务态不受影响。 */
function statusOf(f: BackupForm): JobStatus {
  if (f.source.trim() === '') return 'idle';
  return formReady(f) ? 'ready' : 'editing';
}

/** 可被表单重算覆盖的状态（任务进行中与等待口令的态除外）。 */
const PRE_RUN_STATES: readonly JobStatus[] = [
  'idle',
  'editing',
  'ready',
  'succeeded',
  'failed',
  'cancelled',
];

function toRequest(f: BackupForm): BackupRequest {
  return {
    source: f.source.trim(),
    name: f.name.trim() === '' ? undefined : f.name.trim(),
    filters: f.filters,
    compress: f.compress,
    encrypt: f.encrypt,
  };
}

export const useJobStore = create<JobStore>()((set, get) => {
  /** 修改表单并把状态机拉回合适的 pre-run 态。 */
  const patchForm = (patch: Partial<BackupForm>): void => {
    set((s) => {
      const form = { ...s.form, ...patch };
      return {
        form,
        status: PRE_RUN_STATES.includes(s.status) ? statusOf(form) : s.status,
        preview: { pending: false, summary: null, error: null },
      };
    });
  };

  let unsubscribe: (() => void) | null = null;
  const stopWatching = (): void => {
    unsubscribe?.();
    unsubscribe = null;
  };

  const beginRunning = (kind: JobKind): void => {
    stopWatching();
    // 注意：不重置 authFailures——E_AUTH 后重新提交口令仍走这里，
    // "连续 3 次失败"必须跨多次提交累计（开始新一轮还原时才由 initialJob 清零）。
    set({
      status: 'running',
      kind,
      jobId: null,
      phase: null,
      progress: null,
      logs: [],
      result: null,
      error: null,
      startedAt: Date.now(),
      preview: { pending: false, summary: null, error: null },
    });
  };

  const watchJob = (id: JobId): void => {
    unsubscribe = window.databackup.jobs.onEvent(id, (e) => get().handleEvent(e));
    set({ jobId: id });
  };

  return {
    ...initialJob,

    setSource: (v) => patchForm({ source: v }),
    setName: (v) => patchForm({ name: v }),
    setFilters: (patch) => patchForm({ filters: { ...get().form.filters, ...patch } }),
    setCompress: (v) => patchForm({ compress: v }),
    setEncrypt: (v) => patchForm({ encrypt: v }),
    setPassword: (v) => patchForm({ password: v }),
    setConfirm: (v) => patchForm({ confirm: v }),

    refreshPreview: async () => {
      const { form, status } = get();
      if (status !== 'editing' && status !== 'ready') return;
      set({ preview: { pending: true, summary: null, error: null } });
      try {
        const summary = await window.databackup.backup.dryRun(toRequest(form));
        // 只接受仍匹配当前表单的最新一次结果（组件层防抖，但请求仍可乱序回来）。
        if (get().preview.pending) {
          set({ preview: { pending: false, summary, error: null } });
        }
      } catch (err) {
        if (get().preview.pending) {
          const e = parseApiError(err);
          if (e.code !== 'E_CANCELLED') {
            set({ preview: { pending: false, summary: null, error: e } });
          }
        }
      }
    },

    start: async () => {
      const s = get();
      if (s.status !== 'ready') return;
      const req = toRequest(s.form);
      const password = s.form.password;
      beginRunning('backup');
      // 口令只交给本次调用，立即从 store 中清除（interaction.md §7）。
      set((st) => ({ form: { ...st.form, password: '', confirm: '' } }));
      try {
        const id = await window.databackup.backup.start(req, req.encrypt ? password : undefined);
        watchJob(id);
      } catch (err) {
        set({ status: 'failed', error: parseApiError(err) });
      }
    },

    cancel: async () => {
      const { status, jobId } = get();
      if (status !== 'running' || jobId === null) return;
      set({ status: 'cancelling' });
      try {
        await window.databackup.jobs.cancel(jobId);
      } catch {
        // 取消失败：终态事件仍会到达，错误在 handleEvent 中处理。
      }
    },

    retry: () => {
      if (get().status !== 'failed') return;
      set({ status: 'ready', error: null });
    },

    again: () => {
      if (get().status !== 'succeeded') return;
      stopWatching();
      set({ ...initialJob, form: { ...initialForm, filters: {} } });
    },

    backToReady: () => {
      if (get().status !== 'cancelled') return;
      set({ status: 'ready', error: null });
    },

    handleEvent: (e) => {
      const s = get();
      switch (e.type) {
        case 'progress':
          set({ progress: e, phase: e.phase });
          break;
        case 'log':
          set({ logs: [...s.logs, e].slice(-MAX_LOGS) });
          break;
        case 'result':
          if (e.command !== 'backup' && e.command !== 'restore') break;
          stopWatching();
          set({ status: 'succeeded', result: e });
          break;
        case 'error':
          stopWatching();
          if (e.code === 'E_CANCELLED') {
            set({ status: 'cancelled' });
          } else if (
            s.kind === 'restore' &&
            (e.code === 'E_AUTH' || e.code === 'E_PASSWORD_REQUIRED')
          ) {
            const n = e.code === 'E_AUTH' ? s.authFailures + 1 : s.authFailures;
            // 连续 3 次口令错误回到 ready（02 设计文档 §4、interaction.md §3）。
            set({
              status: n >= 3 ? 'ready' : 'awaitingPassword',
              authFailures: n >= 3 ? 0 : n,
              error: { code: e.code, message: e.message },
            });
          } else {
            set({ status: 'failed', error: { code: e.code, message: e.message } });
          }
          break;
      }
    },

    beginRestore: (b) => {
      stopWatching();
      set({
        ...initialJob,
        form: get().form,
        preview: get().preview,
        kind: 'restore',
        status: 'editing',
        restore: { name: b.name, encrypted: b.encrypted, target: '', password: '' },
      });
    },

    setRestoreTarget: (v) => {
      const r = get().restore;
      if (!r || get().kind !== 'restore') return;
      set({ restore: { ...r, target: v }, status: v.trim() === '' ? 'editing' : 'ready' });
    },

    setRestorePassword: (v) => {
      const r = get().restore;
      if (r) set({ restore: { ...r, password: v } });
    },

    requestRestore: async () => {
      const { status, restore } = get();
      if (status !== 'ready' || !restore) return;
      if (restore.encrypted) {
        set({ status: 'awaitingPassword', error: null });
        return;
      }
      await get().submitRestorePassword();
    },

    submitRestorePassword: async () => {
      const s = get();
      const r = s.restore;
      if (!r || (s.status !== 'awaitingPassword' && s.status !== 'ready')) return;
      const password = r.password || undefined;
      beginRunning('restore');
      // 口令立即从 store 清除。
      set({ restore: { ...r, password: '' } });
      try {
        const id = await window.databackup.restore.start(
          { backup: r.name, target: r.target },
          password,
        );
        watchJob(id);
      } catch (err) {
        set({ status: 'failed', error: parseApiError(err) });
      }
    },

    endRestore: () => {
      stopWatching();
      set({ ...initialJob, form: get().form, preview: get().preview });
    },
  };
});
