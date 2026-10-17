/* global window, document, HTMLElement -- 回调在渲染进程里执行 */
/**
 * 生成 README / 答辩 PPT 用的客户端截图（真实后端，非 mock）。
 *
 * 与 app.spec.ts 的区别：不比对基线、不做断言，只负责把一个"看起来像真实
 * 使用现场"的应用状态截下来——因此需要一个固定的、可读的路径（/home/wang），
 * 以及若干真实 `.dbk` 备份做列表。
 *
 * 运行（容器内；Xvfb 需 2560x1600——2 倍像素下 1280x800 会把窗口压到最小尺寸）：
 *
 *   cd client
 *   Xvfb :98 -screen 0 2560x1600x24 &
 *   DISPLAY=:98 DATABACKUP_BIN=../backend/bin/databackup node e2e/capture-showcase.mjs
 *
 * 产物默认写到 `../docs/images/client/`，可用 SHOWCASE_OUT 覆盖。
 * 脚本会重建 /home/wang 与 /tmp/databackup-showcase，仅可在容器/临时环境运行。
 */
import { execFile } from 'node:child_process';
import { mkdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { _electron as electron } from '@playwright/test';

const clientDir = path.resolve(import.meta.dirname, '..');
const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');
const outDir = path.resolve(clientDir, process.env.SHOWCASE_OUT ?? '../docs/images/client');

const HOME = '/home/wang';
const SRC = `${HOME}/documents`;
const REPO = `${HOME}/DataBackup`;
const RESTORE = `${HOME}/documents-restored`;
const PW = 'demo-2026';

/** 确定性伪随机字节：截图里的体积数字每次一致，便于文档引用。 */
function bytes(n, seed = 0x9e3779b9) {
  const buf = Buffer.alloc(n);
  let s = seed >>> 0;
  for (let i = 0; i < n; i++) {
    s ^= s << 13;
    s >>>= 0;
    s ^= s >>> 17;
    s ^= s << 5;
    s >>>= 0;
    buf[i] = s & 0xff;
  }
  return buf;
}

async function write(rel, data) {
  const p = path.join(SRC, rel);
  await mkdir(path.dirname(p), { recursive: true });
  await writeFile(p, data);
}

/** 造一棵"像课程作业里的资料盘"的源目录树：多级目录、中文名、大小分层。 */
async function makeSourceTree() {
  const text = (s) => Buffer.from(`${s}\n`.repeat(40));
  await write('说明.txt', text('归档说明：本目录用于演示 DataBackup 的备份与还原。'));
  await write('报告/开题报告.md', text('# 开题报告\n\n- 选题：Linux 目录树备份\n- 组长：王科瑾'));
  await write('报告/中期检查.md', text('# 中期检查\n\n后端流水线与客户端三页面已完成。'));
  await write('合同/2026-服务协议.pdf', bytes(1_800_000, 11));
  await write('合同/2025-保密协议.pdf', bytes(900_000, 12));
  for (let i = 1; i <= 8; i++) {
    await write(`照片/2026-10 成都/IMG_${String(i).padStart(4, '0')}.jpg`, bytes(1_500_000, i));
  }
  for (let i = 1; i <= 3; i++) {
    await write(`代码/site/src/module-${i}.ts`, text(`export const module${i} = ${i};`));
  }
  // 被「排除目录」过滤掉的部分：让预览行出现"跳过"数字，截图能体现过滤能力。
  for (let i = 1; i <= 120; i++) {
    await write(`代码/site/node_modules/pkg-${i}/index.js`, text(`module.exports = ${i};`));
  }
  // 等大的文本分片：进度百分比按"已处理文件数"推进，文件大小相近时
  // 百分比与字节数同步增长，截出来的进度帧才不会是 98%。
  const part = Buffer.from(
    '2026-10-17 09:12:33 INFO  sync chunk ok  size=4096 sha=9f2c1a7e\n'.repeat(26_000),
  );
  for (let i = 1; i <= 300; i++) {
    await write(`资料/归档/part-${String(i).padStart(3, '0')}.txt`, part);
  }
  await write('资料/数据集/原始数据.bin', bytes(16_000_000, 21));
}

function cli(args, stdin) {
  return new Promise((resolve, reject) => {
    const child = execFile(backendBin, ['--json', ...args], { env: { ...process.env, HOME } });
    let out = '';
    child.stdout?.on('data', (c) => (out += c));
    child.on('error', reject);
    child.on('exit', (code) => (code === 0 ? resolve(out) : reject(new Error(`cli exit ${code}`))));
    child.stdin?.end(stdin === undefined ? undefined : `${stdin}\n`);
  });
}

/** 预置三个真实备份，让「备份管理」页是一个有内容的列表而不是空态。 */
async function seedBackups() {
  const archive = `${HOME}/archive`;
  await mkdir(`${archive}/photos/2026-09`, { recursive: true });
  await mkdir(`${archive}/contracts/2026`, { recursive: true });
  await mkdir(`${archive}/reports`, { recursive: true });
  await writeFile(`${archive}/photos/2026-09/a.jpg`, bytes(2_400_000, 31));
  await writeFile(`${archive}/photos/2026-09/b.jpg`, bytes(1_600_000, 32));
  await writeFile(`${archive}/photos/c.jpg`, bytes(1_100_000, 34));
  await writeFile(`${archive}/contracts/2026/service.pdf`, bytes(700_000, 33));
  await writeFile(`${archive}/contracts/2026/nda.pdf`, bytes(420_000, 35));
  await writeFile(`${archive}/contracts/readme.txt`, Buffer.from('合同扫描件，按年份归档。\n'));
  await writeFile(
    `${archive}/reports/week-41.md`,
    Buffer.from('# 第 41 周\n\n- 联调完成\n'.repeat(400)),
  );
  await writeFile(
    `${archive}/reports/week-42.md`,
    Buffer.from('# 第 42 周\n\n- 文档定稿\n'.repeat(600)),
  );

  await cli([
    'backup',
    '--source',
    `${archive}/photos`,
    '--repo',
    REPO,
    '--name',
    'photos-20261010',
    '--compress',
    'huffman',
  ]);
  await cli(
    [
      'backup',
      '--source',
      `${archive}/contracts`,
      '--repo',
      REPO,
      '--name',
      'archive-20261001',
      '--compress',
      'huffman',
      '--encrypt',
      'aes256',
      '--password-stdin',
    ],
    PW,
  );
  await cli([
    'backup',
    '--source',
    `${archive}/reports`,
    '--repo',
    REPO,
    '--name',
    'daily-20261015',
    '--compress',
    'huffman',
  ]);
}

const noSandbox = typeof process.getuid === 'function' && process.getuid() === 0;

async function main() {
  await rm(HOME, { recursive: true, force: true });
  await rm(RESTORE, { recursive: true, force: true });
  await mkdir(HOME, { recursive: true });
  await mkdir(outDir, { recursive: true });
  await makeSourceTree();
  await seedBackups();

  const app = await electron.launch({
    // 2x 设备像素：PPT 全宽投放不糊，README 在 Retina 屏上也清晰。
    args: [...(noSandbox ? ['--no-sandbox'] : []), '--force-device-scale-factor=2', '.'],
    cwd: clientDir,
    env: { ...process.env, HOME, XDG_CONFIG_HOME: `${HOME}/.config`, DATABACKUP_BIN: backendBin },
  });

  const win = await app.firstWindow();
  await win.getByTestId('statusbar').waitFor();
  // 版本号到位即后端已就绪（启动时 app.versions() 返回）。
  await win
    .getByTestId('statusbar')
    .filter({ hasText: /后端版本: 1\./ })
    .waitFor();

  const shot = async (name, settle = 450) => {
    // 去掉按钮/输入框的焦点环，并确保没有任何滚动偏移（截图是可交付的静态图）。
    await win.evaluate(() => {
      if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
      window.scrollTo(0, 0);
    });
    if (settle > 0) await win.waitForTimeout(settle); // 让 200~300ms 的布局/Sheet 动效落定
    await win.screenshot({ path: path.join(outDir, `${name}.png`) });
    console.log(`captured ${name}.png`);
  };

  // 1) 备份页：填好源目录、排除 node_modules、压缩+加密，停在预览就绪态。
  await win.getByLabel('源目录').fill(SRC);
  await win.getByRole('button', { name: '排除目录' }).click();
  await win.getByLabel(/排除目录名/).fill('node_modules');
  await win.keyboard.press('Escape');
  await win.getByLabel(/备份名称/).fill('documents-20261018');
  await win.getByRole('switch', { name: '压缩' }).click();
  await win.getByRole('switch', { name: '加密' }).click();
  await win.getByLabel('口令', { exact: true }).fill(PW);
  await win.getByLabel('确认口令').fill(PW);
  await win.getByTestId('dryrun-preview').filter({ hasText: /预计/ }).waitFor({ timeout: 60_000 });
  await shot('01-backup');

  // 2) 进度：轮询到 25% 左右再截（百分比按文件数推进，源树大小均匀）。
  await win.getByRole('button', { name: '开始备份' }).click();
  const pct = win.getByTestId('job-percent');
  for (let i = 0; i < 400; i++) {
    const t = await pct.textContent().catch(() => null);
    if (Number((t ?? '').replace('%', '')) >= 25) break;
    await win.waitForTimeout(100);
  }
  await shot('02-progress', 0);

  // 3) 完成态摘要。
  await win.getByTestId('job-succeeded').waitFor({ timeout: 120_000 });
  await shot('03-done');

  // 4) 备份管理：三个预置备份 + 刚完成的这次。
  await win.getByRole('button', { name: '备份管理' }).click();
  await win.getByTestId('backup-row-documents-20261018').waitFor();
  await shot('04-manage');

  // 5) 详情 Sheet（选加密的那一个，标题带「加密」徽标）。
  await win.getByTestId('backup-row-archive-20261001').click();
  await win.getByTestId('backup-sheet').waitFor();
  await shot('05-detail');

  // 6) 口令对话框：加密备份的还原入口。
  const sheet = win.getByTestId('backup-sheet');
  await sheet.getByRole('button', { name: '还原…' }).click();
  await sheet.getByLabel(/目标目录/).fill(RESTORE);
  await sheet.getByRole('button', { name: '开始还原' }).click();
  await win.getByTestId('password-dialog').waitFor();
  await shot('06-password');
  await win.getByTestId('password-dialog').getByRole('button', { name: '取消' }).click();
  await win.getByTestId('password-dialog').waitFor({ state: 'hidden' });

  // 6b) 关掉详情 Sheet（遮罩会挡住侧栏），再进设置页。
  await win.getByRole('button', { name: '关闭' }).click();
  await win.getByTestId('backup-sheet').waitFor({ state: 'detached' });

  // 7) 设置页。
  await win.getByRole('button', { name: '设置' }).click();
  await shot('07-settings');

  await app.close();
  console.log(`done -> ${outDir}`);
}

await main();
