/* global window, document -- evaluate 回调在页面内执行 */
/**
 * 客户端分段录制 · 通用运行器（启动 / 光标注入 / 时间轴 / 落盘 / 日志 / 注解坐标）。
 *
 * 环境（容器内由 run-in-docker.sh 设置）：
 *   DISPLAY=:98、HOME=/home/demo、XDG_CONFIG_HOME=/home/demo/.config、
 *   DATABACKUP_BIN=/workspace/backend/bin/databackup、
 *   PLAYWRIGHT_BROWSERS_PATH=/workspace/course/04-视频/tools/ms-playwright（缺它 recordVideo 挂死）
 *
 * 输出：
 *   彩排 SEG_MODE=rehearsal → course/04-视频/work/rehearsal/<id>-rehearsal-pass<N>.{webm,json}
 *   正式 SEG_MODE=formal    → course/04-视频/work/seg/<id>.{webm,json} + annotation-boxes.json
 *   seed 日志               → course/04-视频/work/seed/<id>-<mode>-pass<N>-<ts>.json
 */
import { execFile } from 'node:child_process';
import { mkdir, readdir, readFile, rename, rm, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { _electron as electron } from '@playwright/test';

import { assertCursorVisible, injectCursor } from './cursor.mjs';
import { seedHome } from './seed.mjs';
import { Timeline } from './timeline.mjs';

export const repoDir = path.resolve(import.meta.dirname, '..', '..', '..', '..');
export const clientDir = path.join(repoDir, 'client');
export const WORK = path.join(repoDir, 'course', '04-视频', 'work');
export const SEG_DIR = path.join(WORK, 'seg');
export const REHEARSAL_DIR = path.join(WORK, 'rehearsal');
export const SEED_DIR = path.join(WORK, 'seed');

const backendBin = process.env.DATABACKUP_BIN ?? path.join(repoDir, 'backend', 'bin', 'databackup');
/** 容器的 -e VAR= 会让变量存在但为空串，必须按"正数才采用"解析。 */
const positiveNum = (v, fallback) => {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : fallback;
};
const nonNegativeNum = (v, fallback) => {
  const n = Number(v);
  return Number.isFinite(n) && n >= 0 && String(v ?? '').trim() !== '' ? n : fallback;
};
const VIDEO_W = positiveNum(process.env.SEG_VIDEO_W, 2240);
const VIDEO_H = positiveNum(process.env.SEG_VIDEO_H, 1440);
const FPS = positiveNum(process.env.SEG_VIDEO_FPS, 30);
/** 默认时间轴偏移（对齐全色块实测 ≈0.5s；之后从彩排 JSON 读实测值）。 */
const DEFAULT_K_MS = positiveNum(process.env.SEG_K_MS, 500);
/** 状态栏就绪后到时间轴起点的稳定等待（让启动画面落定）。 */
const INTRO_MS = nonNegativeNum(process.env.SEG_INTRO_MS, 300);

const noSandbox = typeof process.getuid === 'function' && process.getuid() === 0;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function ts() {
  return new Date().toISOString().replace(/[:.]/g, '-').replace('T', '_').slice(0, 19);
}

/** Playwright recordVideo 用的自带 ffmpeg（唯一能拿来量 webm 时长的容器内二进制）。 */
async function findPlaywrightFfmpeg() {
  const root = process.env.PLAYWRIGHT_BROWSERS_PATH;
  if (!root) throw new Error('PLAYWRIGHT_BROWSERS_PATH 未设置（recordVideo 硬前提）');
  const dirs = (await readdir(root).catch(() => [])).filter((d) => d.startsWith('ffmpeg-')).sort();
  for (const d of dirs.reverse()) {
    const p = path.join(root, d, 'ffmpeg-linux');
    const st = await stat(p).catch(() => null);
    if (st?.isFile()) return p;
  }
  throw new Error(`找不到 ${root}/ffmpeg-*/ffmpeg-linux`);
}

/** 从 webm 中读出时长（ffmpeg 打印到 stderr 的 Duration 行）。 */
export async function probeDurationSec(file) {
  const ffmpeg = await findPlaywrightFfmpeg();
  const stderr = await new Promise((resolve) => {
    execFile(ffmpeg, ['-hide_banner', '-i', file], { timeout: 30_000 }, (_err, _stdout, errOut) =>
      resolve(String(errOut ?? '')),
    );
  });
  const m = stderr.match(/Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)/);
  if (!m) throw new Error(`解析 webm 时长失败：${file}\n${stderr.slice(0, 500)}`);
  return Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]);
}

