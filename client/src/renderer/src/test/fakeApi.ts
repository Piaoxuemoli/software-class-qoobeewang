import { vi } from 'vitest';
import type { BackupInfo, DataBackupApi, JobId } from '@/shared/api';
import type { BackendEvent } from '@/shared/protocol';

/**
 * 渲染层测试注入的 window.databackup 假实现。
 * 约定与 main/backend/mock.ts 相同（固定版本、固定仓库），
 * 但由测试完全控制：start 后由测试手动 _emit 事件。
 */
export interface FakeApi extends DataBackupApi {
  /** 向某任务的订阅者推送一条后端事件。 */
  _emit: (id: JobId, e: BackendEvent) => void;
}

export const fakeBackups: BackupInfo[] = [
  {
    name: 'daily-20260220',
    path: '/repo/daily-20260220.dbk',
    outputBytes: 2048,
    createdAt: '2026-02-20T09:30:00Z',
    source: '/home/user/notes',
    compressed: false,
    encrypted: false,
    files: 12,
    dirs: 3,
    bytes: 4096,
    formatVersion: 1,
    corrupt: false,
  },
  {
    name: 'secure-20260221',
    path: '/repo/secure-20260221.dbk',
    outputBytes: 1024,
    createdAt: '2026-02-21T10:00:00Z',
    source: '/home/user/docs',
    compressed: true,
    encrypted: true,
    files: 5,
    dirs: 1,
    bytes: 2048,
    formatVersion: 1,
    corrupt: false,
  },
];

export function installFakeApi(overrides: Partial<DataBackupApi> = {}): FakeApi {
  const listeners = new Map<JobId, Set<(e: BackendEvent) => void>>();
  const api: FakeApi = {
    app: { versions: vi.fn(async () => ({ client: '0.1.0', backend: '0.1.0' })) },
    window: {
      minimize: vi.fn(),
      toggleMaximize: vi.fn(),
      close: vi.fn(),
      onMaximizedChange: vi.fn(() => () => {}),
    },
    dialog: { pickDirectory: vi.fn(async () => null) },
    backup: {
      dryRun: vi.fn(async () => ({ files: 2, dirs: 1, bytes: 1024, skipped: 0 })),
      start: vi.fn(async () => 'job-1'),
    },
    restore: { start: vi.fn(async () => 'job-2') },
    jobs: {
      cancel: vi.fn(async () => {}),
      onEvent: vi.fn((id: JobId, cb: (e: BackendEvent) => void) => {
        let set = listeners.get(id);
        if (!set) {
          set = new Set();
          listeners.set(id, set);
        }
        set.add(cb);
        return () => set.delete(cb);
      }),
    },
    repo: {
      list: vi.fn(async () => [...fakeBackups]),
      info: vi.fn(async (name: string) => {
        const b = fakeBackups.find((x) => x.name === name);
        if (!b) throw new Error('[E_NOT_FOUND] 备份不存在');
        return b;
      }),
      remove: vi.fn(async () => {}),
    },
    config: {
      get: vi.fn(async () => ({ repo: '/repo' })),
      set: vi.fn(async () => ({ repo: '/repo' })),
    },
    _emit: (id, e) => {
      for (const cb of listeners.get(id) ?? []) cb(e);
    },
    ...overrides,
  };
  window.databackup = api;
  return api;
}
