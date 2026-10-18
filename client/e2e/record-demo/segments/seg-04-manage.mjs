/**
 * seg-04-manage · 分镜 S4「备份管理」（目标 25s，录 26.5s）
 *
 * 成片时间轴（video-storyboard.json S4）：
 *   0.5–6   备份管理              → 点侧栏「备份管理」，列表 4 行（3 历史 + documents-demo）
 *   6.5–13  列表：大小/加密/完整性 → 鼠标依次划过各行（行悬停高亮）
 *   13.5–20 详情来自容器头尾       → 点 documents-demo 行，右侧详情 Sheet 滑出
 *   20.5–24 list 不需要口令        → 鼠标在详情行间移动（源目录/文件数/算法）
 *
 * 本段是独立段：不依赖 seg-03 的完成卡，从「备份管理」页开始。
 * 开始状态：仓库预置 daily / photos / archive(加密) / documents-demo(加密)。
 */
import { centerOf, clickAt, moveTo } from '../shared/cursor.mjs';
import { runSegment } from '../shared/record.mjs';

/** 列表行高亮停留：把光标放到行内文字左侧（不压住文字）。 */
async function hoverRow(win, row, moveMs = 450) {
  const box = await row.boundingBox();
  if (!box) throw new Error('列表行不可见');
  await moveTo(win, box.x + Math.min(120, box.width * 0.16), box.y + box.height / 2, moveMs);
}

await runSegment({
  id: 'seg-04-manage',
  targetSec: 25,
  preset: 'history',
  note: 'S4 备份管理：切页 → 依次悬停列表行 → documents-demo 详情 Sheet → 详情行间移动',
  async steps({ win, tl, ann, log }) {
    const manage = win.getByRole('button', { name: '备份管理', exact: true });
    const sheet = win.getByTestId('backup-sheet');

    // ---------- 0.5–6s：切到备份管理页（列表已预置 4 行） ----------
    await tl.at(0.5, 'click-manage');
    const mc = await centerOf(manage, '备份管理');
    await moveTo(win, mc.x, mc.y, 650);
    await clickAt(win, mc.x, mc.y, { preMs: 400, postMs: 150 });
    await tl.state('manage-list', () =>
      win.getByTestId('backup-row-documents-demo').waitFor({ timeout: 30_000 }),
    );
    const rowIds = await win
      .locator('[data-testid^="backup-row-"]')
      .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid')));
    log(`列表行：${JSON.stringify(rowIds)}`);
    tl.mark('list-rows', { rows: rowIds });

    // ---------- 6.5–13s：鼠标依次划过各行 ----------
    for (let i = 0; i < rowIds.length; i += 1) {
      await tl.at(3.2 + i * 1.15, `hover-row-${i + 1}`);
      const row = win.getByTestId(rowIds[i]);
      await hoverRow(win, row, 480);
      tl.mark(`hovered-${rowIds[i].replace('backup-row-', '')}`);
    }

    // 加密/完整性两列坐标（注解用，趁列表无遮挡采样）
    const demoRow = win.getByTestId('backup-row-documents-demo');
    await ann(
      'row-encrypted',
      demoRow.locator('td').nth(3),
      'backup-row-documents-demo td:nth(3)（加密列）',
    );
    await ann(
      'row-integrity',
      demoRow.locator('td').nth(4),
      'backup-row-documents-demo td:nth(4)（完整性列）',
    );

    // ---------- 8–13s：点 documents-demo 行 → 详情 Sheet ----------
    await tl.at(8.4, 'click-documents-demo');
    const dc = await centerOf(demoRow, 'documents-demo 行');
    await moveTo(win, dc.x, dc.y, 500);
    await clickAt(win, dc.x, dc.y, { preMs: 450, postMs: 200 });
    await tl.state('detail-sheet', () => sheet.waitFor({ state: 'visible', timeout: 15_000 }));
    tl.mark('sheet-open');

    // ---------- 13.5–20s：鼠标在详情行间移动 ----------
    const detailRow = (label) =>
      sheet.locator('div.flex.justify-between', { hasText: label }).first();
    const labels = ['创建时间', '文件数', '数据量', '输出大小', '加密', '格式版本'];
    for (let i = 0; i < labels.length; i += 1) {
      await tl.at(10.0 + i * 0.85, `detail-row-${labels[i]}`);
      const box = await detailRow(labels[i])
        .boundingBox()
        .catch(() => null);
      if (!box) continue;
      await moveTo(win, box.x + box.width * 0.55, box.y + box.height / 2, 320);
    }

    // 源目录行（注解 detail-source；15–18.5s 是字幕「详情来自容器头尾」窗口）
    await tl.at(15.0, 'hover-detail-source');
    const srcRow = detailRow('源目录');
    const srcBox = await srcRow.boundingBox().catch(() => null);
    if (srcBox)
      await moveTo(win, srcBox.x + srcBox.width * 0.45, srcBox.y + srcBox.height / 2, 450);
    tl.mark('hover-detail-source');
    await ann(
      'detail-source',
      srcRow,
      'backup-sheet 内 label=源目录 的详情行 div.flex.justify-between',
    );

    // ---------- 20.5–24s：移到「还原…」按钮停留（不点，准备进 S5） ----------
    await tl.at(20.6, 'hover-restore-btn');
    const restoreBtn = sheet.getByRole('button', { name: '还原…' });
    const rbox = await restoreBtn.boundingBox();
    if (rbox) await moveTo(win, rbox.x + rbox.width / 2, rbox.y + rbox.height / 2, 650);
    tl.mark('hover-restore-btn');
    await tl.holdUntil(23.8, 'final-hold');
  },
});
