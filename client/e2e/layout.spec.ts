import { expect, test } from '@playwright/test';

import {
  expectBorder,
  expectDialogCentered,
  expectFixedOverlay,
  expectNoHorizontalOverflow,
  expectPill,
  expectRadius,
  expectSheetOnRight,
  expectSidebarWidth,
} from './layout';
import { firstWindow, launchMockApp } from './mock';

/**
 * 结构契约 + 浮层元素截图（docs/client/visual-regression.md §3）。
 *
 * 这些用例不比对整页像素：先断言"布局与样式还在"，再对浮层本身做小范围
 * 截图。样式整体失效时（如 Tailwind 工具类没生成），几何断言会先失败，
 * 不必等到人去比对整页截图。
 */

test('TC-E2E-10 三页结构契约与设计令牌', async () => {
  const app = await launchMockApp();
  const win = await firstWindow(app);

  // 备份页：卡片/输入框/主按钮的令牌与几何。
  await expectSidebarWidth(win.getByTestId('sidebar'));
  await expectNoHorizontalOverflow(win);
  await expectRadius(win.locator('[data-slot="card"]').first(), 12);
  await expectBorder(win.locator('[data-slot="card"]').first());
  await expectRadius(win.getByLabel('源目录'), 8);
  await expectPill(win.getByRole('button', { name: '开始备份' }));

  // 管理页与设置页：不出现水平溢出（Sheet/表格把布局撑破时这里会失败）。
  await win.getByRole('button', { name: '备份管理' }).click();
  await expectNoHorizontalOverflow(win);
  await win.getByRole('button', { name: '设置' }).click();
  await expectNoHorizontalOverflow(win);

  await app.close();
});

test('TC-E2E-11 详情 Sheet 与口令对话框的几何契约', async () => {
  const app = await launchMockApp();
  const win = await firstWindow(app);

  await win.getByRole('button', { name: '备份管理' }).click();
  await win.getByTestId('backup-row-docs-20261015-235500').click();

  const sheet = win.getByTestId('backup-sheet');
  await expect(sheet).toBeVisible();
  await expectFixedOverlay(win, 'sheet-overlay');
  await expectSheetOnRight(sheet);
  await expectNoHorizontalOverflow(win);
  await expect(sheet).toHaveScreenshot('sheet-detail.png');

  await sheet.getByRole('button', { name: '还原…' }).click();
  await sheet.getByLabel(/目标目录/).fill('/restored');
  await sheet.getByRole('button', { name: '开始还原' }).click();

  const dialog = win.getByTestId('password-dialog');
  await expect(dialog).toBeVisible();
  await expectFixedOverlay(win, 'dialog-overlay');
  await expectDialogCentered(dialog);
  await expectNoHorizontalOverflow(win);
  await expect(dialog).toHaveScreenshot('dialog-password.png');

  await dialog.getByRole('button', { name: '取消' }).click();
  await expect(dialog).toBeHidden();
  await app.close();
});

test('TC-E2E-12 筛选 Popover 的定位与令牌', async () => {
  const app = await launchMockApp();
  const win = await firstWindow(app);

  await win.getByRole('button', { name: '排除目录' }).click();
  const popover = win.locator('[data-slot="popover-content"]');
  await expect(popover).toBeVisible();
  // 等 150ms 的进入动效结束，避免量到动画中间帧的几何。
  await win.waitForTimeout(300);

  const viewport = await win.evaluate(() => ({ width: innerWidth, height: innerHeight }));
  const box = await popover.boundingBox();
  expect(box, 'Popover 应可见').not.toBeNull();
  if (box) {
    expect(box.x, 'Popover 不得超出左缘').toBeGreaterThanOrEqual(0);
    expect(box.y, 'Popover 不得超出上缘').toBeGreaterThanOrEqual(0);
    expect(box.x + box.width, 'Popover 不得超出右缘').toBeLessThanOrEqual(viewport.width + 1);
    expect(box.y + box.height, 'Popover 不得超出下缘').toBeLessThanOrEqual(viewport.height + 1);
    expect(box.width, 'Popover 宽度应为 288px').toBeGreaterThanOrEqual(280);
  }
  await expectRadius(popover, 16);
  await expectNoHorizontalOverflow(win);
  await expect(popover).toHaveScreenshot('popover-filter.png');

  await win.keyboard.press('Escape');
  await expect(popover).toBeHidden();
  await app.close();
});
