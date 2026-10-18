/* global window, document -- evaluate 回调在页面内执行 */
/**
 * 媒体基建预检 · 录制链路探针（**不是正式分镜脚本**，只验证 Xvfb + Electron + Playwright recordVideo 可用）。
 *
 * 运行环境：databackup-dev 容器内、Xvfb :98（2560x1600x24）。
 * 由 run-probe-in-docker.sh 调用；也可手动：
 *
 *   cd /workspace/client
 *   DISPLAY=:98 HOME=/home/demo XDG_CONFIG_HOME=/home/demo/.config \
 *   DATABACKUP_BIN=/workspace/backend/bin/databackup \
 *   PROBE_VIDEO_W=1120 PROBE_VIDEO_H=720 \
 *   PROBE_VIDEO_DIR=/workspace/course/04-视频/work/probe/raw-1120 \
 *   node e2e/record-demo/probe-record.mjs
 *
 * 验证点：状态栏「后端版本: 1.0.1」、--force-device-scale-factor=2 下窗口 CSS/物理像素、
 * 设备像素比、GPU 状态、录 >15s 的 webm（分辨率/时长由外部 ffprobe 量）、HOME=/home/demo 隔离。
 * 输出：stdout 日记 + PROBE_VIDEO_DIR/probe-result.json + webm + window-physical.png。
 */
import { execFile } from 'node:child_process';
import { mkdir, readdir, rm, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { _electron as electron } from '@playwright/test';

const clientDir = path.resolve(import.meta.dirname, '..', '..');
const repoDir = path.resolve(clientDir, '..');
const HOME = process.env.PROBE_HOME ?? '/home/demo';
const videoDir =
  process.env.PROBE_VIDEO_DIR ?? path.join(repoDir, 'course', '04-视频', 'work', 'probe', 'raw');
const W = Number(process.env.PROBE_VIDEO_W ?? 1120);
const H = Number(process.env.PROBE_VIDEO_H ?? 720);
// Playwright 录像容器帧率默认 25（kDefaultFps）；显式传 PROBE_VIDEO_FPS=30 可验证 fps 选项。
const FPS = process.env.PROBE_VIDEO_FPS ? Number(process.env.PROBE_VIDEO_FPS) : undefined;
const backendBin = process.env.DATABACKUP_BIN ?? path.join(repoDir, 'backend', 'bin', 'databackup');
const noSandbox = typeof process.getuid === 'function' && process.getuid() === 0;
const extraArgs = (process.env.PROBE_EXTRA_ARGS ?? '').split(/\s+/).filter(Boolean);
const MIN_RECORD_MS = Number(process.env.PROBE_MIN_MS ?? 18_000);

const t0 = Date.now();
const log = (...a) => console.log(`[probe +${((Date.now() - t0) / 1000).toFixed(1)}s]`, ...a);

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, { env: { ...process.env, HOME } }, (err, stdout, stderr) => {
      if (err) reject(new Error(`${err.message}\n${stderr ?? ''}`));
      else resolve(stdout);
    });
  });
}

const result = {
  startedAt: new Date().toISOString(),
  container: {},
  video: { requested: { width: W, height: H, fps: FPS ?? 'default(25)' }, dir: videoDir },
  window: {},
  gpu: null,
  backend: {},
  timeline: [],
  errors: [],
  webms: [],
};

// HOME 每次重建：验证隔离性，同时给 Electron 一个干净的可写 HOME。
if (HOME === '/home/demo') await rm(HOME, { recursive: true, force: true });
await mkdir(HOME, { recursive: true });
await rm(videoDir, { recursive: true, force: true });
await mkdir(videoDir, { recursive: true });

/** HOME 下前两层条目：证明 Electron 把用户数据写进了 /home/demo（隔离可用）。 */
async function walkHome() {
  const top = await readdir(HOME, { withFileTypes: true }).catch(() => []);
  const out = [];
  for (const e of top) {
    const rel = e.name;
    out.push(e.isDirectory() ? `${rel}/` : rel);
    if (e.isDirectory()) {
      const children = await readdir(path.join(HOME, rel), { withFileTypes: true }).catch(() => []);
      for (const c of children.slice(0, 6))
        out.push(`${rel}/${c.name}${c.isDirectory() ? '/' : ''}`);
    }
  }
  return out;
}

