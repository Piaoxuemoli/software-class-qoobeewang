/**
 * seg-05-restore · 分镜 S5「还原」（目标 35s，录 36.5s）
 *
 * 成片时间轴（video-storyboard.json S5）：
 *   0.5–6   还原到新目录          → 打开 documents-demo 详情 → 还原… → 填目标目录
 *   6.5–12  口令校验              → 开始还原 → 口令对话框
 *   12.5–22 错误口令 E_AUTH/退出码 3 → 输入 wrong-demo，提示「还可尝试 2 次」，停留 3s+
 *   22.5–29 正确口令：开始还原      → 输入 demo-2026 → 真实还原（先整文件校验再写盘）
 *   29.5–34 先校验后落盘           → 完成卡「还原完成」并停留
 *
 * 实测（calibrate.mjs）：加密备份还原约 13.0s、错误口令被拒约 1.4s。
 * 因此正确口令要在 ~20s 前提交，完成卡约 33–34s 出现，留 ~2.5–3s 停留。
 * 错误口令只输 1 次，第 2 次必须是正确口令。
 */
import { centerOf, clickAt, moveTo } from '../shared/cursor.mjs';
import { readPercent } from '../shared/actions.mjs';
import { captureRestoreActualRun, writeActualRunReadme } from '../shared/actual-run.mjs';
import { runSegment } from '../shared/record.mjs';
import { PW, RESTORE, WRONG_PW } from '../shared/seed.mjs';

const ALERT_RE = /还可尝试 2 次/;

/** 本段真实运行结果（finalize 时写 actual-run，必须与视频同源）。 */
let actualCard = null;
let actualRestoreMs = null;

