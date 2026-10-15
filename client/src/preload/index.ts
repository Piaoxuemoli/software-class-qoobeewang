import { contextBridge, ipcRenderer } from 'electron';

const api = {
  getBackendVersion: (): Promise<string> => ipcRenderer.invoke('backend:getVersion'),
};

export type DataBackupApi = typeof api;

contextBridge.exposeInMainWorld('databackup', api);