try {
  result.container.node = (await run('node', ['--version'])).trim();
  result.container.uid = typeof process.getuid === 'function' ? process.getuid() : null;
  result.container.home = HOME;
  result.container.display = process.env.DISPLAY ?? null;
  result.container.x11Sockets = (await readdir('/tmp/.X11-unix').catch(() => [])).map(
    (f) => `/tmp/.X11-unix/${f}`,
  );
} catch (e) {
  result.container.error = String(e);
}
log('container:', JSON.stringify(result.container));

try {
  result.backend.version = (await run(backendBin, ['--version'])).trim();
} catch (e) {
  result.backend.versionError = String(e).slice(0, 400);
}
log('backend --version:', result.backend.version ?? result.backend.versionError);

const app = await electron.launch({
  args: [
    ...(noSandbox ? ['--no-sandbox'] : []),
    ...extraArgs,
    '--force-device-scale-factor=2',
    '.',
  ],
  cwd: clientDir,
  env: {
    ...process.env,
    HOME,
    XDG_CONFIG_HOME: `${HOME}/.config`,
    DATABACKUP_BIN: backendBin,
  },
  recordVideo: { dir: videoDir, size: { width: W, height: H }, ...(FPS ? { fps: FPS } : {}) },
  timeout: 60_000,
});

const electronStderr = [];
app.process().stderr?.on('data', (c) => {
  const text = String(c);
  electronStderr.push(...text.split('\n').filter(Boolean));
  if (electronStderr.length > 60) electronStderr.splice(0, electronStderr.length - 60);
});

const win = await app.firstWindow();
const consoleErrors = [];
win.on('console', (m) => {
  if (m.type() === 'error') consoleErrors.push(m.text());
});
win.on('pageerror', (e) => consoleErrors.push(`pageerror: ${e.message}`));

const windowMetrics = () =>
  win.evaluate(() => ({
    innerWidth: window.innerWidth,
    innerHeight: window.innerHeight,
    outerWidth: window.outerWidth,
    outerHeight: window.outerHeight,
    devicePixelRatio: window.devicePixelRatio,
    screenCssWidth: window.screen.width,
    screenCssHeight: window.screen.height,
    userAgent: navigator.userAgent,
    readyState: document.readyState,
  }));

const tWindow = Date.now();
try {
  result.window.before = await windowMetrics();
  log('window(before):', JSON.stringify(result.window.before));
} catch (e) {
  result.window.beforeError = String(e).slice(0, 300);
  log('window(before) FAILED:', result.window.beforeError);
}
try {
  await win.getByTestId('statusbar').waitFor({ timeout: 60_000 });
  await win
    .getByTestId('statusbar')
    .filter({ hasText: /后端版本: 1\.0\.1/ })
    .waitFor({ timeout: 60_000 });
} catch (e) {
  result.statusbarError = String(e).slice(0, 500);
  result.statusbarDebug = {
    url: win.url(),
    playwrightBrowsersPath: process.env.PLAYWRIGHT_BROWSERS_PATH ?? null,
  };
  log('statusbar FAILED:', result.statusbarError, JSON.stringify(result.statusbarDebug));
  throw e;
}
result.backend.statusbar = (await win.getByTestId('statusbar').innerText()).trim();
log('statusbar:', JSON.stringify(result.backend.statusbar));

try {
  result.gpu = await app.evaluate(({ app: electronApp }) => {
    try {
      return electronApp.getGPUFeatureStatus();
    } catch (e) {
      return { error: String(e) };
    }
  });
} catch (e) {
  result.gpu = { error: String(e) };
}
log('gpu:', JSON.stringify(result.gpu));