await runSegment({
  id: 'seg-05-restore',
  targetSec: 35,
  preset: 'history',
  note: 'S5 还原：documents-demo 详情 → 还原… → 目标目录 → 错误口令 E_AUTH（1 次）→ 正确口令 → 完成卡',
  async steps({ win, tl, ann, log }) {
    const manage = win.getByRole('button', { name: '备份管理', exact: true });
    const sheet = win.getByTestId('backup-sheet');
    const dialog = win.getByTestId('password-dialog');
    const pwInput = dialog.getByLabel('备份口令');
    const dlgStart = dialog.getByRole('button', { name: '开始还原' });

    // ---------- 0–2s：切到备份管理 → 打开 documents-demo 详情 ----------
    await tl.at(0.4, 'click-manage');
    const mc = await centerOf(manage, '备份管理');
    await moveTo(win, mc.x, mc.y, 600);
    await clickAt(win, mc.x, mc.y, { preMs: 350, postMs: 120 });
    await tl.state('manage-list', () =>
      win.getByTestId('backup-row-documents-demo').waitFor({ timeout: 30_000 }),
    );

    await tl.at(1.5, 'click-documents-demo');
    const demoRow = win.getByTestId('backup-row-documents-demo');
    const dc = await centerOf(demoRow, 'documents-demo 行');
    await moveTo(win, dc.x, dc.y, 450);
    await clickAt(win, dc.x, dc.y, { preMs: 300, postMs: 180 });
    await tl.state('detail-sheet', () => sheet.waitFor({ state: 'visible', timeout: 15_000 }));

    // ---------- 2–4s：还原… → 还原面板 ----------
    await tl.at(2.9, 'click-restore');
    const restoreBtn = sheet.getByRole('button', { name: '还原…' });
    const rbc = await centerOf(restoreBtn, '还原…');
    await moveTo(win, rbc.x, rbc.y, 500);
    await clickAt(win, rbc.x, rbc.y, { preMs: 400, postMs: 180 });
    await tl.state('restore-panel', () =>
      sheet.getByTestId('restore-panel').waitFor({ state: 'visible', timeout: 10_000 }),
    );

    // ---------- 4–6.5s：填目标目录 /home/demo/documents-restored ----------
    await tl.at(4.2, 'fill-target');
    const target = sheet.getByLabel(/目标目录/);
    const tc = await centerOf(target, '目标目录');
    await moveTo(win, tc.x, tc.y, 500);
    await clickAt(win, tc.x, tc.y, { preMs: 250, postMs: 100 });
    await target.pressSequentially(RESTORE, { delay: 32 });
    tl.mark('typed-target', { value: RESTORE });

    // ---------- 6.5–8s：开始还原 → 口令对话框 ----------
    await tl.at(6.6, 'click-start-restore');
    const startBtn = sheet.getByRole('button', { name: '开始还原' });
    const sbc = await centerOf(startBtn, '开始还原');
    await moveTo(win, sbc.x, sbc.y, 500);
    await clickAt(win, sbc.x, sbc.y, { preMs: 400, postMs: 150 });
    await tl.state('password-dialog', () => dialog.waitFor({ state: 'visible', timeout: 15_000 }));

    // ---------- 8–11s：错误口令 wrong-demo（只输 1 次） ----------
    await tl.at(8.3, 'fill-wrong-password');
    const pwc = await centerOf(pwInput, '备份口令');
    await moveTo(win, pwc.x, pwc.y, 420);
    await clickAt(win, pwc.x, pwc.y, { preMs: 200, postMs: 120 });
    await pwInput.fill(WRONG_PW);
    tl.mark('filled-wrong-password');

    await tl.at(9.6, 'submit-wrong-password');
    const dsbox = await dlgStart.boundingBox();
    if (!dsbox) throw new Error('口令对话框里的「开始还原」不可见');
    await moveTo(win, dsbox.x + dsbox.width / 2, dsbox.y + dsbox.height / 2, 420);
    await clickAt(win, dsbox.x + dsbox.width / 2, dsbox.y + dsbox.height / 2, {
      preMs: 350,
      postMs: 60,
    });
    tl.mark('wrong-password-submitted');

    // 后端先做整文件校验 → E_AUTH（约 1.4s）→ 对话框保留，红色提示「还可尝试 2 次」
    await tl.state('wrong-password-alert', () =>
      win.getByText(ALERT_RE).waitFor({ state: 'visible', timeout: 60_000 }),
    );
    const alertLoc = win.getByText(ALERT_RE);
    const alertAt = tl.contentMs();
    log(`错误口令提示出现 ≈ ${(alertAt / 1000).toFixed(2)}s（成片坐标）`);
    const alertBox = await alertLoc.boundingBox();
    if (alertBox)
      await moveTo(win, alertBox.x + alertBox.width * 0.4, alertBox.y + alertBox.height / 2, 500);
    await ann(
      'dialog-alert',
      alertLoc,
      'getByText(/还可尝试 2 次/)，PasswordDialog 内 role=alert 提示',
    );

    // ---------- 12.5–22s：错误提示停留（分镜要求停 3s 以上） ----------
    await win.waitForTimeout(3000);
    tl.mark('alert-hold-3s');
    await tl.holdUntil(15.4, 'alert-hold');
    // 光标回到口令输入框附近，准备输入正确口令
    const backC = await centerOf(pwInput, '备份口令');
    await moveTo(win, backC.x, backC.y, 550);

    // ---------- ~16–17s：正确口令 demo-2026 → 开始还原 ----------
    // 实测加密备份还原 ≈15.4s（verify+extract），要保证完成卡在 35s 镜头内出现并停留，
    // 正确口令必须在 ~17s 前提交（错误提示仍满足"停 3s 以上"）。
    await tl.at(16.2, 'fill-correct-password');
    await clickAt(win, backC.x, backC.y, { preMs: 150, postMs: 100 });
    await pwInput.fill(PW);
    tl.mark('filled-correct-password');

    await tl.at(16.9, 'submit-correct-password');
    const dsbox2 = await dlgStart.boundingBox();
    if (!dsbox2) throw new Error('口令对话框里的「开始还原」不可见（第 2 次）');
    const dsC = { x: dsbox2.x + dsbox2.width / 2, y: dsbox2.y + dsbox2.height / 2 };
    await moveTo(win, dsC.x, dsC.y, 400);
    await clickAt(win, dsC.x, dsC.y, { preMs: 300, postMs: 60 });
    const correctSubmittedAt = tl.contentMs();
    tl.mark('correct-password-submitted');
    log(`正确口令提交 ≈ ${(correctSubmittedAt / 1000).toFixed(2)}s（成片坐标）`);

    // ---------- 18–34s：进度（先校验后落盘）→ 完成卡「还原完成」 ----------
    await tl.state('restore-progress', () =>
      win.getByTestId('job-percent').first().waitFor({ timeout: 60_000 }),
    );
    const pctLoc = win.getByTestId('job-percent').first();
    const barBox = await pctLoc
      .locator('xpath=..')
      .boundingBox()
      .catch(() => null);
    if (barBox) {
      await moveTo(win, barBox.x + barBox.width * 0.5, barBox.y + barBox.height / 2, 650);
    }
    await ann(
      'job-progress',
      pctLoc.locator('xpath=..'),
      "getByTestId('job-percent') 的父级（进度条 + 百分比）",
    );

    // 轮询进度并记录 10/30/60/90%（注意：完成后 job-percent 会消失，
    // 必须用"先 isVisible 再 textContent"的 readPercent，否则 textContent 会自动等 30s）
    const tracked = [];
    const tPoll = Date.now();
    for (;;) {
      const pct = await readPercent(win);
      if (pct !== null) {
        for (const th of [10, 30, 60, 90]) {
          if (pct >= th && !tracked.includes(th)) {
            tracked.push(th);
            tl.mark(`state:restore-progress-${th}`, { percent: pct });
          }
        }
      }
      if (
        await win
          .getByTestId('job-succeeded')
          .first()
          .isVisible()
          .catch(() => false)
      )
        break;
      if (
        await win
          .getByTestId('job-failed')
          .first()
          .isVisible()
          .catch(() => false)
      ) {
        throw new Error('还原失败（job-failed）');
      }
      if (Date.now() - tPoll > 180_000) throw new Error('还原进度轮询超时');
      await win.waitForTimeout(100);
    }
    await tl.state('restore-succeeded', () =>
      win.getByTestId('job-succeeded').first().waitFor({ timeout: 30_000 }),
    );
    const succeededAt = tl.contentMs();
    log(
      `还原完成卡出现 ≈ ${(succeededAt / 1000).toFixed(2)}s；还原耗时（含 UI 往返）≈ ${((succeededAt - correctSubmittedAt) / 1000).toFixed(2)}s`,
    );
    tl.mark('restore-duration', { ms: Math.round(succeededAt - correctSubmittedAt) });

    // 完成卡读数 + 停留
    await win.waitForTimeout(800);
    const doneText = (
      await win
        .getByTestId('job-succeeded')
        .first()
        .innerText()
        .catch(() => '')
    ).trim();
    const readStat = async (label) =>
      (
        await win
          .getByTestId(`stat-${label}`)
          .innerText()
          .catch(() => '')
      ).trim();
    const stats = {
      files: await readStat('文件'),
      dirs: await readStat('目录'),
      bytes: await readStat('数据量'),
      ms: await readStat('耗时'),
    };
    tl.mark('restore-stats', { doneText, stats });
    actualCard = { doneText, ...stats };
    actualRestoreMs = Math.round(succeededAt - correctSubmittedAt);
    log(`完成卡：${doneText} ${JSON.stringify(stats)}`);

    // 鼠标移到完成卡上的「关闭」按钮附近停留（不点）
    const closeBtn = win.getByRole('button', { name: '关闭', exact: true });
    if (await closeBtn.isVisible().catch(() => false)) {
      const cb = await centerOf(closeBtn, '关闭');
      await moveTo(win, cb.x, cb.y, 650);
      tl.mark('hover-close-btn');
    }
    await tl.holdUntil(34.2, 'final-hold');
  },

  // app.close 之后、容器退出之前：捕获 diff/FIFO/软链接/仓库列表 + 完成卡数字（S6/S5 数据同源）
  async finalize({ video }) {
    const mode = process.env.SEG_MODE === 'formal' ? 'formal' : 'rehearsal';
    const pass = String(process.env.SEG_PASS || '1');
    const r = await captureRestoreActualRun({
      mode,
      pass,
      video,
      card: actualCard,
      restoreMs: actualRestoreMs,
    });
    const readme = await writeActualRunReadme({
      mode,
      pass,
      segments: [
        'seg-03-backup（若同批录制；完成卡与 .dbk）',
        'seg-05-restore（diff 无输出、FIFO/软链接类型、仓库 .dbk 列表、还原完成卡）',
      ],
    });
    return { ...r, readme };
  },
});