/**
 * 找彩排对齐色块在视频里的位置，算出真实的 kMs（视频原点比脚本时间轴原点早多少）。
 * 用 Playwright 自带 ffmpeg 把左上 96×96 区域导出成原始 RGB 帧，逐帧找品红色块。
 */
async function measureFlashK(webm, wallRelMs, fps) {
  const ffmpeg = await findPlaywrightFfmpeg();
  const crop = 96;
  const raw = await new Promise((resolve, reject) => {
    execFile(
      ffmpeg,
      [
        '-hide_banner',
        '-loglevel',
        'error',
        '-i',
        webm,
        '-vf',
        `crop=${crop}:${crop}:0:0,fps=${fps}`,
        '-f',
        'rawvideo',
        '-pix_fmt',
        'rgb24',
        '-',
      ],
      { timeout: 120_000, maxBuffer: 512 * 1024 * 1024, encoding: 'buffer' },
      (err, stdout) => (err ? reject(err) : resolve(stdout)),
    );
  });
  const frameSize = crop * crop * 3;
  const frames = Math.floor(raw.length / frameSize);
  for (let i = 0; i < frames; i += 1) {
    const o = i * frameSize;
    const r = raw[o];
    const g = raw[o + 1];
    const b = raw[o + 2];
    if (r > 200 && b > 200 && g < 90) {
      return { firstFrame: i, frameTimeMs: (i / fps) * 1000, kMs: (i / fps) * 1000 - wallRelMs };
    }
  }
  return null;
}

async function fileSize(p) {
  const st = await stat(p).catch(() => null);
  return st ? st.size : 0;
}

/**
 * 关片后捕获仓库目录真实列表（每段都做，容器还活着时执行）。
 * 正式录制写 <outDir>/actual-run/<id>-ls-repo.txt；彩排写 .../actual-run/<id>-pass<N>/。
 */
async function captureRepoListing({ id, outDir, mode, pass }) {
  const dirName = mode === 'formal' ? 'actual-run' : path.join('actual-run', `${id}-pass${pass}`);
  const dir = path.join(outDir, dirName);
  await mkdir(dir, { recursive: true });
  const repo = path.join(process.env.HOME ?? '/home/demo', 'DataBackup');
  const r = await new Promise((resolve) => {
    execFile('ls', ['-la', repo], { maxBuffer: 8 * 1024 * 1024 }, (err, stdout, stderr) =>
      resolve({
        code: err ? (typeof err.code === 'number' ? err.code : 1) : 0,
        stdout: String(stdout ?? ''),
        stderr: String(stderr ?? ''),
      }),
    );
  });
  const file = path.join(dir, `${id}-ls-repo.txt`);
  await writeFile(
    file,
    [
      `# ls -la ${repo}`,
      `# 采集时刻（容器内）: ${new Date().toISOString()}`,
      `# 退出码: ${r.code}`,
      '--- stdout ---',
      r.stdout.trim() || '(空)',
      '--- stderr ---',
      r.stderr.trim() || '(空)',
      '',
    ].join('\n'),
  );
  return { file, code: r.code, lines: r.stdout.trim().split('\n').filter(Boolean).length };
}

/** 等 close 后 webm 落盘（Playwright 的 flush 是异步的）。 */
async function waitForWebm(dir) {
  for (let i = 0; i < 80; i += 1) {
    const files = (await readdir(dir).catch(() => [])).filter((f) => f.endsWith('.webm'));
    const sizes = await Promise.all(files.map((f) => fileSize(path.join(dir, f))));
    if (files.length > 0 && sizes.every((s) => s > 10_000)) {
      return path.join(dir, files[0]);
    }
    await sleep(250);
  }
  throw new Error(`close 后 20s 内没有等到 webm 落盘：${dir}`);
}

