/// <reference types="vite/client" />

import type { DataBackupApi } from '../../shared/api';

declare global {
  interface Window {
    /** preload 经 contextBridge 暴露的完整 API（interaction.md §2.1）。 */
    databackup: DataBackupApi;
  }
}

export {};
