import path from 'node:path';
import { app } from 'electron';

/** Windows 后端是 databackup.exe（C4b 便携包），其余平台为 databackup。 */
const BIN_NAME = process.platform === 'win32' ? 'databackup.exe' : 'databackup';

/**
 * Resolve the databackup backend binary（docs/architecture/overview.md）:
 *   1. DATABACKUP_BIN env override (development / tests)
 *   2. packaged app: <resources>/bin/databackup[.exe]
 *   3. dev checkout: ../backend/bin/databackup[.exe] relative to client/
 */
export function resolveBackendPath(): string {
  if (process.env.DATABACKUP_BIN) {
    return process.env.DATABACKUP_BIN;
  }
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'bin', BIN_NAME);
  }
  return path.resolve(app.getAppPath(), '..', 'backend', 'bin', BIN_NAME);
}
