import path from 'node:path';
import { _electron as electron, expect, test, type ElectronApplication } from '@playwright/test';

const clientDir = path.resolve(__dirname, '..');
const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');

// Chromium refuses its sandbox when running as root (CI container).
const extraArgs =
  typeof process.getuid === 'function' && process.getuid() === 0 ? ['--no-sandbox'] : [];

async function launchApp(): Promise<ElectronApplication> {
  return electron.launch({
    args: [...extraArgs, '.'],
    cwd: clientDir,
    env: { ...process.env, DATABACKUP_BIN: backendBin },
  });
}

test('window opens and shows the backend version', async () => {
  const app = await launchApp();
  const win = await app.firstWindow();
  await expect(win).toHaveTitle('Data Backup');
  await expect(win.getByTestId('statusbar')).toContainText('后端版本: 0.1.0', {
    timeout: 15_000,
  });
  await app.close();
});

// 截图基线（ui-design.md §7）：三个页面的空闲态各一张。只在 Docker 开发镜像中生成/更新。
test('screenshots: idle state of each page', async () => {
  const app = await launchApp();
  const win = await app.firstWindow();
  await expect(win.getByTestId('statusbar')).toContainText('后端版本', { timeout: 15_000 });
  await expect(win).toHaveScreenshot('page-backup.png');

  await win.getByRole('button', { name: '备份管理' }).click();
  await expect(win).toHaveScreenshot('page-manage.png');

  await win.getByRole('button', { name: '设置' }).click();
  await expect(win).toHaveScreenshot('page-settings.png');
  await app.close();
});
