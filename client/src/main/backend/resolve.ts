import path from 'node:path';
import { app } from 'electron';

/**
 * Resolve the databackup backend binary（docs/architecture/overview.md）:
 *   1. DATABACKUP_BIN env override (development / tests)
 *   2. packaged app: <resources>/bin/databackup
 *   3. dev checkout: ../backend/bin/databackup relative to client/
 */
export function resolveBackendPath(): string {
  if (process.env.DATABACKUP_BIN) {
    return process.env.DATABACKUP_BIN;
  }
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'bin', 'databackup');
  }
  return path.resolve(app.getAppPath(), '..', 'backend', 'bin', 'databackup');
}