/** 读上一遍彩排的 kMs（正式录制/第二遍彩排用）。没有就返回 null。 */
async function readRehearsalK(id, preferPass) {
  const order =
    preferPass === '2'
      ? [`${id}-rehearsal-pass2.json`, `${id}-rehearsal-pass1.json`]
      : [`${id}-rehearsal-pass1.json`, `${id}-rehearsal-pass2.json`];
  const loaded = [];
  for (const f of order) {
    try {
      loaded.push({ f, j: JSON.parse(await readFile(path.join(REHEARSAL_DIR, f), 'utf8')) });
    } catch {
      // 没有这一遍，跳过
    }
  }
  // 1) 优先「对齐色块」实测值（精确到 1 帧，不怕录屏时基偏差）
  for (const { f, j } of loaded) {
    const k = j.timing?.kMsFlashMeasured;
    if (typeof k === 'number') return { kMs: k, from: `${f}:flash` };
  }
  // 2) 退化为时长反推值
  for (const { f, j } of loaded) {
    const k = j.timing?.kMsMeasured ?? j.timing?.kMsActual;
    if (typeof k === 'number') return { kMs: k, from: `${f}:duration` };
  }
  return null;
}

/** 合并写 annotation-boxes.json（每段录制时 upsert 自己那一段）。 */
async function upsertAnnotationBoxes(segId, boxes) {
  const out = path.join(SEG_DIR, 'annotation-boxes.json');
  await mkdir(SEG_DIR, { recursive: true });
  let all;
  try {
    all = JSON.parse(await readFile(out, 'utf8'));
  } catch {
    all = {};
  }
  all[segId] = boxes;
  await writeFile(out, `${JSON.stringify(all, null, 2)}\n`);
  return out;
}

function envFacts() {
  return {
    display: process.env.DISPLAY ?? null,
    home: process.env.HOME ?? null,
    xdgConfigHome: process.env.XDG_CONFIG_HOME ?? null,
    backendBin,
    playwrightBrowsersPath: process.env.PLAYWRIGHT_BROWSERS_PATH ?? null,
    node: process.version,
    uid: typeof process.getuid === 'function' ? process.getuid() : null,
  };
}

/**
 * 运行一个客户端段。
 * @param {object} opts
 * @param {string} opts.id 段 id（seg-02-setup …）
 * @param {number} opts.targetSec 分镜目标时长（秒）
 * @param {'empty'|'history'} opts.preset HOME 预置类型
 * @param {string} [opts.note] 段说明（写进 JSON）
 * @param {(ctx: object) => Promise<void>} opts.steps 段内操作（用 tl.at / tl.state 控制节奏）
 * @param {(ctx: object) => Promise<object>} [opts.annotations] 录制末尾采样注解目标 CSS 坐标
 * @param {number} [opts.closeMarginMs] 目标时长之外的录制余量（默认 1500ms）
 */
