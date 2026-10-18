/* global window, document, getComputedStyle -- evaluate 回调在页面内执行 */
/**
 * 录屏光标层（**本任务关键交付**）。
 *
 * 背景：Playwright/CDP 的 recordVideo 只抓页面合成帧，不含 OS/Chromium 光标
 * （PROBE-infra §B3 实测），所以正式片必须在页面里注入一个 DOM 光标层：
 *   - position:fixed; pointer-events:none; z-index 拉满，不影响任何交互；
 *   - SVG 白箭头 + 深色描边 + 阴影，黑底/白底界面都看得清；
 *   - 跟随 mousemove（Playwright page.mouse.move 派发的真实输入事件）；
 *   - mousedown 时箭头轻微缩小 + 白色涟漪，click 可见；
 *   - 尺寸按 CSS 像素设计，--force-device-scale-factor=2 下自动放大到物理像素
 *     （成片 2240×1440 里箭头约 56 物理像素，缩到 1080p 后约 42px，清晰可见）。
 *
 * 另外提供平滑移动/点击辅助：Playwright 的 mouse.move(steps) 是"瞬时"派发，
 * 录像里会表现为光标瞬移，所以按 40ms/步手动插值，0.5–0.8s 移到位。
 */

/** 注入页面（幂等）。window.__demoCursor 供 Node 侧查询/兜底设置位置。 */
export function cursorInitScript() {
  return `(() => {
    if (window.__demoCursor) return 'already';
    const style = document.createElement('style');
    style.textContent = [
      '@keyframes dsh-cursor-ripple { 0% { transform: scale(.35); opacity: .8 } 100% { transform: scale(1.8); opacity: 0 } }',
      '#dsh-demo-cursor { transition: none; }',
    ].join('\\n');
    document.head.appendChild(style);

    const wrap = document.createElement('div');
    wrap.id = 'dsh-demo-cursor';
    wrap.style.cssText = [
      'position:fixed', 'left:0', 'top:0', 'width:0', 'height:0',
      'pointer-events:none', 'z-index:2147483647', 'will-change:transform',
    ].join(';');

    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', '28');
    svg.setAttribute('height', '28');
    svg.setAttribute('viewBox', '0 0 28 28');
    svg.style.cssText = [
      'position:absolute', 'left:-3px', 'top:-2px',
      'filter:drop-shadow(0 1px 2px rgba(0,0,0,.9))',
      'transition:transform .07s ease-out', 'transform-origin:4px 3px',
    ].join(';');
    svg.innerHTML =
      '<path d="M4 2 L4 22.5 L9.8 17.3 L13.2 25 L17.2 23.2 L13.8 15.7 L21 15.2 Z" ' +
      'fill="#ffffff" stroke="#0b0f14" stroke-width="1.7" stroke-linejoin="round"/>';
    wrap.appendChild(svg);
    document.body.appendChild(wrap);

    const state = { x: Math.round(window.innerWidth * 0.62), y: Math.round(window.innerHeight * 0.72), down: false };
    const apply = () => {
      wrap.style.transform = 'translate(' + state.x + 'px,' + state.y + 'px)';
    };
    apply();

    const ripple = (x, y) => {
      const r = document.createElement('div');
      r.style.cssText = [
        'position:fixed', 'left:' + (x - 22) + 'px', 'top:' + (y - 22) + 'px',
        'width:44px', 'height:44px', 'border-radius:50%',
        'border:2.5px solid rgba(255,255,255,.9)',
        'box-shadow:0 0 0 1px rgba(0,0,0,.55), inset 0 0 6px rgba(0,0,0,.35)',
        'pointer-events:none', 'z-index:2147483646',
        'animation:dsh-cursor-ripple .42s ease-out forwards',
      ].join(';');
      document.body.appendChild(r);
      setTimeout(() => r.remove(), 460);
    };

    window.addEventListener('mousemove', (e) => {
      state.x = e.clientX;
      state.y = e.clientY;
      apply();
    }, true);
    window.addEventListener('mousedown', (e) => {
      state.down = true;
      svg.style.transform = 'scale(.86)';
      ripple(e.clientX, e.clientY);
    }, true);
    window.addEventListener('mouseup', () => {
      state.down = false;
      svg.style.transform = 'scale(1)';
    }, true);

    window.__demoCursor = {
      set: (x, y) => { state.x = x; state.y = y; apply(); },
      ripple: (x, y) => ripple(x, y),
      pos: () => ({ x: state.x, y: state.y }),
    };
    return 'ok';
  })()`;
}

