/// <reference types="vite/client" />

import type { DataBackupApi } from '../../shared/api';

declare global {
  interface Window {
    /**
     * preload 暴露的 API。C0 提供 window.* 命名空间与既有 getBackendVersion；
     * app/dialog/backup/restore/jobs/repo/config 由 C1 补齐后，
     * 本声明扩展为完整 DataBackupApi。
     */
    databackup: Pick<DataBackupApi, 'window'> & {
      getBackendVersion(): Promise<string>;
    };
  }
}

export {};
