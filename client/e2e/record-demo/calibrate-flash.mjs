/**
 * 宿主侧：用便携版 ffmpeg 找回彩排视频里的「对齐色块」，算出精确 kMs，
 * 并回写 rehearsal JSON（容器内 Playwright 自带 ffmpeg 太精简，解不了 webm 帧）。
 *
 * 用法（仓库根目录，宿主机 node）：
 *   node client/e2e/record-demo/calibrate-flash.mjs                    # 4 段的 pass1
 *   node client/e2e/record-demo/calibrate-flash.mjs seg-02-setup ...   # 指定段
 */
import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';

const repoDir = path.resolve(import.meta.dirname, '..', '..', '..');
const WORK = path.join(repoDir, 'course', '04-视频', 'work');
const REH = path.join(WORK, 'rehearsal');
const FFMPEG = path.join(repoDir, 'course', '04-视频', 'tools', 'ffmpeg', 'bin', 'ffmpeg.exe');

const DEFAULT_IDS = ['seg-02-setup', 'seg-03-backup', 'seg-04-manage', 'seg-05-restore'];
const ids = process.argv.slice(2).length > 0 ? process.argv.slice(2) : DEFAULT_IDS;
const CROP = 96;
const FRAME_BYTES = CROP * CROP * 3;

/** JSON 里记录的是容器内路径 /workspace/...，宿主上要换成仓库根目录。 */
function hostPath(p) {
  const s = String(p ?? '');
  return s.startsWith('/workspace/') ? path.join(repoDir, s.slice('/workspace/'.length)) : s;
}

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, { maxBuffer: 64 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err)
        reject(
          new Error(
            `${path.basename(cmd)} ${args.slice(0, 6).join(' ')}…: ${stderr || err.message}`,
          ),
        );
      else resolve(stdout);
    });
  });
}

async function measure(video, wallRelMs) {
  const tmp = await mkdtemp(path.join(tmpdir(), 'dbk-flash-'));
  const rawPath = path.join(tmp, 'crop.raw');
  try {
    await run(FFMPEG, [
      '-y',
      '-hide_banner',
      '-loglevel',
      'error',
      '-i',
      video,
      '-vf',
      `crop=${CROP}:${CROP}:0:0,fps=30`,
      '-f',
      'rawvideo',
      '-pix_fmt',
      'rgb24',
      rawPath,
    ]);
    const raw = await readFile(rawPath);
    const frames = Math.floor(raw.length / FRAME_BYTES);
    for (let i = 0; i < frames; i += 1) {
      const o = i * FRAME_BYTES;
      if (raw[o] > 200 && raw[o + 2] > 200 && raw[o + 1] < 90) {
        return {
          firstFrame: i,
          frameTimeMs: (i / 30) * 1000,
          kMs: (i / 30) * 1000 - wallRelMs,
          frames,
        };
      }
    }
    return { error: `扫了 ${frames} 帧没有品红色块`, frames };
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
}

const out = {};
for (const id of ids) {
  const jsonPath = path.join(REH, `${id}-rehearsal-pass1.json`);
  let rec;
  try {
    rec = JSON.parse(await readFile(jsonPath, 'utf8'));
  } catch (e) {
    console.error(`[flash] ${id}: 读不到 ${jsonPath}: ${String(e).slice(0, 120)}`);
    continue;
  }
  const flashEv = (rec.events ?? []).find((e) => e.name === 'sync-flash-on');
  if (!flashEv) {
    console.error(`[flash] ${id}: JSON 里没有 sync-flash-on 事件（该遍没开 SEG_SYNC_FLASH？）`);
    out[id] = { error: 'no sync flash event' };
    continue;
  }
  const video = hostPath(rec.video?.file);
  const m = await measure(video, flashEv.wallMs).catch((e) => ({ error: String(e).slice(0, 300) }));
  out[id] = m;
  if (m.error) {
    console.error(`[flash] ${id}: ${m.error}`);
    continue;
  }
  const kMs = Math.round(m.kMs);
  const contentOffsetSec = Number((kMs / 1000).toFixed(3));
  rec.timing = {
    ...rec.timing,
    kMsFlashMeasured: kMs,
    kMsMeasured: kMs,
    contentOffsetSec,
    contentDurationSec: Number((((rec.video?.durationSec ?? 0) * 1000 - kMs) / 1000).toFixed(3)),
    kMsMethod: 'host-ffmpeg-flash-frame',
  };
  // 事件/关键状态按实测 kMs 重算成片坐标
  for (const e of rec.events ?? []) {
    e.videoMs = Math.round(e.wallMs + kMs);
    e.contentMs = Math.round(e.wallMs);
  }
  rec.keyStates = (rec.events ?? []).filter(
    (e) => e.name.startsWith('state:') || e.name.startsWith('hold:end'),
  );
  await writeFile(jsonPath, `${JSON.stringify(rec, null, 2)}\n`);
  console.log(
    `[flash] ${id}: 色块在第 ${m.firstFrame} 帧 (${(m.frameTimeMs / 1000).toFixed(3)}s), ` +
      `flashWall=${(flashEv.wallMs / 1000).toFixed(3)}s -> kMs=${kMs}ms  ` +
      `(duration ${rec.video?.durationSec}s, contentOffset ${contentOffsetSec}s)`,
  );
}

await writeFile(
  path.join(REH, 'flash-calibration.json'),
  `${JSON.stringify({ generatedAt: new Date().toISOString(), segments: out }, null, 2)}\n`,
);
console.log(`[flash] -> ${path.join(REH, 'flash-calibration.json')}`);
