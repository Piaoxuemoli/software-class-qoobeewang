import path from 'node:path';
import {
  _electron as electron,
  expect,
  type ElectronApplication,
  type Page,
} from '@playwright/test';

/** mock 后端（DATABACKUP_MOCK=1，见 docs/client/interaction.md §2.1）的启动与就绪判断。 */

export const clientDir = path.resolve(__dirname, '..');
export const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');

// Chromium refuses its sandbox when running as root (CI container).
const extraArgs =
  typeof process.getuid === 'function' && process.getuid() === 0 ? ['--no-sandbox'] : [];

export async function launchMockApp(): Promise<ElectronApplication> {
  return electron.launch({
    args: [...extraArgs, '.'],
    cwd: clientDir,
    env: { ...process.env, DATABACKUP_BIN: backendBin, DATABACKUP_MOCK: '1' },
  });
}

export async function firstWindow(app: ElectronApplication): Promise<Page> {
  const win = await app.firstWindow();
  // 等后端版本出现（app.versions() 成功）再操作，保证各用例时序稳定。
  await expect(win.getByTestId('statusbar')).toContainText('后端版本: 0.0.0-mock', {
    timeout: 15_000,
  });
  return win;
}
