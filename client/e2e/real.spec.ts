import { mkdir, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { assertTreeEqual, expect, makeBigTree, runCli, stubOpenDialog, test } from './fixtures';

/**
 * 真实后端链路（DATABACKUP_MOCK 未设，spawn 真二进制）：
 * 验收 roadmap C3——界面完成一次"过滤+压缩+加密"备份并还原成功，
 * 取消、口令错误路径通过。断言三层：UI 终态 + 仓库文件 + 还原后的目录树。
 */

const PW = 'e2e-secret-pw';

test('TC-E2E-07 过滤+压缩+加密备份 → 列表 → 口令还原，目录树原样', async ({ app, env, win }) => {
  test.setTimeout(120_000);

  // 编排：源目录 + 排除目录过滤 + 压缩 + 加密（口令进 stdin）
  await win.getByLabel('源目录').fill(env.src);
  // 注意：--exclude-dir 按契约接相对源目录的路径（不是正则），字面量 node_modules。
  await win.getByRole('button', { name: '排除目录' }).click();
  await win.getByLabel(/排除目录名/).fill('node_modules');
  await win.keyboard.press('Escape');
  await win.getByLabel(/备份名称/).fill('e2e-real');
  await win.getByRole('switch', { name: '压缩' }).click();
  await win.getByRole('switch', { name: '加密' }).click();
  await win.getByLabel('口令', { exact: true }).fill(PW);
  await win.getByLabel('确认口令').fill(PW);
  await win.getByRole('button', { name: '开始备份' }).click();
  await expect(win.getByTestId('job-succeeded')).toContainText('备份完成', { timeout: 60_000 });
  await expect(win).toHaveScreenshot('real-backup-done.png', {
    mask: [
      // 耗时与临时路径每次运行都不同，其余统计（文件/目录/数据量/输出/压缩率）应稳定。
      win.getByTestId('stat-耗时'),
      win.getByTestId('sidebar-repo'),
      win.locator('div.truncate.font-mono'),
    ],
  });

  // 仓库层断言：备份文件真实落盘
  expect(await readdir(env.repo)).toContain('e2e-real.dbk');

  // 备份管理页出现新条目
  await win.getByRole('button', { name: '查看备份' }).click();
  await expect(win.getByTestId('backup-row-e2e-real')).toBeVisible();
  await win.getByTestId('backup-row-e2e-real').click();
  await expect(win.getByTestId('backup-sheet')).toBeVisible();

  // 还原：目标目录走原生"浏览…"对话框（已 stub 到 main 进程，覆盖 dialogs.ts 链路）
  const sheet = win.getByTestId('backup-sheet');
  await stubOpenDialog(app, [env.dst]);
  await sheet.getByRole('button', { name: '还原…' }).click();
  await sheet.getByRole('button', { name: '浏览…' }).click();
  await expect(sheet.getByLabel(/目标目录/)).toHaveValue(env.dst);
  await sheet.getByRole('button', { name: '开始还原' }).click();

  // 加密备份 → 口令对话框 → 正确口令 → 完成
  const dialog = win.getByTestId('password-dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('textbox', { name: '备份口令' }).fill(PW);
  await dialog.getByRole('button', { name: '开始还原' }).click();
  await expect(win.getByTestId('job-succeeded')).toContainText('还原完成', { timeout: 60_000 });

  // 文件系统层断言：还原树 = 源树（node_modules 已按过滤规则排除）
  await assertTreeEqual(env.src, env.dst, { excludeDirs: [/^node_modules$/] });
});

test('TC-E2E-08 备份进行中取消：UI 已取消态，仓库无 .partial 残留', async ({ env, win }) => {
  test.setTimeout(120_000);
  const bigSrc = path.join(env.root, 'bigsrc');
  await makeBigTree(bigSrc);

  await win.getByLabel('源目录').fill(bigSrc);
  await win.getByLabel(/备份名称/).fill('e2e-cancel');
  await win.getByRole('switch', { name: '压缩' }).click();
  await win.getByRole('button', { name: '开始备份' }).click();

  // 等第一个 progress 事件到达界面再取消：此时 jobId 已注册，cancel() 才会生效。
  await expect(
    win.getByTestId('job-percent').or(win.getByTestId('current-path')).first(),
  ).toBeVisible({ timeout: 30_000 });
  await win.getByRole('button', { name: '取消' }).click();
  await expect(win.getByTestId('job-cancelled')).toContainText('已取消', { timeout: 30_000 });

  // 取消后仓库不得留下该备份的成品或半成品文件
  const names = await readdir(env.repo).catch(() => [] as string[]);
  expect(names.filter((n) => n.includes('e2e-cancel'))).toEqual([]);
});

test('TC-E2E-09 还原加密备份：错误口令 E_AUTH → 重试成功', async ({ env, win }) => {
  test.setTimeout(120_000);
  // 前置：用真实 CLI 造一个加密备份（本条只考 UI 的口令重试路径）
  const seedSrc = path.join(env.root, 'seed');
  await mkdir(path.join(seedSrc, 'docs'), { recursive: true });
  await writeFile(path.join(seedSrc, 'readme.txt'), 'seed\n');
  await writeFile(path.join(seedSrc, 'docs', 'a.bin'), Buffer.alloc(2048, 7));
  await runCli(
    [
      'backup',
      '--source',
      seedSrc,
      '--repo',
      env.repo,
      '--name',
      'e2e-secret',
      '--encrypt',
      'aes256',
      '--password-stdin',
    ],
    'e2e-pw',
  );

  // 列表是启动时预取的（backupsStore.refresh），造种发生在启动后 → 手动刷新一次。
  await win.getByRole('button', { name: '备份管理' }).click();
  await win.getByRole('button', { name: '刷新' }).click();
  await expect(win.getByTestId('backup-row-e2e-secret')).toBeVisible();
  await win.getByTestId('backup-row-e2e-secret').click();
  const sheet = win.getByTestId('backup-sheet');
  await expect(sheet).toBeVisible();
  await sheet.getByRole('button', { name: '还原…' }).click();
  await sheet.getByLabel(/目标目录/).fill(env.dst);
  await sheet.getByRole('button', { name: '开始还原' }).click();

  const dialog = win.getByTestId('password-dialog');
  await expect(dialog).toBeVisible();

  // 错误口令 → 后端 E_AUTH（退出码 3）→ 回到对话框并提示剩余次数
  await dialog.getByRole('textbox', { name: '备份口令' }).fill('wrong-pw');
  await dialog.getByRole('button', { name: '开始还原' }).click();
  await expect(dialog.getByRole('alert')).toContainText('还可尝试 2 次', { timeout: 30_000 });

  // 正确口令 → 还原完成 + 目录树一致
  await dialog.getByRole('textbox', { name: '备份口令' }).fill('e2e-pw');
  await dialog.getByRole('button', { name: '开始还原' }).click();
  await expect(win.getByTestId('job-succeeded')).toContainText('还原完成', { timeout: 60_000 });
  await assertTreeEqual(seedSrc, env.dst);
});