// 导航按钮的 CSS 像素包围盒：后期光标叠加要按 物理像素 = CSS 像素 × devicePixelRatio 换算。
result.navBoxes = {};
for (const name of ['备份', '备份管理', '设置']) {
  const box = await win
    .getByRole('button', { name, exact: true })
    .boundingBox()
    .catch(() => null);
  result.navBoxes[name] = box && {
    ...box,
    centerX: box.x + box.width / 2,
    centerY: box.y + box.height / 2,
  };
}
log('navBoxes(cssPx):', JSON.stringify(result.navBoxes));

// 设备像素级截图：用 ffprobe 量这张 PNG 的像素宽高，即窗口物理像素。
const windowShot = path.join(videoDir, 'window-physical.png');
await win.screenshot({ path: windowShot });
result.window.screenshot = windowShot;

const mark = (name) => {
  const entry = { name, tMs: Date.now() - tWindow };
  result.timeline.push(entry);
  log(`step ${name} @ +${(entry.tMs / 1000).toFixed(2)}s`);
};

mark('statusbar-ready');
await win.waitForTimeout(1500);

// 操作 1：切到「备份管理」，悬停列表（空仓库时是空态，也不算失败）。
await win.getByRole('button', { name: '备份管理', exact: true }).click();
mark('clicked 备份管理');
await win.waitForTimeout(2200);
const rows = win.getByTestId(/^backup-row-/);
const rowCount = await rows.count().catch(() => 0);
result.rows = rowCount;
if (rowCount > 0) {
  await rows.first().hover();
  mark('hover backup row');
  await rows.first().click();
  mark('clicked backup row');
  await win.waitForTimeout(1600);
  const close = win.getByRole('button', { name: '关闭', exact: true });
  if ((await close.count().catch(() => 0)) > 0) {
    await close.click().catch(() => {});
    mark('closed sheet');
  }
  await win.waitForTimeout(900);
}
await win.waitForTimeout(1200);

// 操作 2：回备份页、去设置页，制造可辨认的画面变化。
await win.getByRole('button', { name: '备份', exact: true }).click();
mark('clicked 备份');
await win.waitForTimeout(1800);
await win.getByRole('button', { name: '设置', exact: true }).click();
mark('clicked 设置');
await win.waitForTimeout(1600);
await win.getByRole('button', { name: '备份', exact: true }).click();
mark('clicked 备份(return)');

// 补时长：录满 MIN_RECORD_MS 再关闭。
const elapsed = Date.now() - tWindow;
if (elapsed < MIN_RECORD_MS) {
  log(`pad ${((MIN_RECORD_MS - elapsed) / 1000).toFixed(1)}s to reach ${MIN_RECORD_MS / 1000}s`);
  await win.waitForTimeout(MIN_RECORD_MS - elapsed);
}
result.window.after = await windowMetrics();
result.recordedWallMs = Date.now() - tWindow;
log('window(after):', JSON.stringify(result.window.after));
log(`recorded wall time: ${(result.recordedWallMs / 1000).toFixed(1)}s`);

await app.close();
log('app closed');

result.homeTree = await walkHome();
log('HOME tree:', JSON.stringify(result.homeTree));

// 等 webm 落盘（close 后异步 flush）。
let webms;
for (let i = 0; i < 40; i++) {
  const files = await readdir(videoDir).catch(() => []);
  webms = files.filter((f) => f.endsWith('.webm'));
  if (webms.length > 0) {
    const sizes = await Promise.all(
      webms.map(async (f) => (await stat(path.join(videoDir, f))).size),
    );
    if (sizes.every((s) => s > 10_000)) {
      result.webms = await Promise.all(
        webms.map(async (f) => ({
          path: path.join(videoDir, f),
          bytes: (await stat(path.join(videoDir, f))).size,
        })),
      );
      break;
    }
  }
  await new Promise((r) => setTimeout(r, 250));
}
result.errors = consoleErrors.slice(0, 20);
result.electronStderrTail = electronStderr.slice(-25);
if (result.webms.length === 0) {
  result.errors.push('no webm produced');
  process.exitCode = 1;
}

const resultPath = path.join(videoDir, 'probe-result.json');
await writeFile(resultPath, JSON.stringify(result, null, 2));
log('result ->', resultPath);
console.log(
  JSON.stringify({ webms: result.webms, timeline: result.timeline, errors: result.errors }),
);
