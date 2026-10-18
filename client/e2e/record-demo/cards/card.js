/* global window, document, getComputedStyle, NodeFilter, requestAnimationFrame, cancelAnimationFrame -- 本文件在浏览器页内运行 */
/* DataBackup 演示视频 · 卡片渲染器（片头 / 终端 / 片尾）
 *
 * 纯展示层：内容与时间轴全部由 scenes.json 注入（scenes.json 又由 build-scenes.mjs
 * 从「容器内真实命令输出」生成），这里不写死任何业务数据，避免手写假输出。
 *
 * 用法（Node/Playwright 侧）：
 *   await page.evaluate(s => window.Card.mount(s), sceneSpec);
 *   await page.evaluate(() => window.Card.fontsReady());
 *   await page.evaluate(() => window.Card.startAt(Date.now()));   // 录制：按真实时钟播
 *   await page.evaluate(ms => window.Card.renderAt(ms), 4000);    // 彩排：直接跳到任意时刻
 *   const layout = await page.evaluate(() => window.Card.measure());
 *
 * 两种时钟都只走 renderAt(t)：录制时用 requestAnimationFrame + Date.now() 驱动，
 * 因此同一份 scenes.json 在彩排与正式录制里得到一致的画面与几何。
 */
(() => {
  'use strict';

  const stage = document.getElementById('stage');
  const SANS = "'Noto Sans CJK SC', 'Noto Sans CJK JP', sans-serif";
  const MONO = "'DejaVu Sans Mono', 'Noto Sans Mono CJK SC', monospace";

  const state = {
    spec: null,
    kind: null, // brand | terminal | mixed
    origin: 0,
    rafId: 0,
    nodes: {},
    lines: [], // [{atMs, node, cls}]
    phases: {},
    overlay: null,
    fade: null,
    timeline: null,
    cursor: null, // 终端卡常驻闪烁光标（也用作持续重绘源）
    ticker: null, // 2×2px 不可察觉重绘 ticker（防止 Chromium 静态时停止出帧）
  };

  const clamp01 = (v) => (v <= 0 ? 0 : v >= 1 ? 1 : v);
  const CURSOR_BLINK_MS = 265; // 530ms 方波
  const TICKER_MS = 500;

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function clear() {
    if (state.rafId) cancelAnimationFrame(state.rafId);
    stage.textContent = '';
    state.spec = null;
    state.kind = null;
    state.origin = 0;
    state.rafId = 0;
    state.nodes = {};
    state.lines = [];
    state.phases = {};
    state.overlay = null;
    state.fade = null;
    state.timeline = null;
    state.cursor = null;
    state.ticker = null;
    stage.classList.remove('measuring');
  }

  // ---------- 构建：片头 / 片尾品牌版式（左侧深绿竖条 + 文字组） ----------
  function buildBrand(spec) {
    const scene = el('div', `scene scene-${spec.variant}`);
    scene.dataset.role = 'scene';

    const bar = el('div', 'brand-bar');
    bar.dataset.role = 'brand-bar';
    scene.appendChild(bar);

    const group = el('div', 'brand-text');
    const main = el('div', 'brand-main', spec.mainText);
    main.dataset.role = 'brand-main';
    group.appendChild(main);
    state.nodes.main = main;

    if (spec.subText !== undefined) {
      const sub = el('div', 'brand-sub', spec.subText);
      sub.dataset.role = 'brand-sub';
      group.appendChild(sub);
      state.nodes.sub = sub;
    }
    if (spec.footText !== undefined) {
      const foot = el('div', 'brand-foot', spec.footText);
      foot.dataset.role = 'brand-foot';
      group.appendChild(foot);
      state.nodes.foot = foot;
    }
    scene.appendChild(group);
    state.nodes.textGroup = group;
    stage.appendChild(scene);
    state.kind = 'brand';
    state.fade = spec.fadeIn || null;
  }

  // ---------- 构建：终端卡片 ----------
  function buildTerminal(spec, parent) {
    const card = el('div', 'term-card');
    card.dataset.role = 'terminal-card';

    const bar = el('div', 'term-bar');
    for (let i = 0; i < 3; i += 1) bar.appendChild(el('span', 'term-dot'));
    bar.appendChild(el('span', 'term-title', 'demo@databackup: ~/demo'));
    card.appendChild(bar);

    const body = el('div', 'term-body');
    const lines = [];
    for (const line of spec.lines) {
      const node = el('div', `term-line ${line.cls}`);
      node.dataset.role = 'terminal-line';
      node.dataset.index = String(lines.length);
      node.dataset.cls = line.cls;
      node.dataset.atMs = String(line.atMs);
      if (line.cls === 'cmd') {
        node.appendChild(el('span', 'prompt', '$ '));
        node.appendChild(document.createTextNode(line.text));
      } else {
        node.textContent = line.text;
      }
      body.appendChild(node);
      lines.push({ atMs: line.atMs, node, cls: line.cls });
    }
    card.appendChild(body);
    parent.appendChild(card);
    // 终端常驻闪烁光标：放在已显示的最后一行下方（同时也是持续重绘源之一）
    const cursor = el('div', 'term-cursor');
    body.appendChild(cursor);
    state.cursor = cursor;
    return lines;
  }

  // ---------- 构建：片尾混合场景（终端 → 数据卡 → 谢谢 → 淡出） ----------
  function buildMixed(spec) {
    const scene = el('div', 'scene scene-verify');

    const terminalPhase = el('div', 'phase phase-terminal');
    state.lines = buildTerminal(spec.terminal, terminalPhase);
    state.phases.terminal = terminalPhase;
    scene.appendChild(terminalPhase);

    const statsPhase = el('div', 'phase phase-stats');
    statsPhase.hidden = true;
    state.phases.stats = statsPhase;
    const wrap = el('div', 'stats-wrap');
    spec.stats.items.forEach((item, index) => {
      if (index > 0) wrap.appendChild(el('div', 'stat-sep'));
      const stat = el('div', 'stat');
      const value = el('div', 'stat-value', item.value);
      value.dataset.role = 'stat-value';
      value.dataset.index = String(index);
      const label = el('div', 'stat-label', item.label);
      label.dataset.role = 'stat-label';
      label.dataset.index = String(index);
      stat.appendChild(value);
      stat.appendChild(label);
      wrap.appendChild(stat);
    });
    statsPhase.appendChild(wrap);
    scene.appendChild(statsPhase);

    const thanksPhase = el('div', 'phase phase-thanks scene-thanks');
    thanksPhase.hidden = true;
    state.phases.thanks = thanksPhase;
    const bar = el('div', 'brand-bar');
    bar.dataset.role = 'thanks-bar';
    thanksPhase.appendChild(bar);
    const group = el('div', 'brand-text');
    const main = el('div', 'brand-main', spec.thanks.main);
    main.dataset.role = 'thanks-main';
    const byline = el('div', 'brand-sub', spec.thanks.byline);
    byline.dataset.role = 'thanks-byline';
    group.appendChild(main);
    group.appendChild(byline);
    thanksPhase.appendChild(group);
    scene.appendChild(thanksPhase);

    const overlay = el('div', 'fade-overlay');
    overlay.dataset.role = 'fade-overlay';
    scene.appendChild(overlay);
    state.overlay = overlay;

    stage.appendChild(scene);
    state.kind = 'mixed';
    state.timeline = spec.timeline;
  }

  // ---------- 渲染：renderAt(tMs) 是唯一的画面状态入口 ----------
  // 不可察觉的持续重绘 ticker：Chromium 在画面完全静止时会停止产生合成帧，
  // 导致 Playwright recordVideo 的 webm 时长变短。2×2px 的 #000/#010101 交替足以
  // 强制合成器持续出帧，肉眼完全不可见。
  function updateTicker(t) {
    if (!state.ticker) return;
    state.ticker.style.backgroundColor =
      Math.floor(t / TICKER_MS) % 2 === 0 ? '#000000' : '#010101';
  }

  // 终端行显示 + 闪烁光标位置（光标在已显示的最后一行下方，530ms 方波）
  function updateTerminal(t) {
    let revealed = 0;
    for (const line of state.lines) {
      const show = t >= line.atMs;
      line.node.style.opacity = show ? '1' : '0';
      if (show) revealed = Math.max(revealed, Number(line.node.dataset.index) + 1);
    }
    if (state.cursor) {
      state.cursor.style.top = `${26 + 40 * revealed}px`;
      state.cursor.style.opacity = Math.floor(t / CURSOR_BLINK_MS) % 2 === 0 ? '1' : '0';
    }
  }

  function renderAt(t) {
    if (!state.spec) return;
    updateTicker(t);

    if (state.kind === 'brand') {
      const fade = state.fade;
      const opacity = fade ? clamp01((t - fade.startMs) / fade.durationMs) : 1;
      state.nodes.textGroup.style.opacity = String(opacity);
      return;
    }

    if (state.kind === 'terminal') {
      updateTerminal(t);
      return;
    }

    if (state.kind === 'mixed') {
      const tl = state.timeline;
      state.phases.terminal.hidden = !(t < tl.terminalEndMs);
      state.phases.stats.hidden = !(t >= tl.statsStartMs && t < tl.statsEndMs);
      state.phases.thanks.hidden = !(t >= tl.thanksStartMs && t < tl.fadeEndMs);
      updateTerminal(t);
      const fadeProgress = clamp01(
        (t - tl.fadeStartMs) / Math.max(1, tl.fadeEndMs - tl.fadeStartMs),
      );
      state.overlay.style.opacity = String(fadeProgress);
    }
  }

  // ---------- 测量：给出每个可见元素的视口坐标/字号（彩排 JSON 与 card-layout.json 的来源） ----------
  function rectOf(node) {
    const r = node.getBoundingClientRect();
    const cs = getComputedStyle(node);
    const round = (v) => Math.round(v * 100) / 100;
    return {
      x: round(r.x),
      y: round(r.y),
      width: round(r.width),
      height: round(r.height),
      fontSize: parseFloat(cs.fontSize),
      fontFamily: cs.fontFamily,
      color: cs.color,
    };
  }

  function fontReport() {
    const declared = { sans: SANS, mono: MONO };
    const available = {};
    for (const name of ['Noto Sans CJK SC', 'DejaVu Sans Mono', 'Noto Sans Mono CJK SC']) {
      try {
        available[name] = document.fonts.check(`28px "${name}"`);
      } catch (err) {
        available[name] = `error: ${err.message}`;
      }
    }
    return { declared, available };
  }

  function unionBox(boxes) {
    if (!boxes.length) return null;
    const round = (v) => Math.round(v * 100) / 100;
    const x1 = Math.min(...boxes.map((b) => b.x));
    const y1 = Math.min(...boxes.map((b) => b.y));
    const x2 = Math.max(...boxes.map((b) => b.x + b.width));
    const y2 = Math.max(...boxes.map((b) => b.y + b.height));
    return { x: round(x1), y: round(y1), width: round(x2 - x1), height: round(y2 - y1) };
  }

  // 在节点内定位 match 子串，返回其像素包围盒（用于注解框的行内 x 范围）
  function matchRangeRect(node, match) {
    const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
    const parts = [];
    let full = '';
    let current = walker.nextNode();
    while (current) {
      parts.push({ node: current, start: full.length });
      full += current.nodeValue || '';
      current = walker.nextNode();
    }
    const index = full.indexOf(match);
    if (index < 0) return null;
    const end = index + match.length;
    let startPoint = null;
    let endPoint = null;
    for (const part of parts) {
      const partStart = part.start;
      const partEnd = partStart + (part.node.nodeValue || '').length;
      if (!startPoint && index >= partStart && index < partEnd) {
        startPoint = { node: part.node, offset: index - partStart };
      }
      if (!endPoint && end > partStart && end <= partEnd) {
        endPoint = { node: part.node, offset: end - partStart };
      }
    }
    if (!startPoint || !endPoint) return null;
    const range = document.createRange();
    range.setStart(startPoint.node, startPoint.offset);
    range.setEnd(endPoint.node, endPoint.offset);
    const r = range.getBoundingClientRect();
    const round = (v) => Math.round(v * 100) / 100;
    return { x: round(r.x), y: round(r.y), width: round(r.width), height: round(r.height) };
  }

  // 注解目标框：P3 用 ASS 叠加「注解框/标签」时直接消费这里的坐标（1920×1080 画布像素）
  function measureTargets() {
    const spec = state.spec;
    if (!spec || !Array.isArray(spec.targets) || !spec.targets.length) return [];
    const lineNodes = state.lines.map((line) => line.node);
    const out = [];
    for (const target of spec.targets) {
      const indices = target.lineIndices || [];
      const nodes = indices.map((i) => lineNodes[i]).filter(Boolean);
      if (!nodes.length) {
        out.push({
          name: target.name,
          annotationWindowMs: target.annotationWindowMs || null,
          visibleWindowMs: target.visibleWindowMs || null,
          note: target.note || null,
          missing: 'line-indices',
        });
        continue;
      }
      const lineBoxes = nodes.map((node) => {
        const rect = rectOf(node);
        return {
          index: Number(node.dataset.index),
          cls: node.dataset.cls,
          text: (node.textContent || '').trim(),
          appearMs: Number(node.dataset.atMs),
          x: rect.x,
          y: rect.y,
          width: rect.width,
          height: rect.height,
          fontSize: rect.fontSize,
        };
      });
      const entry = {
        name: target.name,
        annotationWindowMs: target.annotationWindowMs || null,
        visibleWindowMs: target.visibleWindowMs || null,
        note: target.note || null,
        lineIndices: indices,
        lineBoxes,
        box: unionBox(lineBoxes),
      };
      if (target.match) {
        const withMatch = nodes.find((node) => (node.textContent || '').includes(target.match));
        const matchBox = withMatch ? matchRangeRect(withMatch, target.match) : null;
        if (matchBox) {
          entry.match = target.match;
          entry.matchLineIndex = Number(withMatch.dataset.index);
          entry.matchBox = matchBox;
        }
      }
      out.push(entry);
    }
    return out;
  }

  function measure() {
    if (!state.spec) return null;
    // 展开被隐藏的阶段，才能量到数据卡/片尾元素（仅测量用）
    stage.classList.add('measuring');
    const elements = [];
    let targets;
    try {
      for (const node of stage.querySelectorAll('[data-role]')) {
        const role = node.dataset.role;
        if (role === 'scene') continue;
        const info = { role, text: (node.textContent || '').trim() };
        Object.assign(info, rectOf(node));
        if (node.dataset.atMs !== undefined) info.atMs = Number(node.dataset.atMs);
        if (node.dataset.index !== undefined) info.index = Number(node.dataset.index);
        if (node.dataset.cls !== undefined) info.cls = node.dataset.cls;
        elements.push(info);
      }
      targets = measureTargets();
    } finally {
      stage.classList.remove('measuring');
    }
    const result = {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      deviceScaleRatio: window.devicePixelRatio,
      fonts: fontReport(),
      elements,
    };
    if (targets.length) result.targets = targets;
    return result;
  }

  // ---------- 对外 API ----------
  window.Card = {
    ready: true,

    mount(spec) {
      clear();
      state.spec = spec;
      // 持续重绘 ticker：2×2px、右下角，颜色在 #000000 / #010101 间切换，肉眼不可见
      const ticker = el('div', 'ticker');
      stage.appendChild(ticker);
      state.ticker = ticker;
      if (spec.kind === 'title') {
        buildBrand({
          variant: 'title',
          mainText: spec.texts.main,
          subText: spec.texts.sub,
          footText: spec.texts.foot,
          fadeIn: spec.fadeIn,
        });
      } else if (spec.kind === 'terminal') {
        const scene = el('div', 'scene scene-terminal');
        scene.dataset.role = 'scene';
        state.lines = buildTerminal(spec, scene);
        stage.appendChild(scene);
        state.kind = 'terminal';
      } else if (spec.kind === 'mixed') {
        buildMixed(spec);
      } else {
        throw new Error(`unknown scene kind: ${spec.kind}`);
      }
      renderAt(0);
      return true;
    },

    renderAt(ms) {
      renderAt(ms);
      return true;
    },

    startAt(originEpochMs) {
      state.origin = originEpochMs || Date.now();
      const loop = () => {
        renderAt(Date.now() - state.origin);
        state.rafId = requestAnimationFrame(loop);
      };
      loop();
    },

    elapsed() {
      return state.origin ? Date.now() - state.origin : 0;
    },

    async fontsReady() {
      try {
        await Promise.allSettled([
          document.fonts.load('700 150px "Noto Sans CJK SC"'),
          document.fonts.load('400 28px "DejaVu Sans Mono"'),
          document.fonts.load('400 28px "Noto Sans Mono CJK SC"'),
        ]);
        await document.fonts.ready;
      } catch {
        // 量不到字体也不阻断渲染；由 frames 目检兜底
      }
      return fontReport();
    },

    measure,
  };
})();