/** 等待页面可注入（body 存在）。 */
export async function injectCursor(win) {
  await win.waitForFunction(() => document.body !== null, null, { timeout: 30_000 });
  const r = await win.evaluate(cursorInitScript());
  if (r !== 'ok' && r !== 'already') throw new Error(`光标层注入失败: ${r}`);
  // 初始位置：先放右下角空白处，避免和启动画面重叠
  await win.evaluate(() =>
    window.__demoCursor.set(window.innerWidth * 0.62, window.innerHeight * 0.72),
  );
}

/** 当前光标 CSS 坐标。 */
export async function cursorPos(win) {
  return win.evaluate(() => window.__demoCursor.pos());
}

/**
 * 平滑移动光标到 (x, y)，耗时 durationMs（默认 600ms，符合分镜 0.5–0.8s）。
 * 手动插值 + 每步等待：录像里能连续看到光标轨迹（mouse.move steps 是瞬时派发）。
 */
export async function moveTo(win, x, y, durationMs = 600) {
  const from = await cursorPos(win);
  const steps = Math.max(6, Math.round(durationMs / 40));
  const stepMs = durationMs / steps;
  for (let i = 1; i <= steps; i += 1) {
    const t = i / steps;
    // easeInOutQuad：起步/收尾慢，中段快，接近真人
    const e = t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2;
    const nx = from.x + (x - from.x) * e;
    const ny = from.y + (y - from.y) * e;
    await win.mouse.move(nx, ny);
    await win.waitForTimeout(stepMs);
  }
  await win.evaluate(([px, py]) => window.__demoCursor.set(px, py), [x, y]);
}

/** locator 中心的 CSS 坐标；元素不可见时抛错（早失败早定位）。 */
export async function centerOf(locator, label = 'target') {
  await locator.waitFor({ state: 'visible', timeout: 30_000 });
  const box = await locator.boundingBox();
  if (!box) throw new Error(`拿不到 ${label} 的 boundingBox`);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2, box };
}

/**
 * 移动 → 停 preMs → 点击坐标（真实 mousedown/up，触发点击涟漪）→ 停 postMs。
 * 用 page.mouse.click 而不是 locator.click：locator.click 会把光标瞬移到元素上，
 * 破坏我们录制的平滑轨迹；坐标点与元素中心一致，效果等价。
 */
export async function clickAt(win, x, y, { preMs = 400, postMs = 120 } = {}) {
  await win.waitForTimeout(preMs);
  await win.mouse.click(x, y);
  await win.waitForTimeout(postMs);
}

/** 移动并点击某个 locator（返回点击坐标，便于日志）。 */
export async function clickLocator(
  win,
  locator,
  { moveMs = 600, preMs = 400, postMs = 120, label } = {},
) {
  const c = await centerOf(locator, label);
  await moveTo(win, c.x, c.y, moveMs);
  await clickAt(win, c.x, c.y, { preMs, postMs });
  return { x: c.x, y: c.y };
}

/** 分段录制结束后自检：光标层是否还在、位置是否合理（防 React 重挂载把节点删掉）。 */
export async function assertCursorVisible(win) {
  return win.evaluate(() => {
    const el = document.getElementById('dsh-demo-cursor');
    if (!el) return { ok: false, reason: 'missing' };
    const svg = el.querySelector('svg');
    const st = getComputedStyle(svg);
    const pos = window.__demoCursor ? window.__demoCursor.pos() : null;
    return { ok: !!svg && st.visibility !== 'hidden' && st.opacity !== '0', pos };
  });
}
