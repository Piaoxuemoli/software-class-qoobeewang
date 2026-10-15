import path from 'node:path';
import { _electron as electron, expect, test } from '@playwright/test';

const clientDir = path.resolve(__dirname, '..');
const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');

// Chromium refuses its sandbox when running as root (CI container).
const extraArgs =
  typeof process.getuid === 'function' && process.getuid() === 0 ? ['--no-sandbox'] : [];

test('window opens and shows the backend version', async () => {
  const app = await electron.launch({
    args: [...extraArgs, '.'],
    cwd: clientDir,
    env: { ...process.env, DATABACKUP_BIN: backendBin },
  });
  const win = await app.firstWindow();
  await expect(win).toHaveTitle('Data Backup');
  await expect(win.getByTestId('statusbar')).toContainText('后端版本: 0.1.0', {
    timeout: 15_000,
  });
  await app.close();
});
