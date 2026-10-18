#!/usr/bin/env node
/* global window -- evaluate 回调在页面内执行 */
/* DataBackup 演示视频 · 卡片段彩排 / 正式录制 / 链路探针
 *
 * 用法（databackup-dev 容器内，见 run-in-docker.sh）：
 *   node render-scenes.mjs --mode rehearse    # 彩排：几何 JSON + 每个时刻抽帧 PNG（不录视频）
 *   node render-scenes.mjs --mode record      # 正式录制：Playwright recordVideo 1920×1080@30fps webm
 *   node render-scenes.mjs --mode probe       # 链路探针：用片头卡录 4s，验证 webm 尺寸/帧率/中文
 *   可选：--only seg-01-data   只处理某一段
 *
 * 渲染时钟：录制时页面用 requestAnimationFrame + Date.now() 驱动 Card.renderAt(t)，
 * 彩排时直接 Card.renderAt(tMs) 跳到任意时刻截图/测量——两者画面完全一致。
 * 硬前提（PROBE-infra.md §B0）：PLAYWRIGHT_BROWSERS_PATH 必须指向
 * course/04-视频/tools/ms-playwright，否则 recordVideo 依赖的 Playwright ffmpeg 找不到、页面挂死。
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../../../..');
const WORK = path.join(REPO, 'course/04-视频/work');
const CARDS = path.join(WORK, 'cards');
const SCENES_PATH = path.join(CARDS, 'scenes.json');
const CARD_URL = pathToFileURL(path.join(HERE, 'card.html')).href;
const VIEWPORT = { width: 1920, height: 1080 };
const FPS = 30;

const argv = process.argv.slice(2);
function arg(name, fallback) {
  const i = argv.indexOf(`--${name}`);
  return i >= 0 && argv[i + 1] && !argv[i + 1].startsWith('--') ? argv[i + 1] : fallback;
}
const MODE = arg('mode', 'rehearse');
const ONLY = arg('only', null);
const HEADED = process.env.CARD_HEADED === '1';
const PAD_MS = Number(process.env.CARD_RECORD_PAD_MS || '0');
// 录制时长标定（容器内实测 3 次）：videoDuration ≈ (tClose - tCtx) + 1220ms，
// 其中 tCtx 是 newContext 前时刻、tClose 是开始 close 的时刻；1220ms 是 Playwright 录像器
// 时间轴相对我们时钟的固定偏移（≈1.2s）。录制时用 (目标时长 - loadMs - offset) 作为等待时长，
// 使成片时长 ≈ recordSec（目标 +1s 余量），已在 probe 中验证（6s/10s 两次，误差 ≤1 帧）。
const RECORD_OVERHEAD_MS = Number(process.env.CARD_RECORD_OVERHEAD_MS || '1220');

const LAUNCH_ARGS = [
  '--no-sandbox',
  '--disable-dev-shm-usage',
  '--default-background-color=00000000',
  '--font-render-hinting=none',
];

const log = (...args) => console.log('[cards]', ...args);
const rel = (file) => path.relative(REPO, file).split(path.sep).join('/');

function writeJson(file, value) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}
function fail(message) {
  console.error(`[cards] FATAL: ${message}`);
  process.exit(1);
}
function loadScenes() {
  if (!fs.existsSync(SCENES_PATH)) fail(`缺少 ${rel(SCENES_PATH)}（先跑 build-scenes.mjs）`);
  return JSON.parse(fs.readFileSync(SCENES_PATH, 'utf8'));
}
function selectedSegments(scenes) {
  const wanted = ONLY
    ? ONLY.split(',')
        .map((name) => name.trim())
        .filter(Boolean)
    : null;
  const list = wanted
    ? scenes.segments.filter((spec) => wanted.includes(spec.name))
    : scenes.segments;
  if (!list.length) fail(`没有匹配的段落（--only ${ONLY}）`);
  if (wanted && list.length !== wanted.length) {
    const missing = wanted.filter((name) => !scenes.segments.some((spec) => spec.name === name));
    fail(`--only 里有不存在的段落: ${missing.join(', ')}`);
  }
  return list;
}

async function launchBrowser() {
  // 动态 import：让本脚本在没有 playwright-core 的环境（如宿主）也能跑 --mode webm-duration 自检
  const { chromium } = await import('playwright-core');
  return chromium.launch({ headless: !HEADED, args: LAUNCH_ARGS });
}

async function preparePage(context, spec, { blackPreload = false } = {}) {
  const page = await context.newPage();
  if (blackPreload) {
    // 录制的首帧之前是 about:blank；先铺黑底，避免 webm 开头闪白（P3 裁头更省事）
    await page.setContent(
      '<!doctype html><html><head><style>html,body{margin:0;background:#000}</style></head><body></body></html>',
    );
  }
  await page.goto(CARD_URL, { waitUntil: 'load' });
  await page.waitForFunction(() => window.Card && window.Card.ready === true);
  await page.evaluate((sceneSpec) => window.Card.mount(sceneSpec), spec);
  const fonts = await page.evaluate(() => window.Card.fontsReady());
  return { page, fonts };
}

// ---------------------------------------------------------------------------
// 几何检查
// ---------------------------------------------------------------------------
function checkScene(spec, measured) {
  const problems = [];
  const viewport = VIEWPORT;
  for (const e of measured.elements) {
    if (e.role === 'fade-overlay') continue;
    if (
      e.x < -0.5 ||
      e.y < -0.5 ||
      e.x + e.width > viewport.width + 0.5 ||
      e.y + e.height > viewport.height + 0.5
    ) {
      problems.push(
        `元素越界 ${e.role}${e.index !== undefined ? `#${e.index}` : ''} (${e.x},${e.y},${e.width},${e.height})`,
      );
    }
  }
  const card = measured.elements.find((e) => e.role === 'terminal-card');
  if (card) {
    if (Math.abs(card.width - 1536) > 1 || Math.abs(card.height - 864) > 1) {
      problems.push(`终端卡不是 1536×864（画面 80%）: ${card.width}×${card.height}`);
    }
    const innerRight = card.x + card.width - 40;
    for (const e of measured.elements.filter((item) => item.role === 'terminal-line')) {
      if (e.x + e.width > innerRight + 0.5) {
        problems.push(
          `终端行 #${e.index} 超出卡片内宽: right=${(e.x + e.width).toFixed(1)} > ${innerRight}`,
        );
      }
      if (Math.abs(e.fontSize - 28) > 0.1)
        problems.push(`终端行 #${e.index} 字号异常: ${e.fontSize}`);
    }
  }
  const lineElements = measured.elements.filter((e) => e.role === 'terminal-line');
  const lastAppearMs = lineElements.length ? Math.max(...lineElements.map((e) => e.atMs)) : 0;
  if (lineElements.length && lastAppearMs > spec.targetSec * 1000 - 500) {
    problems.push(`最后一行出现 ${lastAppearMs}ms，距镜头结束不足 0.5s`);
  }
  if (spec.kind === 'title') {
    const fadeEnd = spec.fadeIn.startMs + spec.fadeIn.durationMs;
    if (fadeEnd > 1000) problems.push(`片头淡入结束于 ${fadeEnd}ms，应在 1s 内`);
  }
  if (spec.kind === 'mixed') {
    const tl = spec.timeline;
    if (tl.terminalStartMs !== 0) problems.push('timeline.terminalStartMs 应为 0');
    if (tl.terminalEndMs !== tl.statsStartMs) problems.push('终端段结束与数据卡开始不连续');
    if (tl.statsEndMs !== tl.thanksStartMs) problems.push('数据卡结束与片尾开始不连续');
    if (tl.thanksEndMs > tl.fadeStartMs) problems.push('片尾停留结束后才开始淡出（顺序错误）');
    if (tl.fadeEndMs > spec.targetSec * 1000) problems.push('淡出结束超过镜头时长');
  }
  const targetProblems = [];
  for (const t of measured.targets || []) {
    if (!t.box) targetProblems.push(`注解目标 ${t.name} 没量到包围盒（${t.missing || 'unknown'}）`);
    else if (t.match && !t.matchBox)
      targetProblems.push(`注解目标 ${t.name} 没量到子串框（${t.match}）`);
  }
  problems.push(...targetProblems);
  return {
    ok: problems.length === 0,
    problems,
    lastAppearMs,
    terminalCard: card ? { x: card.x, y: card.y, w: card.width, h: card.height } : null,
  };
}

// ---------------------------------------------------------------------------
// 几何 → card-layout.json（给 P3 生成 ASS 字幕/注解框用）
// ---------------------------------------------------------------------------
function describeElement(e) {
  if (!e) return null;
  return {
    text: e.text,
    x: e.x,
    y: e.y,
    w: e.width,
    h: e.height,
    fontSize: e.fontSize,
    fontFamily: e.fontFamily,
    color: e.color,
  };
}

function buildLayoutEntry(spec, measured, extra = {}) {
  const find = (role, index) =>
    measured.elements.find((e) => e.role === role && (index === undefined || e.index === index));
  const toBox = (e) => (e ? { x: e.x, y: e.y, w: e.width, h: e.height } : null);
  const terminalLines = measured.elements
    .filter((e) => e.role === 'terminal-line')
    .sort((a, b) => a.index - b.index)
    .map((e) => ({
      index: e.index,
      cls: e.cls,
      text: e.text,
      appearMs: e.atMs,
      x: e.x,
      y: e.y,
      w: e.width,
      h: e.height,
      fontSize: e.fontSize,
    }));

  const targets = {};
  for (const t of measured.targets || []) {
    if (!t.box) {
      targets[t.name] = { missing: t.missing || 'not-measured' };
      continue;
    }
    targets[t.name] = {
      x: t.box.x,
      y: t.box.y,
      w: t.box.width,
      h: t.box.height,
      annotationWindowMs: t.annotationWindowMs,
      visibleWindowMs: t.visibleWindowMs || null,
      note: t.note || null,
      lineIndices: t.lineIndices,
      lineBoxes: (t.lineBoxes || []).map((l) => ({
        index: l.index,
        cls: l.cls,
        text: l.text,
        appearMs: l.appearMs,
        x: l.x,
        y: l.y,
        w: l.width,
        h: l.height,
      })),
    };
    if (t.matchBox) {
      targets[t.name].matchBox = {
        text: t.match,
        lineIndex: t.matchLineIndex,
        x: t.matchBox.x,
        y: t.matchBox.y,
        w: t.matchBox.width,
        h: t.matchBox.height,
      };
    }
  }

  const base = { targetSec: spec.targetSec, recordSec: spec.recordSec, ...extra };

  if (spec.kind === 'title') {
    return {
      ...base,
      kind: 'title',
      fadeIn: spec.fadeIn,
      elements: {
        bar: describeElement(find('brand-bar')),
        main: describeElement(find('brand-main')),
        sub: describeElement(find('brand-sub')),
        foot: describeElement(find('brand-foot')),
      },
      annotationTargets: {},
    };
  }

  if (spec.kind === 'terminal') {
    return {
      ...base,
      kind: 'terminal',
      terminalCard: toBox(find('terminal-card')),
      lines: terminalLines,
      targets,
    };
  }

  const stats = spec.stats.items.map((item, index) => ({
    value: item.value,
    label: item.label,
    valueLayout: describeElement(find('stat-value', index)),
    labelLayout: describeElement(find('stat-label', index)),
  }));
  return {
    ...base,
    kind: 'mixed',
    timeline: spec.timeline,
    terminal: { card: toBox(find('terminal-card')), lines: terminalLines },
    targets,
    stats: { items: stats },
    thanks: {
      bar: describeElement(find('thanks-bar')),
      main: describeElement(find('thanks-main')),
      byline: describeElement(find('thanks-byline')),
    },
    fade: { startMs: spec.timeline.fadeStartMs, endMs: spec.timeline.fadeEndMs },
  };
}

function baseLayout(scenes) {
  return {
    version: 1,
    generatedAt: new Date().toISOString(),
    generator: 'client/e2e/record-demo/cards/render-scenes.mjs',
    video: { width: 1920, height: 1080, fps: FPS },
    fonts: scenes.fonts,
    safeArea: {
      x: 80,
      y: 0,
      w: 1760,
      h: 960,
      note: '07-…§6：左右各 80px、底部留 120px（避开字幕与播放器控件）',
    },
    seed: scenes.seed,
    provenance: scenes.provenance,
    notes: [
      '终端每行 y/h 是行盒（含行高 40px），appearMs 是该行出现时刻；文字为等宽字体 DejaVu Sans Mono，中文回退 Noto Sans Mono CJK SC。',
      'targets.* 为 P3 注解框建议位置（1920×1080 画布像素）：box 为整行/整块包围盒，matchBox 为行内目标子串的精确包围盒。',
      'annotationWindowMs 来自 docs/deliverables/video-storyboard.json 的 annotations 时间轴（相对本镜）。',
      '卡片只画内容像素；字幕/注解字幕条由 P3 用 ASS 叠加，不在卡片里。',
      '录制时动画时钟从页面就绪时刻开始，webm 第一帧即 scene t=0（contentStartSec=0）；录制尾部多出的 ~1s 为裁切余量。',
    ],
  };
}

// ---------------------------------------------------------------------------
// 彩排
// ---------------------------------------------------------------------------
const REHEARSE_PROBES = {
  'seg-00-title': [0, 250, 550, 700, 4000, 9000],
  'seg-01-data': [0, 2000, 5000, 9000, 11000, 13000, 19000, 20000],
  'seg-06-verify': [500, 4400, 7900, 9000, 13000, 19000, 23000, 24000],
};

async function rehearse() {
  const scenes = loadScenes();
  const segments = selectedSegments(scenes);
  const outDir = path.join(WORK, 'rehearsal/cards');
  const frameDir = path.join(outDir, 'frames');
  fs.mkdirSync(frameDir, { recursive: true });
  const layout = baseLayout(scenes);
  const summary = [];
  const browser = await launchBrowser();
  try {
    for (const spec of segments) {
      const context = await browser.newContext({
        viewport: VIEWPORT,
        deviceScaleFactor: 1,
        colorScheme: 'dark',
        locale: 'zh-CN',
      });
      const { page, fonts } = await preparePage(context, spec);
      const probes = REHEARSE_PROBES[spec.name] || [0, Math.round(spec.targetSec * 500)];
      for (const t of probes) {
        await page.evaluate((ms) => window.Card.renderAt(ms), t);
        await page.waitForTimeout(30);
        const framePath = path.join(frameDir, `${spec.name}-t${String(t).padStart(5, '0')}ms.png`);
        await page.screenshot({ path: framePath });
        log(`frame ${rel(framePath)}`);
      }
      const measured = await page.evaluate(() => window.Card.measure());
      const checks = checkScene(spec, measured);
      const segmentJson = {
        segment: spec.name,
        kind: spec.kind,
        targetSec: spec.targetSec,
        recordSec: spec.recordSec,
        rehearsedAt: new Date().toISOString(),
        browser: { headed: HEADED, viewport: VIEWPORT, deviceScaleFactor: 1 },
        fonts,
        probesMs: probes,
        checks,
        timing: measured.elements
          .filter((e) => e.role !== 'fade-overlay')
          .map((e) => ({
            role: e.role,
            index: e.index,
            cls: e.cls,
            text: e.text,
            appearMs: e.atMs,
            x: e.x,
            y: e.y,
            w: e.width,
            h: e.height,
            fontSize: e.fontSize,
            fontFamily: e.fontFamily,
          })),
        targets: measured.targets || [],
      };
      writeJson(path.join(outDir, `${spec.name}.json`), segmentJson);
      layout[spec.name] = buildLayoutEntry(spec, measured, { layoutSource: 'rehearsal' });
      summary.push({
        name: spec.name,
        kind: spec.kind,
        targetSec: spec.targetSec,
        recordSec: spec.recordSec,
        checks,
        lines: measured.elements.filter((e) => e.role === 'terminal-line').length,
        targets: (measured.targets || []).map((t) => t.name),
        frames: probes.length,
      });
      log(
        `${spec.name}: checks.ok=${checks.ok}${checks.ok ? '' : ` problems=${JSON.stringify(checks.problems)}`} — ${rel(path.join(outDir, `${spec.name}.json`))}`,
      );
      await context.close();
    }
  } finally {
    await browser.close();
  }
  writeJson(path.join(CARDS, 'card-layout.json'), layout);
  writeJson(path.join(outDir, 'summary.json'), {
    generatedAt: new Date().toISOString(),
    mode: 'rehearse',
    browser: { headed: HEADED, viewport: VIEWPORT, deviceScaleFactor: 1 },
    segments: summary,
  });
  const bad = summary.filter((s) => !s.checks.ok);
  log(`rehearse done: ${summary.length} segments, ${bad.length} with problems`);
  log(`layout -> ${rel(path.join(CARDS, 'card-layout.json'))}`);
  if (bad.length) process.exitCode = 2;
}

// ---------------------------------------------------------------------------
// 极简 EBML 解析：只读 webm 的 Segment/Info/Duration（毫秒）。
// 容器里没有 ffprobe，录制后用它在容器内自检时长；不够长就自动加长等待重录。
// ---------------------------------------------------------------------------
function readVint(buf, pos, forId) {
  const first = buf[pos];
  if (first === undefined) return null;
  let mask = 0x80;
  let length = 1;
  while (length <= 8 && (first & mask) === 0) {
    mask >>= 1;
    length += 1;
  }
  if (length > 8) return null;
  let value = forId ? first : first & (mask - 1);
  let unknown = !forId && (first & (mask - 1)) === mask - 1;
  for (let i = 1; i < length; i += 1) {
    const byte = buf[pos + i];
    if (byte === undefined) return null;
    value = value * 256 + byte;
    if (!forId && byte !== 0xff) unknown = false;
  }
  return { length, value, unknown };
}

function readElementHeader(buf, pos) {
  const id = readVint(buf, pos, true);
  if (!id) return null;
  const size = readVint(buf, pos + id.length, false);
  if (!size) return null;
  const dataStart = pos + id.length + size.length;
  return { id: id.value, size: size.unknown ? null : size.value, dataStart };
}

function readWebmDurationMs(file) {
  let buf;
  try {
    buf = fs.readFileSync(file);
  } catch {
    return null;
  }
  // 顶层找 Segment（0x18538067）
  let pos = 0;
  let foundSegment = false;
  while (pos < buf.length) {
    const el = readElementHeader(buf, pos);
    if (!el) return null;
    if (el.id === 0x18538067) {
      pos = el.dataStart;
      foundSegment = true;
      break;
    }
    if (el.size === null) return null;
    pos = el.dataStart + el.size;
  }
  if (!foundSegment) return null;
  // Segment 内找 Info（0x1549A966）
  let foundInfo = false;
  while (pos < buf.length) {
    const el = readElementHeader(buf, pos);
    if (!el) return null;
    if (el.id === 0x1549a966) {
      pos = el.dataStart;
      foundInfo = true;
      break;
    }
    if (el.size === null) return null;
    pos = el.dataStart + el.size;
  }
  if (!foundInfo) return null;
  // Info 内找 Duration（0x4489，float；TimecodeScale 默认 1ms）
  while (pos < buf.length) {
    const el = readElementHeader(buf, pos);
    if (!el || el.size === null) return null;
    if (el.id === 0x4489) {
      if (el.size === 8) return Math.round(buf.readDoubleBE(el.dataStart));
      if (el.size === 4) return Math.round(buf.readFloatBE(el.dataStart));
      return null;
    }
    pos = el.dataStart + el.size;
  }
  return null;
}

// ---------------------------------------------------------------------------
// 正式录制（Playwright recordVideo → webm，无音轨）
// ---------------------------------------------------------------------------
async function record() {
  const scenes = loadScenes();
  const segments = selectedSegments(scenes);
  const segDir = path.join(WORK, 'seg');
  const tempRoot = path.join(segDir, 'raw');
  fs.mkdirSync(tempRoot, { recursive: true });
  const layoutPath = path.join(CARDS, 'card-layout.json');
  const layout = fs.existsSync(layoutPath)
    ? JSON.parse(fs.readFileSync(layoutPath, 'utf8'))
    : baseLayout(scenes);
  layout.generatedAt = new Date().toISOString();
  const meta = {
    generatedAt: new Date().toISOString(),
    mode: 'record',
    headed: HEADED,
    browser: 'playwright-core chromium',
    viewport: VIEWPORT,
    fps: FPS,
    recordVideo: { size: VIEWPORT, fps: FPS },
    padMs: PAD_MS,
    segments: [],
  };
  const browser = await launchBrowser();
  try {
    for (const spec of segments) {
      const desiredMs = spec.recordSec * 1000; // 目标：分镜时长 +1s
      const minDurationMs = spec.targetSec * 1000 + 700; // 至少覆盖到镜头结束 +0.7s，P3 才有得裁
      const maxDurationMs = desiredMs + 500;
      let padGuess = PAD_MS;
      let recorded = null;
      for (let attempt = 1; attempt <= 5 && !recorded; attempt += 1) {
        const tempDir = path.join(tempRoot, spec.name);
        fs.rmSync(tempDir, { recursive: true, force: true });
        fs.mkdirSync(tempDir, { recursive: true });
        const tCtx = Date.now();
        const context = await browser.newContext({
          viewport: VIEWPORT,
          deviceScaleFactor: 1,
          colorScheme: 'dark',
          locale: 'zh-CN',
          recordVideo: { dir: tempDir, size: VIEWPORT, fps: FPS },
        });
        try {
          const { page, fonts } = await preparePage(context, spec, { blackPreload: true });
          // 录制前复测几何，保证与彩排一致（P3 的 ASS 坐标不会漂）
          const measured = await page.evaluate(() => window.Card.measure());
          const checks = checkScene(spec, measured);
          if (!checks.ok)
            log(`${spec.name}: WARN 几何/时间轴问题 ${JSON.stringify(checks.problems)}`);
          // 动画时钟从「页面已就绪」开始，保证 webm 的第一帧就是 scene t=0
          const tAnim = Date.now();
          await page.evaluate((originEpochMs) => window.Card.startAt(originEpochMs), tAnim);
          const wallTarget = Math.max(
            1000,
            spec.recordSec * 1000 - (tAnim - tCtx) - RECORD_OVERHEAD_MS + padGuess,
          );
          for (;;) {
            const remain = wallTarget - (Date.now() - tAnim);
            if (remain <= 0) break;
            await page.waitForTimeout(Math.min(remain, 250));
          }
          const wallMs = Date.now() - tAnim;
          const tClose = Date.now();
          const video = page.video();
          await context.close();
          let tempVideo = null;
          try {
            tempVideo = await video.path();
          } catch {
            // 兜底：Playwright 未返回路径时在临时目录里找 webm
            const files = fs.readdirSync(tempDir).filter((file) => file.endsWith('.webm'));
            if (files.length) tempVideo = path.join(tempDir, files[0]);
          }
          if (!tempVideo || !fs.existsSync(tempVideo))
            fail(`${spec.name}: 没有拿到 webm（recordVideo 失败）`);
          const finalPath = path.join(segDir, `${spec.name}.webm`);
          fs.renameSync(tempVideo, finalPath);
          const stat = fs.statSync(finalPath);
          const durationMs = readWebmDurationMs(finalPath);
          const inRange =
            durationMs !== null && durationMs >= minDurationMs && durationMs <= maxDurationMs;
          if (durationMs === null) log(`${spec.name}: 无法解析 webm Duration（按 wallMs 兜底）`);
          if (inRange || durationMs === null) {
            recorded = {
              finalPath,
              stat,
              measured,
              checks,
              fonts,
              wallMs,
              tCtx,
              tAnim,
              tClose,
              durationMs,
              attempt,
              usedPad: padGuess,
            };
            break;
          }
          const delta = Math.max(-2000, Math.min(2000, desiredMs - durationMs));
          log(
            `${spec.name}: 第 ${attempt} 次成片 ${durationMs}ms 不在 [${minDurationMs}, ${maxDurationMs}]ms，` +
              `等待 ${delta >= 0 ? '+' : ''}${delta}ms 后重录`,
          );
          padGuess += delta;
        } finally {
          if (!recorded) fs.rmSync(tempDir, { recursive: true, force: true });
        }
      }
      if (!recorded) fail(`${spec.name}: 5 次录制都没拿到合适时长`);
      const {
        finalPath,
        stat,
        measured,
        checks,
        fonts,
        wallMs,
        tCtx,
        tAnim,
        tClose,
        durationMs,
        attempt,
        usedPad,
      } = recorded;
      layout[spec.name] = buildLayoutEntry(spec, measured, {
        layoutSource: 'record',
        recording: {
          file: rel(finalPath),
          bytes: stat.size,
          durationMs,
          attempts: attempt,
          wallMs,
          requestFps: FPS,
          requestSize: VIEWPORT,
          paddedMs: usedPad,
          clock: {
            tCtx,
            tAnim,
            tClose,
            loadMs: tAnim - tCtx,
            waitMs: wallMs,
            closeDelayMs: tClose - tAnim,
          },
        },
      });
      meta.segments.push({
        name: spec.name,
        targetSec: spec.targetSec,
        recordSec: spec.recordSec,
        durationMs,
        attempts: attempt,
        wallMs,
        file: rel(finalPath),
        bytes: stat.size,
        clock: { tCtx, tAnim, tClose, loadMs: tAnim - tCtx, closeDelayMs: tClose - tAnim },
        checks,
        fonts,
      });
      log(
        `recorded ${spec.name}: ${stat.size} bytes, ${durationMs}ms (attempt ${attempt}) -> ${rel(finalPath)}`,
      );
      fs.rmSync(path.join(tempRoot, spec.name), { recursive: true, force: true });
    }
  } finally {
    await browser.close();
  }
  writeJson(layoutPath, layout);
  writeJson(path.join(segDir, 'record-cards.json'), meta);
  log(`record done: ${meta.segments.length} segments`);
  for (const s of meta.segments)
    log(
      `  ${s.name}: ${s.file} (${s.bytes} bytes, target ${s.targetSec}s / record ${s.recordSec}s)`,
    );
}

// ---------------------------------------------------------------------------
// 链路探针：短录一段片头卡，验证 recordVideo/字体/尺寸（正式录制前跑）
// ---------------------------------------------------------------------------
async function probe() {
  const scenes = loadScenes();
  const spec = scenes.segments.find((s) => s.name === 'seg-00-title');
  if (!spec) fail('scenes.json 里没有 seg-00-title');
  const probeMs = Number(process.env.CARD_PROBE_MS || '4000');
  const outDir = path.join(CARDS, 'probe-video');
  fs.mkdirSync(outDir, { recursive: true });
  const label = HEADED ? 'headed-xvfb' : 'headless';
  const tempDir = path.join(outDir, `raw-${label}`);
  fs.rmSync(tempDir, { recursive: true, force: true });
  fs.mkdirSync(tempDir, { recursive: true });
  const browser = await launchBrowser();
  const tCtx = Date.now();
  try {
    const context = await browser.newContext({
      viewport: VIEWPORT,
      deviceScaleFactor: 1,
      colorScheme: 'dark',
      locale: 'zh-CN',
      recordVideo: { dir: tempDir, size: VIEWPORT, fps: FPS },
    });
    const { page, fonts } = await preparePage(context, spec, { blackPreload: true });
    // 动画从页面就绪时刻开始（首帧即 scene t=0）
    const tAnim = Date.now();
    await page.evaluate((originEpochMs) => window.Card.startAt(originEpochMs), tAnim);
    const waitTarget = probeMs + PAD_MS;
    for (;;) {
      const remain = waitTarget - (Date.now() - tAnim);
      if (remain <= 0) break;
      await page.waitForTimeout(Math.min(remain, 250));
    }
    const shotPath = path.join(outDir, `probe-${label}-mid.png`);
    await page.screenshot({ path: shotPath });
    const measured = await page.evaluate(() => window.Card.measure());
    const fontInfo = await page.evaluate(() => window.Card.fontsReady());
    const wallMs = Date.now() - tAnim;
    const tClose = Date.now();
    const video = page.video();
    await context.close();
    let tempVideo = null;
    try {
      tempVideo = await video.path();
    } catch {
      const files = fs.readdirSync(tempDir).filter((file) => file.endsWith('.webm'));
      if (files.length) tempVideo = path.join(tempDir, files[0]);
    }
    if (!tempVideo || !fs.existsSync(tempVideo)) fail('probe: 没有拿到 webm（recordVideo 失败）');
    const finalPath = path.join(outDir, `probe-${label}.webm`);
    fs.renameSync(tempVideo, finalPath);
    const stat = fs.statSync(finalPath);
    writeJson(path.join(outDir, `probe-${label}.json`), {
      generatedAt: new Date().toISOString(),
      mode: 'probe',
      headed: HEADED,
      display: process.env.DISPLAY || null,
      viewport: VIEWPORT,
      requestFps: FPS,
      requestedProbeMs: probeMs,
      paddedMs: PAD_MS,
      wallMs,
      clock: {
        tCtx,
        tAnim,
        tClose,
        loadMs: tAnim - tCtx,
        waitMs: wallMs,
        closeDelayMs: tClose - tAnim,
      },
      file: rel(finalPath),
      bytes: stat.size,
      fonts: fontInfo || fonts,
      measured: measured
        ? {
            viewport: measured.viewport,
            deviceScaleRatio: measured.deviceScaleRatio,
            elements: measured.elements.length,
          }
        : null,
    });
    log(`probe ${label}: ${stat.size} bytes, wall ${wallMs}ms -> ${rel(finalPath)}`);
    log(`probe ${label} screenshot -> ${rel(shotPath)}`);
  } finally {
    await browser.close();
  }
}

// ---------------------------------------------------------------------------
(async () => {
  log(`mode=${MODE} headed=${HEADED} padMs=${PAD_MS} card=${rel(path.join(HERE, 'card.html'))}`);
  if (MODE === 'webm-duration') {
    // 容器/宿主自检：读一个 webm 的 Duration（毫秒）
    const file = arg('file', null);
    if (!file) fail('--mode webm-duration 需要 --file <path>');
    const durationMs = readWebmDurationMs(file);
    console.log(JSON.stringify({ file, durationMs }));
    if (durationMs === null) process.exitCode = 2;
    return;
  }
  if (MODE === 'rehearse') await rehearse();
  else if (MODE === 'record') await record();
  else if (MODE === 'probe') await probe();
  else fail(`未知 --mode ${MODE}（rehearse|record|probe|webm-duration）`);
})().catch((err) => {
  console.error('[cards] FATAL:', err);
  process.exit(1);
});
