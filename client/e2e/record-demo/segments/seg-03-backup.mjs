/**
 * seg-03-backup · 分镜 S3「备份执行」（目标 28s，录 29.5s）
 *
 * 成片时间轴（video-storyboard.json S3）：
 *   0.5–7   开始备份             → 段内先把 S2 表单快速填好（0–2s 内点「开始备份」）
 *   7.5–17  扫描 → 打包 → 压缩 → 加密（进度卡，来自后端 JSON Lines）
 *   17.5–26 完成：统计与压缩率    → 完成卡六项统计 + 输出路径；鼠标移到「查看备份」停留不点
 *
 * 真实后端速度（calibrate.mjs 实测 2026-10-18）：Huffman+AES 备份约 12.0s，
 * 因此 2s 点击 → 约 14–15s 出现完成卡（分镜 17.5 前 2–3s，属于"提前亮出"，可接受）。
 * 开始状态：空仓库 + 源树已预置；客户端停在备份页空闲态。
 */
import { centerOf, clickAt, moveTo } from '../shared/cursor.mjs';
import { click, trackProgress, waitEnabled } from '../shared/actions.mjs';
import { captureBackupActualRun, writeActualRunReadme } from '../shared/actual-run.mjs';
import { runSegment } from '../shared/record.mjs';
import { PW, SRC } from '../shared/seed.mjs';

/** 本段真实运行结果（finalize 时写 actual-run，必须与视频同源）。 */
let actualCard = null;
let actualBackupMs = null;

