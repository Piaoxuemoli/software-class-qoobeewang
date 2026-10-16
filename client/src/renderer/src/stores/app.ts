import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type PageId = 'backup' | 'manage' | 'settings';

export type BackendStatus = 'connecting' | 'ready' | 'unavailable';

interface AppState {
  /** 当前页面（侧栏导航）。 */
  page: PageId;
  backendStatus: BackendStatus;
  backendVersion: string | null;
  clientVersion: string | null;
  /** 后端不可用时的诊断信息（路径等）。 */
  backendError: string | null;
  /** 应用内"减少动效"设置（持久化），与 prefers-reduced-motion 叠加生效。 */
  reduceMotion: boolean;
  /** 侧栏折叠为 64px 图标栏（Ctrl+B，持久化）。 */
  sidebarCollapsed: boolean;
  setPage: (page: PageId) => void;
  /** app.versions() 成功：同时记录客户端与后端版本。 */
  setVersions: (v: { client: string; backend: string }) => void;
  setBackendUnavailable: (message: string) => void;
  setReduceMotion: (v: boolean) => void;
  toggleSidebar: () => void;
}

/** 非函数的初始状态；测试用它在用例之间重置 store。 */
export const initialAppState = {
  page: 'backup',
  backendStatus: 'connecting',
  backendVersion: null,
  clientVersion: null,
  backendError: null,
  reduceMotion: false,
  sidebarCollapsed: false,
} satisfies Partial<AppState>;

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      ...initialAppState,
      setPage: (page) => set({ page }),
      setVersions: (v) =>
        set({
          backendStatus: 'ready',
          backendVersion: v.backend,
          clientVersion: v.client,
          backendError: null,
        }),
      setBackendUnavailable: (message) =>
        set({ backendStatus: 'unavailable', backendError: message, backendVersion: null }),
      setReduceMotion: (reduceMotion) => set({ reduceMotion }),
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
    }),
    {
      name: 'databackup-app',
      // 只持久化用户设置，页面与会话状态每次启动重置。
      partialize: (s) => ({ reduceMotion: s.reduceMotion, sidebarCollapsed: s.sidebarCollapsed }),
    },
  ),
);
