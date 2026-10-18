/**
 * 段脚本共用的"像真人"的界面动作（鼠标轨迹 + 停顿 + 状态等待）。
 * 时间轴由各段用 Timeline.at() 控制，这里只负责动作本身。
 */
import { centerOf, clickAt, moveTo } from './cursor.mjs';

/** 鼠标移动（默认 0.5–0.8s）→ 点击前停顿 → 点击（真实 mousedown/up，触发光标涟漪）。 */
export async function click(
  win,
  locator,
  { moveMs = 600, preMs = 400, postMs = 120, label = 'target' } = {},
) {
  const c = await centerOf(locator, label);
  await moveTo(win, c.x, c.y, moveMs);
  await clickAt(win, c.x, c.y, { preMs, postMs });
  return c;
}

/** 移动到位后只悬停不点击（用于"鼠标移到按钮上停留不点"的分镜）。 */
export async function hover(win, locator, { moveMs = 600, label = 'target', xRatio = 0.5 } = {}) {
  const c = await centerOf(locator, label);
  return moveTo(win, c.x + (xRatio - 0.5) * 2 * 0, c.y, moveMs);
}

/** 移动 → 点击聚焦 → 逐字输入（可压缩文本用；口令一律用 fill）。 */
export async function typeInto(
  win,
  locator,
  text,
  { moveMs = 550, delay = 45, preMs = 300, label = 'input' } = {},
) {
  const c = await centerOf(locator, label);
  await moveTo(win, c.x, c.y, moveMs);
  await clickAt(win, c.x, c.y, { preMs, postMs: 120 });
  await locator.pressSequentially(text, { delay });
  return c;
}

/** 轮询等到谓词为真（用于 enabled/文本等非"可见性"状态）。 */
export async function waitUntil(
  win,
  fn,
  { timeoutMs = 30_000, intervalMs = 100, what = 'condition' } = {},
) {
  const t0 = Date.now();
  for (;;) {
    if (await fn()) return Date.now() - t0;
    if (Date.now() - t0 > timeoutMs) throw new Error(`等待超时 ${timeoutMs}ms: ${what}`);
    await win.waitForTimeout(intervalMs);
  }
}

/** 等某个按钮变成可点击（disabled=false）。 */
export async function waitEnabled(win, locator, { timeoutMs = 30_000, what = 'button' } = {}) {
  return waitUntil(win, async () => locator.isEnabled().catch(() => false), {
    timeoutMs,
    what: `${what} enabled`,
  });
}

/** 从 job-percent 读当前百分比；不在进度态时返回 null。 */
export async function readPercent(win) {
  const el = win.getByTestId('job-percent').first();
  if (!(await el.isVisible().catch(() => false))) return null;
  const txt = await el.textContent().catch(() => null);
  const n = Number.parseInt((txt ?? '').replace('%', '').trim(), 10);
  return Number.isNaN(n) ? null : n;
}

/** 轮询进度并记录 30/60/90 阈值；返回是否等到完成。 */
export async function trackProgress(win, tl, { timeoutMs = 120_000 } = {}) {
  const marks = new Map();
  const t0 = Date.now();
  for (;;) {
    const pct = await readPercent(win);
    if (pct !== null) {
      for (const th of [10, 30, 60, 90]) {
        if (pct >= th && !marks.has(th)) {
          marks.set(th, pct);
          tl.mark(`state:progress-${th}`, { percent: pct });
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
      return { done: true, marks };
    if (
      await win
        .getByTestId('job-failed')
        .first()
        .isVisible()
        .catch(() => false)
    )
      return { done: false, failed: true, marks };
    if (Date.now() - t0 > timeoutMs)
      throw new Error(`进度轮询超时 ${timeoutMs}ms（最后百分比 ${pct ?? '未知'}）`);
    await win.waitForTimeout(100);
  }
}
