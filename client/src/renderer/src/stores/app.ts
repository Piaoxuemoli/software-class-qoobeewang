import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type PageId = 'backup' | 'manage' | 'settings';

export type BackendStatus = 'connecting' | 'ready' | 'unavailable';

interface AppState {
  /** 当前页面（侧栏导航）。 */
  page: PageId;
  backendStatus: BackendStatus;
  backendVersion: string | null;
  /** 后端不可用时的诊断信息（路径等）。 */
  backendError: string | null;
  /** 应用内"减少动效"设置（持久化），与 prefers-reduced-motion 叠加生效。 */
  reduceMotion: boolean;
  setPage: (page: PageId) => void;
  setBackendReady: (version: string) => void;
  setBackendUnavailable: (message: string) => void;
  setReduceMotion: (v: boolean) => void;
}

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      page: 'backup',
      backendStatus: 'connecting',
      backendVersion: null,
      backendError: null,
      reduceMotion: false,
      setPage: (page) => set({ page }),
      setBackendReady: (version) =>
        set({ backendStatus: 'ready', backendVersion: version, backendError: null }),
      setBackendUnavailable: (message) =>
        set({ backendStatus: 'unavailable', backendError: message, backendVersion: null }),
      setReduceMotion: (reduceMotion) => set({ reduceMotion }),
    }),
    {
      name: 'databackup-app',
      // 只持久化用户设置，页面与会话状态每次启动重置。
      partialize: (s) => ({ reduceMotion: s.reduceMotion }),
    },
  ),
);