await runSegment({
  id: 'seg-03-backup',
  targetSec: 28,
  preset: 'empty',
  note: 'S3 备份执行：表单就绪 → 开始备份 → 进度 → 完成卡统计 → 悬停「查看备份」',
  async steps({ win, tl, ann, log }) {
    const source = win.getByLabel('源目录');
    const nameInput = win.getByLabel(/备份名称/);
    const excludeBtn = win.getByRole('button', { name: '排除目录' });
    const excludeInput = win.getByLabel(/排除目录名/);
    const compress = win.getByRole('switch', { name: '压缩' });
    const encrypt = win.getByRole('switch', { name: '加密' });
    const pw = win.getByLabel('口令', { exact: true });
    const pw2 = win.getByLabel('确认口令');
    const start = win.getByRole('button', { name: '开始备份' });

    // ---------- 0–2s：把 S2 的表单快速填好（本段独立录制，不依赖上一段） ----------
    // 预填必须压进 2s：动作只给最小停顿（动画可辨即可），2s 内点「开始备份」。
    await tl.at(0.15, 'prefill');
    await source.fill(SRC);
    await nameInput.fill('documents-demo');
    await click(win, excludeBtn, { moveMs: 140, preMs: 70, postMs: 60, label: '排除目录胶囊' });
    await excludeInput.waitFor({ state: 'visible', timeout: 8000 });
    await excludeInput.fill('node_modules');
    await win.keyboard.press('Escape');
    await win.waitForTimeout(80);
    await click(win, compress, { moveMs: 140, preMs: 70, postMs: 60, label: '压缩开关' });
    await click(win, encrypt, { moveMs: 140, preMs: 70, postMs: 60, label: '加密开关' });
    await pw.waitFor({ state: 'visible', timeout: 8000 });
    await pw.fill(PW);
    await pw2.fill(PW);
    await tl.state('form-ready', () =>
      waitEnabled(win, start, { timeoutMs: 15_000, what: '开始备份' }),
    );

    // ---------- 2s：移到「开始备份」并点击 ----------
    await tl.at(1.35, 'move-to-start');
    const stC = await centerOf(start, '开始备份');
    await moveTo(win, stC.x, stC.y, 400);
    await tl.at(1.65, 'click-start');
    await clickAt(win, stC.x, stC.y, { preMs: 350, postMs: 60 });
    const clickedAt = tl.videoMs();
    tl.mark('backup-submitted');

    // ---------- 2–17s：进度（扫描 → 打包 → 压缩 → 加密） ----------
    await tl.state('progress-visible', () =>
      win.getByTestId('job-percent').first().waitFor({ timeout: 60_000 }),
    );
    const pctLoc = win.getByTestId('job-percent').first();
    const barLoc = pctLoc.locator('xpath=..');
    const barBox = await barLoc.boundingBox().catch(() => null);
    const barCenter = barBox
      ? { x: barBox.x + barBox.width * 0.5, y: barBox.y + barBox.height / 2 }
      : { x: 620, y: 470 };
    await moveTo(win, barCenter.x, barCenter.y, 700);
    tl.mark('hover-progress-bar');
    await ann('job-percent', pctLoc, "getByTestId('job-percent')（进度条右侧百分比）");

    // 轮询进度：记录 10/30/60/90% 的时刻，等完成卡
    const tracker = await trackProgress(win, tl, { timeoutMs: 120_000 });
    if (!tracker.done) throw new Error('备份任务未成功（出现 job-failed）');
    const succeededAt = tl.videoMs();

    // ---------- 完成卡：六项统计 + 输出路径 ----------
    await win.waitForTimeout(800); // 关键状态后停 0.8s
    tl.mark('success-settle');

    const outStat = win.getByTestId('stat-输出文件');
    const ratioStat = win.getByTestId('stat-压缩率');
    // 先在统计区上方轻轻移到「文件」统计，再依次对应注解时间点（输出文件 20–24、压缩率 24.5–28）
    const statFiles = win.getByTestId('stat-文件');
    if (await statFiles.isVisible().catch(() => false)) {
      const fc = await centerOf(statFiles, 'stat-文件');
      await moveTo(win, fc.x, fc.y, 650);
      tl.mark('hover-stat-文件');
    }
    await tl.at(19.9, 'move-stat-输出文件');
    const oc = await centerOf(outStat, 'stat-输出文件');
    await moveTo(win, oc.x, oc.y, 600);
    tl.mark('hover-stat-输出文件');
    await ann('stat-输出文件', outStat, "getByTestId('stat-输出文件')");

    await tl.at(24.2, 'move-stat-压缩率');
    const rc = await centerOf(ratioStat, 'stat-压缩率');
    await moveTo(win, rc.x, rc.y, 600);
    tl.mark('hover-stat-压缩率');
    await ann('stat-压缩率', ratioStat, "getByTestId('stat-压缩率')");

    // 完成卡读数（写进日志，供 QC 与字幕核对）
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
      output: await readStat('输出文件'),
      ratio: await readStat('压缩率'),
    };
    tl.mark('stats-read', stats);
    actualCard = stats;
    actualBackupMs = Math.round(succeededAt - clickedAt);
    log(
      `完成卡：${JSON.stringify(stats)}；备份耗时（含 UI 往返）≈ ${((succeededAt - clickedAt) / 1000).toFixed(2)}s`,
    );

    // ---------- 24–28s：鼠标移到「查看备份」停留不点 ----------
    await tl.at(25.6, 'hover-view-backup');
    const view = win.getByRole('button', { name: '查看备份' });
    const vc = await centerOf(view, '查看备份');
    await moveTo(win, vc.x, vc.y, 650);
    tl.mark('hover-view-backup');
    await tl.holdUntil(27.6, 'final-hold');
  },

  // app.close 之后、容器退出之前：捕获本次真实运行的 .dbk 与完成卡数字（S6/S3 数据同源）
  async finalize({ video }) {
    const mode = process.env.SEG_MODE === 'formal' ? 'formal' : 'rehearsal';
    const pass = String(process.env.SEG_PASS || '1');
    const r = await captureBackupActualRun({
      mode,
      pass,
      video,
      card: actualCard,
      backupMs: actualBackupMs,
    });
    const readme = await writeActualRunReadme({
      mode,
      pass,
      segments: ['seg-03-backup（完成卡六项统计 + documents-demo.dbk 实际输出）'],
    });
    return { ...r, readme };
  },
});
