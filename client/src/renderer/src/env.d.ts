/// <reference types="vite/client" />

interface DataBackupApi {
  getBackendVersion(): Promise<string>;
}

interface Window {
  databackup: DataBackupApi;
}
