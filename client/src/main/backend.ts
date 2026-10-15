import { spawn } from 'node:child_process';
import path from 'node:path';
import { app } from 'electron';
import { createJsonlParser, type JsonlEvent } from './jsonl';

/**
 * Resolve the databackup backend binary:
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

export class BackendError extends Error {}

/** Run the backend once and return all emitted JSONL events. */
export function runBackend(args: string[]): Promise<JsonlEvent[]> {
  const bin = resolveBackendPath();
  return new Promise((resolve, reject) => {
    const child = spawn(bin, args);
    const events: JsonlEvent[] = [];
    const parser = createJsonlParser((e) => events.push(e));
    let stderr = '';

    child.on('error', (err: NodeJS.ErrnoException) => {
      reject(
        new BackendError(
          err.code === 'ENOENT'
            ? `backend binary not found: ${bin}`
            : `failed to start backend: ${err.message}`,
        ),
      );
    });
    child.stdout.on('data', (chunk: Buffer) => parser.feed(chunk.toString('utf8')));
    child.stderr.on('data', (chunk: Buffer) => {
      stderr += chunk.toString('utf8');
    });
    child.on('close', (code) => {
      parser.flush();
      if (code === 0) {
        resolve(events);
      } else {
        reject(new BackendError(`backend exited with code ${code}: ${stderr.trim()}`));
      }
    });
  });
}

/** Query `databackup --version --json` and return the version string. */
export async function getBackendVersion(): Promise<string> {
  const events = await runBackend(['--version', '--json']);
  for (const e of events) {
    if (e.type === 'result' && e.command === 'version' && typeof e.version === 'string') {
      return e.version;
    }
  }
  throw new BackendError('backend did not report a version');
}
