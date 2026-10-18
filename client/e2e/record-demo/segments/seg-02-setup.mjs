/**
 * seg-02-setup · 分镜 S2「界面编排」（目标 37s，录 38.5s）
 *
 * 成片时间轴（video-storyboard.json S2 字幕）：
 *   0.5–6   选择演示源目录       → 鼠标移到源目录、填 /home/demo/documents
 *   6.5–12  排除 node_modules    → 点「排除目录」胶囊、输入 node_modules、Esc
 *   12.5–18 命名并选择算法       → 备份名称 documents-demo
 *   18.5–24 压缩：Huffman        → 打开压缩开关
 *   24.5–30 加密：AES-256        → 打开加密开关、输入演示口令（圆点）
 *   30.5–36 预览：将备份的数量   → dry-run 预览行「预计 …」出现并停留
 *
 * 开始状态：空仓库 + 源树已预置；客户端停在备份页空闲态。
 */
import { centerOf, clickAt, moveTo } from '../shared/cursor.mjs';
import { click, typeInto } from '../shared/actions.mjs';
import { runSegment } from '../shared/record.mjs';
import { PW, SRC } from '../shared/seed.mjs';

await runSegment({
  id: 'seg-02-setup',
  targetSec: 37,
  preset: 'empty',
  note: 'S2 界面编排：源目录 → 排除 node_modules → 命名 → 压缩 → 加密 → 口令 → dry-run 预览',
  async steps({ win, tl, log }) {
    const source = win.getByLabel('源目录');
    const excludeBtn = win.getByRole('button', { name: '排除目录' });
    const excludeInput = win.getByLabel(/排除目录名/);
    const nameInput = win.getByLabel(/备份名称/);
    const compress = win.getByRole('switch', { name: '压缩' });
    const encrypt = win.getByRole('switch', { name: '加密' });
    const pw = win.getByLabel('口令', { exact: true });
    const pw2 = win.getByLabel('确认口令');
    const preview = win.getByTestId('dryrun-preview');

    // ---------- 0.5–6s：源目录 ----------
    await tl.at(0.9, 'hover-source');
    const sc = await centerOf(source, '源目录');
    await moveTo(win, sc.x, sc.y, 700);
    await tl.at(3.1, 'fill-source');
    await clickAt(win, sc.x, sc.y, { preMs: 350, postMs: 120 });
    await source.pressSequentially(SRC, { delay: 38 });
    tl.mark('typed-source', { value: SRC });

    // ---------- 6.5–12s：排除目录 node_modules ----------
    await tl.at(6.6, 'open-filter');
    await click(win, excludeBtn, { moveMs: 600, preMs: 400, postMs: 150, label: '排除目录胶囊' });
    await tl.state('filter-popover', () =>
      excludeInput.waitFor({ state: 'visible', timeout: 10_000 }),
    );
    await tl.at(8.4, 'type-exclude');
    const xc = await centerOf(excludeInput, '排除目录名');
    await moveTo(win, xc.x, xc.y, 450);
    await clickAt(win, xc.x, xc.y, { preMs: 250, postMs: 100 });
    await excludeInput.pressSequentially('node_modules', { delay: 55 });
    await win.waitForTimeout(250);
    await win.keyboard.press('Escape');
    tl.mark('closed-filter-popover');
    await tl.state('filter-capsule-active', () =>
      win.getByText(/排除目录\s*·\s*1 条/).waitFor({ timeout: 8000 }),
    );

    // ---------- 12.5–18s：备份名称 ----------
    await tl.at(12.6, 'fill-name');
    await typeInto(win, nameInput, 'documents-demo', { moveMs: 600, delay: 50, label: '备份名称' });
    tl.mark('typed-name', { value: 'documents-demo' });

    // ---------- 18.5–24s：压缩 ----------
    await tl.at(18.6, 'toggle-compress');
    await click(win, compress, { moveMs: 600, preMs: 400, postMs: 200, label: '压缩开关' });
    await tl.state('compress-on', () =>
      win.getByRole('switch', { name: '压缩', checked: true }).waitFor({ timeout: 8000 }),
    );

    // ---------- 24.5–30s：加密 + 口令 ----------
    await tl.at(24.6, 'toggle-encrypt');
    await click(win, encrypt, { moveMs: 600, preMs: 400, postMs: 250, label: '加密开关' });
    await tl.state('encrypt-on', () =>
      win.getByRole('switch', { name: '加密', checked: true }).waitFor({ timeout: 8000 }),
    );
    await tl.state('password-fields', () => pw.waitFor({ state: 'visible', timeout: 8000 }));

    await tl.at(26.9, 'fill-password');
    const pwC = await centerOf(pw, '口令');
    await moveTo(win, pwC.x, pwC.y, 450);
    await clickAt(win, pwC.x, pwC.y, { preMs: 250, postMs: 150 });
    await pw.fill(PW); // 口令不逐字输入（分镜要求：直接 fill，显示为圆点）
    tl.mark('filled-password');

    await tl.at(28.2, 'fill-confirm');
    const pw2C = await centerOf(pw2, '确认口令');
    await moveTo(win, pw2C.x, pw2C.y, 450);
    await clickAt(win, pw2C.x, pw2C.y, { preMs: 200, postMs: 120 });
    await pw2.fill(PW);
    tl.mark('filled-confirm');

    // ---------- 30.5–36s：dry-run 预览 ----------
    // 口令变化后 400ms 防抖 + 后端 dry-run 扫描（约 500MB / 400+ 文件）
    await tl.at(30.4, 'wait-preview');
    await tl.state('dryrun-preview', () =>
      preview.filter({ hasText: /预计/ }).waitFor({ timeout: 60_000 }),
    );
    const previewText = (await preview.innerText()).trim();
    tl.mark('preview-ready', { text: previewText });
    log(`preview: ${JSON.stringify(previewText)}`);

    await tl.holdUntil(36.4, 'preview-hold');
  },

  async annotations({ win, ann }) {
    await ann(
      'filter-capsules',
      win.getByTestId('filter-capsules'),
      "getByTestId('filter-capsules')",
    );
    await ann(
      'switch-compress',
      win.getByRole('switch', { name: '压缩' }),
      "getByRole('switch', { name: '压缩' })",
    );
    await ann(
      'switch-encrypt',
      win.getByRole('switch', { name: '加密' }),
      "getByRole('switch', { name: '加密' })",
    );
    await ann('dryrun-preview', win.getByTestId('dryrun-preview'), "getByTestId('dryrun-preview')");
  },
});
