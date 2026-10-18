/**
 * 宿主侧 QC（Windows Git Bash / 宿主机 node 运行，不进容器）：
 *   - 对 4 个客户端段正式 webm 跑便携版 ffprobe：分辨率/fps/帧数/时长/编码/大小；
 *   - 按录制 JSON 里的关键状态时刻抽 3–4 帧到 work/seg/frames-app/（人工目检光标/状态栏/敏感信息）；
 *   - 结果写 work/seg/qc-ffprobe.json 与 qc-frames.json。
 *
 * 用法（仓库根目录）：
 *   node client/e2e/record-demo/qc-host.mjs [seg-02-setup ...]
 */
import { execFile } from 'node:child_process';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const repoDir = path.resolve(import.meta.dirname, '..', '..', '..');
const WORK = path.join(repoDir, 'course', '04-视频', 'work');
const SEG_DIR = path.join(WORK, 'seg');
const FRAMES_DIR = path.join(SEG_DIR, 'frames-app');
const FFMPEG = path.join(repoDir, 'course', '04-视频', 'tools', 'ffmpeg', 'bin', 'ffmpeg.exe');
const FFPROBE = path.join(repoDir, 'course', '04-视频', 'tools', 'ffmpeg', 'bin', 'ffprobe.exe');

const DEFAULT_IDS = ['seg-02-setup', 'seg-03-backup', 'seg-04-manage', 'seg-05-restore'];
const ids = process.argv.slice(2).length > 0 ? process.argv.slice(2) : DEFAULT_IDS;

/** 每段挑哪几个时刻抽帧：优先这些事件（+0.35s 看"状态已出现"的画面）。 */
const FRAME_PICK = {
  'seg-02-setup': [
    'step:fill-source',
    'state:filter-popover',
    'typed-name',
    'state:encrypt-on',
    'state:dryrun-preview',
  ],
  'seg-03-backup': [
    'state:progress-30',
    'state:progress-90',
    'success-settle',
    'hover-stat-输出文件',
    'hover-view-backup',
  ],
  'seg-04-manage': [
    'state:manage-list',
    'hovered-archive',
    'state:detail-sheet',
    'hover-detail-source',
    'hover-restore-btn',
  ],
  'seg-05-restore': [
    'state:password-dialog',
    'state:wrong-password-alert',
    'filled-correct-password',
    'state:restore-progress-30',
    'state:restore-succeeded',
  ],
};

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, { maxBuffer: 16 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err && err.code !== 1)
        reject(new Error(`${cmd} ${args.join(' ')}: ${stderr || err.message}`));
      else resolve({ code: err ? 1 : 0, stdout: String(stdout), stderr: String(stderr) });
    });
  });
}

await mkdir(FRAMES_DIR, { recursive: true });
const report = { generatedAt: new Date().toISOString(), segments: {} };

for (const id of ids) {
  const jsonPath = path.join(SEG_DIR, `${id}.json`);
  const webmPath = path.join(SEG_DIR, `${id}.webm`);
  let rec;
  try {
    rec = JSON.parse(await readFile(jsonPath, 'utf8'));
  } catch (e) {
    console.error(`[qc] ${id}: 缺少录制 JSON (${jsonPath})：${String(e).slice(0, 120)}`);
    report.segments[id] = { error: 'missing json' };
    continue;
  }
  const probe = await run(FFPROBE, [
    '-v',
    'error',
    '-count_frames',
    '-select_streams',
    'v:0',
    '-show_entries',
    'stream=codec_name,width,height,r_frame_rate,avg_frame_rate,nb_read_frames,pix_fmt',
    '-show_entries',
    'format=duration,size',
    '-of',
    'json',
    webmPath,
  ]).catch((e) => ({ stdout: '', stderr: String(e) }));
  let probeJson = {};
  try {
    probeJson = JSON.parse(probe.stdout);
  } catch {
    // ignore
  }
  const stream = probeJson.streams?.[0] ?? {};
  const fmt = probeJson.format ?? {};
  const segReport = {
    webm: webmPath,
    durationSec: fmt.duration ? Number(fmt.duration) : rec.video?.durationSec,
    sizeBytes: fmt.size ? Number(fmt.size) : rec.video?.bytes,
    codec: stream.codec_name,
    width: stream.width,
    height: stream.height,
    fps: stream.r_frame_rate,
    avgFps: stream.avg_frame_rate,
    frames: stream.nb_read_frames ? Number(stream.nb_read_frames) : null,
    targetSec: rec.targetSec,
    targetDeltaSec: fmt.duration ? Number((Number(fmt.duration) - rec.targetSec).toFixed(2)) : null,
    contentOffsetSec: rec.timing?.contentOffsetSec,
    contentDurationSec: rec.timing?.contentDurationSec,
    kMsMeasured: rec.timing?.kMsMeasured,
    statusbar: rec.statusbar,
    cursor: rec.cursor,
    frameShots: [],
  };

  // 抽帧：每个事件取 videoSec + 0.35s（状态出现后的画面）
  const events = [...(rec.events ?? [])];
  const want = FRAME_PICK[id] ?? [];
  for (const name of want) {
    const ev = events.find((e) => e.name === name) ?? events.find((e) => e.name.includes(name));
    if (!ev) {
      segReport.frameShots.push({ name, error: 'event not found' });
      continue;
    }
    const t = Math.min(Number(ev.videoMs) / 1000 + 0.35, Number(segReport.durationSec) - 0.2);
    const label = name.replace(/[:/]/g, '_');
    const out = path.join(FRAMES_DIR, `${id}-${label}-t${t.toFixed(2)}.png`);
    await run(FFMPEG, [
      '-y',
      '-hide_banner',
      '-loglevel',
      'error',
      '-ss',
      String(t),
      '-i',
      webmPath,
      '-frames:v',
      '1',
      '-update',
      '1',
      out,
    ]).catch((e) => segReport.frameShots.push({ name, t, error: String(e).slice(0, 200) }));
    segReport.frameShots.push({
      name,
      t,
      contentSec: Number((ev.contentMs / 1000).toFixed(2)),
      file: out,
    });
  }
  report.segments[id] = segReport;
  console.log(
    `[qc] ${id}: ${segReport.width}x${segReport.height} ${segReport.fps} ${segReport.frames ?? '?'} frames ` +
      `${segReport.durationSec}s (Δ ${segReport.targetDeltaSec}s vs ${segReport.targetSec}s)`,
  );
}

await writeFile(path.join(SEG_DIR, 'qc-ffprobe.json'), `${JSON.stringify(report, null, 2)}\n`);
console.log(`[qc] -> ${path.join(SEG_DIR, 'qc-ffprobe.json')}`);
console.log(`[qc] frames -> ${FRAMES_DIR}`);