export async function runSegment(opts) {
  const {
    id,
    targetSec,
    preset,
    note = '',
    steps,
    annotations,
    finalize,
    closeMarginMs = nonNegativeNum(process.env.SEG_CLOSE_MARGIN_MS, 2000),
  } = opts;
  const mode = process.env.SEG_MODE ?? 'rehearsal';
  const pass = String(process.env.SEG_PASS || (mode === 'formal' ? '1' : '1'));
  const stamp = ts();
  const log = (...a) => console.log(`[${id} +${((Date.now() - tRun0) / 1000).toFixed(1)}s]`, ...a);

  const outDir = mode === 'formal' ? SEG_DIR : REHEARSAL_DIR;
  const videoDir = path.join(outDir, 'raw', `${id}-${mode}-pass${pass}-${stamp}`);
  await mkdir(videoDir, { recursive: true });
  await mkdir(SEED_DIR, { recursive: true });
  await mkdir(outDir, { recursive: true });

  // 记录本次运行的元数据
  const tRun0 = Date.now();
  const startedAt = new Date().toISOString();

  // kMs：正式录制/第二遍彩排用彩排实测值；否则用默认值
  let kMs = DEFAULT_K_MS;
  let kFrom = 'default';
  const kEnvRaw = process.env.SEG_K_MS;
  if (kEnvRaw && Number.isFinite(Number(kEnvRaw)) && Number(kEnvRaw) > 0) {
    kMs = Number(kEnvRaw);
    kFrom = 'env';
  } else if (mode === 'formal' || pass === '2') {
    const r = await readRehearsalK(id, mode === 'formal' ? '2' : '1');
    if (r) {
      kMs = r.kMs;
      kFrom = r.from;
    }
  }

  // 1) 数据预置（与录制分离：先把 HOME/仓库/源树造好再启动应用）
  const tSeed = Date.now();
  const seedFile = path.join(SEED_DIR, `${id}-${mode}-pass${pass}-${stamp}.json`);
  const seed = await seedHome(preset, { logFile: seedFile });
  log(`seed done in ${((Date.now() - tSeed) / 1000).toFixed(1)}s -> ${seedFile}`);

  // 2) 启动 Electron + 录屏
  log(`launch electron (video ${VIDEO_W}x${VIDEO_H}@${FPS})`);
  const app = await electron.launch({
    args: [...(noSandbox ? ['--no-sandbox'] : []), '--force-device-scale-factor=2', '.'],
    cwd: clientDir,
    env: {
      ...process.env,
      HOME: process.env.HOME ?? '/home/demo',
      XDG_CONFIG_HOME: process.env.XDG_CONFIG_HOME ?? `${process.env.HOME ?? '/home/demo'}/.config`,
      DATABACKUP_BIN: backendBin,
    },
    recordVideo: { dir: videoDir, size: { width: VIDEO_W, height: VIDEO_H }, fps: FPS },
    timeout: 90_000,
  });
  const electronStderr = [];
  app.process().stderr?.on('data', (c) => {
    for (const line of String(c).split('\n').filter(Boolean)) electronStderr.push(line);
    if (electronStderr.length > 80) electronStderr.splice(0, electronStderr.length - 80);
  });

  const win = await app.firstWindow({ timeout: 90_000 });
  const consoleErrors = [];
  win.on('console', (m) => {
    if (m.type() === 'error') consoleErrors.push(m.text());
  });
  win.on('pageerror', (e) => consoleErrors.push(`pageerror: ${e.message}`));

  const tWindow = Date.now();
  log(`window ready after ${((tWindow - tRun0) / 1000).toFixed(2)}s; waiting statusbar`);
  // 尽早注入光标层：录屏从页面创建就开始了，状态栏就绪前也可以先把光标显示出来。
  await injectCursor(win).catch((e) => log(`cursor inject warning: ${String(e).slice(0, 200)}`));
  log('cursor overlay injected');
  await win
    .getByTestId('statusbar')
    .filter({ hasText: /后端版本: 1\.0\.1/ })
    .waitFor({ timeout: 90_000 });
  const statusbarText = (await win.getByTestId('statusbar').innerText()).trim();
  const tReady = Date.now();
  log(
    `statusbar: ${JSON.stringify(statusbarText)} (+${((tReady - tWindow) / 1000).toFixed(2)}s after window)`,
  );

  const windowInfo = await win.evaluate(() => ({
    innerWidth: window.innerWidth,
    innerHeight: window.innerHeight,
    devicePixelRatio: window.devicePixelRatio,
    screenCssWidth: window.screen.width,
    screenCssHeight: window.screen.height,
    userAgent: navigator.userAgent,
  }));

  // 3) 光标层已在 firstWindow 后注入，这里只补一次兜底（页面重载时不会丢，幂等）
  await injectCursor(win).catch(() => {});

  // 4) 时间轴（先让启动画面稳定，再开始计成片时间）
  await new Promise((r) => setTimeout(r, INTRO_MS));
  await win.waitForTimeout(0);
  const tl = new Timeline(win, { kMs, log: (...a) => log(...a) });
  tl.start();

  // 4b) 对齐探针：彩排第一遍在时间轴起点闪 48×48 品红色块 260ms，录完后按帧找回
  // 它在视频里的位置，就能精确算出「视频原点 ↔ 脚本原点」的偏移 kMs（正式录制不闪）。
  let syncFlash = null;
  if (process.env.SEG_SYNC_FLASH === '1') {
    await win.evaluate(() => {
      const d = document.createElement('div');
      d.id = 'demo-sync-flash';
      d.style.cssText =
        'position:fixed;left:0;top:0;width:48px;height:48px;background:#ff00ff;' +
        'z-index:2147483645;pointer-events:none;';
      document.body.appendChild(d);
    });
    syncFlash = { wallRelMs: tl.relMs() };
    tl.mark('sync-flash-on');
    await win.waitForTimeout(260);
    await win.evaluate(() => document.getElementById('demo-sync-flash')?.remove());
    tl.mark('sync-flash-off');
    log(`sync flash at wallRel=${syncFlash.wallRelMs}ms`);
  }

  const annotationBoxes = {};
  const ann = async (key, locator, selector) => {
    try {
      await locator.first().waitFor({ state: 'visible', timeout: 15_000 });
      const box = await locator.first().boundingBox();
      if (!box) throw new Error('boundingBox 为空');
      annotationBoxes[key] = {
        css: {
          x: Math.round(box.x * 10) / 10,
          y: Math.round(box.y * 10) / 10,
          w: Math.round(box.width * 10) / 10,
          h: Math.round(box.height * 10) / 10,
        },
        selectors: selector,
        sampledAtMs: Math.round(tl.videoMs()),
      };
      log(
        `annotation ${key} css=${JSON.stringify(annotationBoxes[key].css)} @${(tl.videoMs() / 1000).toFixed(2)}s`,
      );
    } catch (e) {
      annotationBoxes[key] = { error: String(e).slice(0, 300), selectors: selector };
      log(`annotation ${key} FAILED: ${String(e).slice(0, 160)}`);
    }
  };

  const ctx = { win, app, tl, ann, log, windowInfo, seed };
  let failure = null;
  const tSteps = Date.now();
  try {
    await steps(ctx);
  } catch (e) {
    failure = e;
    log(`STEPS FAILED: ${e?.stack ?? e}`);
  }
  log(`steps done in ${((Date.now() - tSteps) / 1000).toFixed(1)}s`);

  // 5a) 注解坐标（在最终画面状态下采样）。必须在 padding/关片之前完成——
  //     UI 采样本身要花 1–2s，若放在 padding 之后会把视频时长拖长。
  if (annotations) {
    try {
      await annotations(ctx);
    } catch (e) {
      log(`annotations FAILED: ${String(e).slice(0, 200)}`);
    }
  }

  // 5) padding：录到「目标时长 + 余量」再关（guardClose 要求任务已结束）
  const closeRelMs = targetSec * 1000 + closeMarginMs - kMs;
  const padWait = closeRelMs - tl.relMs();
  if (padWait > 0) {
    log(`pad ${(padWait / 1000).toFixed(2)}s (closeRel=${(closeRelMs / 1000).toFixed(2)}s)`);
    await win.waitForTimeout(padWait);
  } else {
    log(`WARN 已超时 ${(-padWait / 1000).toFixed(2)}s，不再 padding`);
  }
  const cursorCheck = await assertCursorVisible(win).catch((e) => ({
    ok: false,
    reason: String(e),
  }));

  const timing = {
    tRun0ToWindowMs: tWindow - tRun0,
    windowToStatusbarMs: tReady - tWindow,
    statusbarToTimelineMs: INTRO_MS,
    timelineRelBeforeCloseMs: tl.relMs(),
    plannedKMs: kMs,
    kFrom,
    plannedCloseRelMs: closeRelMs,
  };

  // 7) 关闭应用（先确认没有跑着的任务，否则 guardClose 会弹确认框）
  const tCloseAbs = Date.now();
  log('closing app');
  let closeTimer = null;
  await Promise.race([
    app.close(),
    new Promise((_, reject) => {
      closeTimer = setTimeout(
        () => reject(new Error('app.close() 超过 30s 未完成（guardClose 弹框？）')),
        30_000,
      );
    }),
  ])
    .catch(async (e) => {
      log(`app.close 失败：${String(e).slice(0, 200)}；尝试 kill`);
      try {
        app.process().kill('SIGKILL');
      } catch {
        // 忽略
      }
      await sleep(1000);
    })
    .finally(() => {
      if (closeTimer) clearTimeout(closeTimer);
    });
  const tClosedAbs = Date.now();
  log(`app closed (${((tClosedAbs - tCloseAbs) / 1000).toFixed(2)}s)`);

  // 8) 等 webm 落盘 + 量时长
  const webmRaw = await waitForWebm(videoDir);
  const D = await probeDurationSec(webmRaw);
  const bytes = await fileSize(webmRaw);
  // 视频起点 S ≈ 视频结束时刻 - 时长；结束时刻取「关闭开始/进程退出」中点（录像在
  // 页面关闭时 flush，进程退出略晚）。
  const tVideoEnd = (tCloseAbs + tClosedAbs) / 2;
  const kMsActual = tl.t0 - (tVideoEnd - D * 1000);
  // 彩排第一遍有对齐色块：按帧找回它，得到精确 kMs（不受录屏编码时基影响）。
  let flashMeasured = null;
  if (syncFlash) {
    try {
      flashMeasured = await measureFlashK(webmRaw, syncFlash.wallRelMs, FPS);
    } catch (e) {
      log(`对齐色块测量失败：${String(e).slice(0, 200)}`);
    }
  }
  const kMsMeasured =
    flashMeasured && Number.isFinite(flashMeasured.kMs)
      ? flashMeasured.kMs
      : kFrom !== 'default'
        ? kMs // 正式录制没有色块：用彩排实测值（kFrom 指向 rehearsal JSON）
        : kMsActual;
  log(
    `webm ${path.basename(webmRaw)} duration=${D.toFixed(2)}s size=${(bytes / 1e6).toFixed(2)}MB ` +
      `kMsActual=${Math.round(kMsActual)} kMsFlash=${flashMeasured ? Math.round(flashMeasured.kMs) : 'n/a'}`,
  );

  // 9) 归位文件 + 写 JSON 日志
  const baseName = mode === 'formal' ? id : `${id}-rehearsal-pass${pass}`;
  const webmOut = path.join(outDir, `${baseName}.webm`);
  await rename(webmRaw, webmOut).catch(async () => {
    // 跨设备/目标已存在时兜底：copy+rm
    const { copyFile } = await import('node:fs/promises');
    await copyFile(webmRaw, webmOut);
    await rm(webmRaw, { force: true });
  });
  await rm(videoDir, { recursive: true, force: true });

  // 9a) 每段都捕获仓库目录真实列表（close 之后、容器退出前；正式写 work/seg/actual-run/）
  let repoListing;
  try {
    repoListing = await captureRepoListing({ id, outDir, mode, pass });
    log(`repo listing -> ${repoListing.file}`);
  } catch (e) {
    repoListing = { error: String(e).slice(0, 300) };
    log(`repo listing FAILED: ${String(e).slice(0, 200)}`);
  }

  const events = tl.events.map((e) => ({
    name: e.name,
    wallMs: e.wallMs,
    videoMs: Math.round(e.wallMs + kMsMeasured),
    contentMs: Math.round(e.wallMs),
    ...(e.plannedVideoSec !== undefined ? { plannedVideoSec: e.plannedVideoSec } : {}),
    ...(e.plannedContentSec !== undefined ? { plannedContentSec: e.plannedContentSec } : {}),
    ...(e.plannedVideoMs !== undefined ? { plannedVideoMs: e.plannedVideoMs } : {}),
    ...(e.waitMs !== undefined ? { waitMs: e.waitMs } : {}),
  }));
  const keyStates = events.filter(
    (e) => e.name.startsWith('state:') || e.name.startsWith('hold:end'),
  );
  const stepsDone = events.filter((e) => e.name.startsWith('step:'));

  // 9b) 真实产物捕获（seg-03/seg-05 用）：在 app.close 之后、容器还活着时执行，
  //     保证 S6 卡片/S3、S5 完成卡的数字与这次录制产生的视频一致。
  let finalizeInfo = null;
  if (finalize) {
    try {
      finalizeInfo = await finalize({
        ...ctx,
        video: { file: webmOut, durationSec: D },
        recordSoFar: events,
      });
      log(`finalize ok -> ${JSON.stringify(finalizeInfo)?.slice(0, 300)}`);
    } catch (e) {
      finalizeInfo = { error: String(e?.stack ?? e).slice(0, 2000) };
      log(`finalize FAILED: ${String(e).slice(0, 300)}`);
    }
  }

  const record = {
    id,
    mode,
    pass,
    note,
    targetSec,
    recordedAt: startedAt,
    finishedAt: new Date().toISOString(),
    env: envFacts(),
    preset,
    seed,
    window: windowInfo,
    statusbar: statusbarText,
    video: {
      file: webmOut,
      bytes,
      durationSec: Number(D.toFixed(3)),
      requested: { width: VIDEO_W, height: VIDEO_H, fps: FPS },
    },
    timing: {
      ...timing,
      wallToCloseMs: tCloseAbs - tRun0,
      closeDurationMs: tClosedAbs - tCloseAbs,
      kMsActual: Math.round(kMsActual),
      kMsFlashMeasured: flashMeasured ? Math.round(flashMeasured.kMs) : null,
      kMsMeasured: Math.round(kMsMeasured),
      contentOffsetSec: Number((kMsMeasured / 1000).toFixed(3)),
      contentDurationSec: Number(((D * 1000 - kMsMeasured) / 1000).toFixed(3)),
    },
    cursor: cursorCheck,
    annotations: annotationBoxes,
    repoListing,
    events,
    keyStates,
    steps: stepsDone,
    finalize: finalizeInfo,
    consoleErrors: [...new Set(consoleErrors)].slice(0, 20),
    electronStderrTail: electronStderr.slice(-30),
    failure: failure ? String(failure.stack ?? failure).slice(0, 4000) : null,
  };
  const jsonOut = path.join(outDir, `${baseName}.json`);
  await writeFile(jsonOut, `${JSON.stringify(record, null, 2)}\n`);

  // 10) 注解坐标 upsert（正式录制也写，彩排也写方便 Lead 提前换算）
  if (Object.keys(annotationBoxes).length > 0) {
    const p = await upsertAnnotationBoxes(id, annotationBoxes);
    log(`annotation boxes -> ${p}`);
  }

  // 11) 摘要（给编排脚本 grep / 给 Lead 汇报用）
  const summary = {
    id,
    mode,
    pass,
    webm: webmOut,
    json: jsonOut,
    durationSec: Number(D.toFixed(2)),
    targetSec,
    deltaSec: Number((D - targetSec).toFixed(2)),
    contentOffsetSec: Number((kMsMeasured / 1000).toFixed(2)),
    contentDurationSec: Number(((D * 1000 - kMsMeasured) / 1000).toFixed(2)),
    kMsMeasured: Math.round(kMsMeasured),
    keyStates: keyStates.map((e) => ({
      name: e.name,
      videoSec: Number((e.videoMs / 1000).toFixed(2)),
      contentSec: Number((e.contentMs / 1000).toFixed(2)),
    })),
    finalize: finalizeInfo,
    failure: record.failure,
  };
  console.log(`SEG_RESULT ${JSON.stringify(summary)}`);
  if (failure) process.exitCode = 1;
  return summary;
}
