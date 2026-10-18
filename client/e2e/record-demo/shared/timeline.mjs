/**
 * 分镜时间轴：把"成片坐标的秒"映射到录制脚本里的墙钟等待。
 *
 * 坐标约定：
 *   - 计划时刻（tl.at / holdUntil 的入参）= **成片坐标**（= 分镜表/字幕时间轴，
 *     = 录完裁掉开头启动段之后的坐标）；
 *   - 录制视频里的原始坐标 = 成片坐标 + kMs（视频从页面创建就开始录，比脚本时间轴早
 *     约 0.5s；kMs 由彩排第一遍的"对齐色块"按帧实测，见 calibrate-flash.mjs）。
 * 所以事件日志同时记 videoMs（原始视频坐标）与 contentMs（成片坐标）。
 */
export class Timeline {
  /**
   * @param {import('@playwright/test').Page} win
   * @param {{kMs?: number, log?: (msg: string) => void}} [opts]
   */
  constructor(win, { kMs = 500, log = () => {} } = {}) {
    this.win = win;
    this.kMs = kMs;
    this.log = log;
    this.t0 = null;
    this.events = [];
  }

  start() {
    this.t0 = Date.now();
    this.log(`[tl] timeline start (kMs=${this.kMs})`);
  }

  /** 墙钟（相对时间轴起点）= 成片坐标。 */
  relMs() {
    if (this.t0 === null) throw new Error('Timeline 未 start()');
    return Date.now() - this.t0;
  }

  /** 当前时刻的原始视频坐标（毫秒）。 */
  videoMs() {
    return this.relMs() + this.kMs;
  }

  /** 当前时刻的成片坐标（毫秒）。 */
  contentMs() {
    return this.relMs();
  }

  /** 记一个事件（墙钟/视频/成片三种坐标）。 */
  mark(name, data = {}) {
    const wallMs = this.relMs();
    const e = { name, wallMs, videoMs: wallMs + this.kMs, contentMs: wallMs, ...data };
    this.events.push(e);
    this.log(
      `[tl] ${name} content=+${(wallMs / 1000).toFixed(2)}s video=+${((wallMs + this.kMs) / 1000).toFixed(2)}s`,
    );
    return e;
  }

  /** 等到成片坐标 tSec 处开始动作。 */
  async at(tSec, label) {
    const targetWall = tSec * 1000;
    const wait = targetWall - this.relMs();
    if (wait > 0) await this.win.waitForTimeout(wait);
    else if (wait < -350) this.log(`[tl] WARN step ${label} 晚 ${(-wait / 1000).toFixed(2)}s`);
    this.mark(`step:${label}`, { plannedContentSec: tSec });
  }

  /** 等到成片坐标 tSec 处（用于片尾停留）。 */
  async holdUntil(tSec, label = 'hold-end') {
    const targetWall = tSec * 1000;
    const wait = targetWall - this.relMs();
    if (wait > 0) await this.win.waitForTimeout(wait);
    this.mark(`hold:${label}`, { plannedContentSec: tSec });
  }

  /**
   * 等一个关键状态出现并记录时刻（waitForFn 由段脚本给）。
   * @param {string} name
   * @param {() => Promise<void>} waitForFn
   */
  async state(name, waitForFn, timeoutMs = 120_000) {
    const t = Date.now();
    await Promise.race([
      waitForFn(),
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error(`等待状态超时 ${timeoutMs}ms: ${name}`)), timeoutMs),
      ),
    ]);
    return this.mark(`state:${name}`, { waitMs: Date.now() - t });
  }
}
