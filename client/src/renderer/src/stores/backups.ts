import { create } from 'zustand';
import { parseApiError, type ApiError, type BackupInfo } from '@/shared/api';

/**
 * 备份仓库的列表缓存（interaction.md §1 backupsStore）：
 * 启动时预取一次，备份完成 / 删除 / 改仓库路径后刷新。
 */
interface BackupsStore {
  /** 仓库目录（config.repo）；空串表示尚未读到。 */
  repoPath: string;
  /** null 表示尚未加载。 */
  backups: BackupInfo[] | null;
  loading: boolean;
  error: ApiError | null;
  /** 详情面板当前选中的备份名。 */
  selected: string | null;
  /** 先取 config.get 拿仓库路径，再 repo.list；任一失败都记入 error。 */
  refresh: () => Promise<void>;
  /** 删除备份并刷新列表；失败抛错由调用方提示。 */
  remove: (name: string) => Promise<void>;
  /** 修改仓库路径（config.set）并刷新列表；失败抛错。 */
  setRepo: (dir: string) => Promise<void>;
  select: (name: string | null) => void;
}

export const initialBackupsState = {
  repoPath: '',
  backups: null,
  loading: false,
  error: null,
  selected: null,
} satisfies Partial<BackupsStore>;

export const useBackupsStore = create<BackupsStore>()((set, get) => ({
  ...initialBackupsState,

  refresh: async () => {
    set({ loading: true });
    try {
      const cfg = await window.databackup.config.get();
      const backups = await window.databackup.repo.list();
      // 选中项可能刚被删除
      const selected = get().selected;
      set({
        repoPath: cfg.repo,
        backups,
        loading: false,
        error: null,
        selected: backups.some((b) => b.name === selected) ? selected : null,
      });
    } catch (err) {
      set({ loading: false, error: parseApiError(err) });
    }
  },

  remove: async (name) => {
    await window.databackup.repo.remove(name);
    await get().refresh();
  },

  setRepo: async (dir) => {
    await window.databackup.config.set({ repo: dir });
    await get().refresh();
  },

  select: (name) => set({ selected: name }),
}));
