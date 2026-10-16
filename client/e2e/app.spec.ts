import path from 'node:path';
import { _electron as electron, expect, test, type ElectronApplication } from '@playwright/test';

const clientDir = path.resolve(__dirname, '..');
const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');

// Chromium refuses its sandbox when running as root (CI container).
const extraArgs =
  typeof process.getuid === 'function' && process.getuid() === 0 ? ['--no-sandbox'] : [];

/** 用 mock 后端启动应用（DATABACKUP_MOCK=1，见 interaction.md §2.1）。 */
async function launchApp(): Promise<ElectronApplication> {
  return electron.launch({
    args: [...extraArgs, '.'],
    cwd: clientDir,
    env: { ...process.env, DATABACKUP_BIN: backendBin, DATABACKUP_MOCK: '1' },
  });
}

async function firstWindow(app: ElectronApplication) {
  const win = await app.firstWindow();
  // 等后端版本出现（app.versions() 成功）再操作，保证各用例时序稳定。
  await expect(win.getByTestId('statusbar')).toContainText('后端版本: 0.0.0-mock', {
    timeout: 15_000,
  });
  return win;
}

test('TC-E2E-01 window opens and shows the mock backend version', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);
  await expect(win).toHaveTitle('Data Backup');
  await app.close();
});

test('TC-E2E-02 backup with filters, compression and encryption completes', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);

  // 源目录
  await win.getByLabel('源目录').fill('/data');

  // 筛选：排除目录正则
  await win.getByRole('button', { name: '排除目录' }).click();
  await win.getByLabel(/排除目录名/).fill('^node_modules$');
  await win.keyboard.press('Escape');

  // 选项：压缩 + 加密（口令 mock）
  await win.getByRole('switch', { name: '压缩' }).click();
  await win.getByRole('switch', { name: '加密' }).click();
  await win.getByLabel('口令', { exact: true }).fill('mock');
  await win.getByLabel('确认口令').fill('mock');

  await win.getByRole('button', { name: '开始备份' }).click();
  await expect(win.getByRole('button', { name: '取消' })).toBeVisible();
  // 等总量出现（pack 阶段）：此时标题稳定为"备份中 · 处理中"。
  await expect(win.getByTestId('job-percent')).toBeVisible();

  // 进度卡截图：遮住一切随时间变化的元素（计数、已用时间、当前路径、
  // 进度条/百分比、旋转的状态图标、侧栏迷你进度、可能已出现的日志区）。
  await expect(win).toHaveScreenshot('backup-progress.png', {
    mask: [
      win.getByTestId('status-running'),
      win.getByTestId('stat-文件'),
      win.getByTestId('stat-大小'),
      win.getByTestId('elapsed'),
      win.getByTestId('current-path'),
      win.getByTestId('sidebar-progress'),
      win.getByTestId('job-percent'),
      win.locator('[data-slot="progress"]'),
      win.locator('details'),
    ],
  });

  await expect(win.getByTestId('job-succeeded')).toContainText('备份完成', { timeout: 20_000 });
  await expect(win).toHaveScreenshot('backup-done.png');
  await app.close();
});

test('TC-E2E-03 cancelling a running backup returns to cancelled state', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);

  await win.getByLabel('源目录').fill('/data');
  await win.getByRole('button', { name: '开始备份' }).click();
  await win.getByRole('button', { name: '取消' }).click();
  await expect(win.getByTestId('job-cancelled')).toContainText('已取消', { timeout: 10_000 });
  await app.close();
});

test('TC-E2E-04 encrypted restore: wrong password then retry with mock', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);

  await win.getByRole('button', { name: '备份管理' }).click();
  await win.getByTestId('backup-row-docs-20261015-235500').click();
  await expect(win.getByTestId('backup-sheet')).toBeVisible();
  await expect(win).toHaveScreenshot('restore-sheet.png');

  await win.getByRole('button', { name: '还原…' }).click();
  await win.getByLabel(/目标目录/).fill('/restored');
  await win.getByRole('button', { name: '开始还原' }).click();

  const dialog = win.getByTestId('password-dialog');
  await expect(dialog).toBeVisible();
  await expect(win).toHaveScreenshot('password-dialog.png');

  // 错误口令 → E_AUTH → 回到口令输入并提示剩余次数
  await dialog.getByRole('textbox', { name: '备份口令' }).fill('wrong');
  await dialog.getByRole('button', { name: '开始还原' }).click();
  await expect(dialog.getByRole('alert')).toContainText('还可尝试 2 次', { timeout: 10_000 });

  // 正确口令 mock → 还原完成
  await dialog.getByRole('textbox', { name: '备份口令' }).fill('mock');
  await dialog.getByRole('button', { name: '开始还原' }).click();
  await expect(win.getByTestId('job-succeeded')).toContainText('还原完成', { timeout: 20_000 });
  await app.close();
});

test('TC-E2E-05 deleting a corrupt backup removes it from the list', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);

  await win.getByRole('button', { name: '备份管理' }).click();
  await win.getByTestId('backup-row-old').click();
  await expect(win.getByTestId('backup-sheet')).toBeVisible();

  // 长按 1200ms 删除
  const hold = win.getByRole('button', { name: /长按.*删除/ });
  await hold.dispatchEvent('pointerdown');
  await expect(win.getByTestId('backup-row-old')).toHaveCount(0, { timeout: 10_000 });
  await app.close();
});

// 截图基线（ui-design.md §7）：三个页面的空闲态各一张。只在 Docker 开发镜像中生成/更新。
test('TC-E2E-06 screenshots: idle state of each page', async () => {
  const app = await launchApp();
  const win = await firstWindow(app);
  await expect(win).toHaveScreenshot('page-backup.png');

  await win.getByRole('button', { name: '备份管理' }).click();
  await expect(win).toHaveScreenshot('page-manage.png');

  await win.getByRole('button', { name: '设置' }).click();
  await expect(win).toHaveScreenshot('page-settings.png');
  await app.close();
});
