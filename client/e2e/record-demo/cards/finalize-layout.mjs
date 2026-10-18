#!/usr/bin/env node
/* DataBackup 演示视频 · 卡片段宿主侧收尾（Windows 宿主机运行）
 *
 * 做四件事：
 *   1) 用便携版 ffprobe 量 3 个 webm 的编码/分辨率/帧率/帧数/时长；
 *   2) 用 ffmpeg blackdetect 量录制开头的浏览器启动黑场（contentStartSec），供 P3 裁头对齐；
 *   3) 每段抽 3–6 帧 PNG 到 course/04-视频/work/seg/frames-cards/（目检中文/内容/无敏感信息）；
 *   4) 回填 course/04-视频/work/cards/card-layout.json（P3 生成 ASS 注解的几何 + 视频参数）。
 *
 * 用法（仓库根目录）：
 *   node client/e2e/record-demo/cards/finalize-layout.mjs
 * 依赖：course/04-视频/tools/ffmpeg/bin/{ffprobe,ffmpeg}.exe（PROBE-infra.md §A，宿主无系统 ffmpeg）
 */
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../../../..');
const WORK = path.join(REPO, 'course/04-视频/work');
const SEG_DIR = path.join(WORK, 'seg');
const FRAMES_DIR = path.join(SEG_DIR, 'frames-cards');
const LAYOUT_PATH = path.join(WORK, 'cards/card-layout.json');
const RECORD_META_PATH = path.join(SEG_DIR, 'record-cards.json');
const REPORT_PATH = path.join(SEG_DIR, 'ffprobe-cards.txt');
const FFPROBE = path.join(REPO, 'course/04-视频/tools/ffmpeg/bin/ffprobe.exe');
const FFMPEG = path.join(REPO, 'course/04-视频/tools/ffmpeg/bin/ffmpeg.exe');

const SEGMENTS = [
  { name: 'seg-00-title', targetSec: 9, frameTimesSec: [0.5, 3.0, 6.0] },
  { name: 'seg-01-data', targetSec: 20, frameTimesSec: [3.0, 8.0, 11.0, 16.0] },
  { name: 'seg-06-verify', targetSec: 24, frameTimesSec: [1.5, 5.0, 9.5, 13.0, 19.0, 23.5] },
];

function log(...args) {
  console.log('[finalize]', ...args);
}
function rel(file) {
  return path.relative(REPO, file).split(path.sep).join('/');
}
function die(message) {
  console.error(`[finalize] FATAL: ${message}`);
  process.exit(1);
}

for (const tool of [FFPROBE, FFMPEG]) {
  if (!fs.existsSync(tool)) die(`缺少便携版工具 ${tool}（见 PROBE-infra.md §A）`);
}
fs.mkdirSync(FRAMES_DIR, { recursive: true });
// 清掉上一次的抽帧，避免新旧帧混在一起
for (const old of fs.readdirSync(FRAMES_DIR)) {
  if (old.endsWith('.png')) fs.rmSync(path.join(FRAMES_DIR, old), { force: true });
}
if (!fs.existsSync(LAYOUT_PATH)) die(`缺少 ${rel(LAYOUT_PATH)}（先跑 rehearse / record）`);
const layout = JSON.parse(fs.readFileSync(LAYOUT_PATH, 'utf8'));

let recordMeta = null;
if (fs.existsSync(RECORD_META_PATH))
  recordMeta = JSON.parse(fs.readFileSync(RECORD_META_PATH, 'utf8'));

function ffprobe(file) {
  const res = spawnSync(
    FFPROBE,
    [
      '-v',
      'error',
      '-count_frames',
      '-show_entries',
      'stream=codec_name,codec_type,width,height,r_frame_rate,avg_frame_rate,nb_read_frames:format=duration,size',
      '-of',
      'json',
      file,
    ],
    { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 },
  );
  if (res.status !== 0) die(`ffprobe 失败 ${path.basename(file)}: ${res.stderr || res.error}`);
  return JSON.parse(res.stdout);
}

function blackIntervals(file) {
  const res = spawnSync(
    FFMPEG,
    [
      '-hide_banner',
      '-nostats',
      '-i',
      file,
      '-vf',
      'blackdetect=d=0.03:pix_th=0.02',
      '-an',
      '-f',
      'null',
      '-',
    ],
    { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 },
  );
  const logText = `${res.stderr || ''}\n${res.stdout || ''}`;
  const intervals = [];
  const re = /black_start:([\d.]+)\s+black_end:([\d.]+)\s+black_duration:([\d.]+)/g;
  let match = re.exec(logText);
  while (match) {
    intervals.push({
      startSec: Number(match[1]),
      endSec: Number(match[2]),
      durationSec: Number(match[3]),
    });
    match = re.exec(logText);
  }
  return intervals;
}

function extractFrame(file, atSec, outPng) {
  fs.rmSync(outPng, { force: true });
  const res = spawnSync(
    FFMPEG,
    [
      '-y',
      '-v',
      'error',
      '-ss',
      atSec.toFixed(3),
      '-i',
      file,
      '-frames:v',
      '1',
      '-update',
      '1',
      outPng,
    ],
    { encoding: 'utf8' },
  );
  if (res.status !== 0) {
    console.error(`[finalize] WARN 抽帧失败 ${path.basename(file)} @${atSec}s: ${res.stderr}`);
    return false;
  }
  return fs.existsSync(outPng);
}

// 首帧平均亮度（YAVG，limited range 黑=16）→ 用于反推「内容真正开始的时刻」
function frameLumaSeries(file, seconds = 3) {
  const res = spawnSync(
    FFMPEG,
    [
      '-hide_banner',
      '-nostats',
      '-t',
      String(seconds),
      '-i',
      file,
      '-vf',
      'signalstats,metadata=mode=print:key=lavfi.signalstats.YAVG:file=-',
      '-an',
      '-f',
      'null',
      '-',
    ],
    { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 },
  );
  const rows = [];
  let pts = null;
  for (const line of `${res.stdout || ''}\n${res.stderr || ''}`.split('\n')) {
    const ptsMatch = /pts_time:([\d.]+)/.exec(line);
    if (ptsMatch) {
      pts = Number(ptsMatch[1]);
      continue;
    }
    const lumaMatch = /YAVG=([\d.]+)/.exec(line);
    if (lumaMatch && pts !== null) {
      rows.push({ ptsTime: pts, yAvg: Number(lumaMatch[1]) });
      pts = null;
    }
  }
  return rows;
}

// 内容起点：第一帧亮度明显离开「limited-range 纯黑(16)」的时刻
function contentStartSec(file) {
  const rows = frameLumaSeries(file, 3);
  const hit = rows.find((row) => Math.abs(row.yAvg - 16) > 0.05);
  return hit ? Number(hit.ptsTime.toFixed(3)) : 0;
}

const report = [];
const jsonReport = {
  generatedAt: new Date().toISOString(),
  ffprobe: rel(FFPROBE),
  ffmpeg: rel(FFMPEG),
  segments: [],
  frames: [],
};
if (recordMeta) {
  report.push(
    `录制脚本: ${rel(RECORD_META_PATH)}（headed=${recordMeta.headed}, fps=${recordMeta.fps}, viewport=${recordMeta.viewport.width}x${recordMeta.viewport.height}, recordVideo.size=${recordMeta.recordVideo.size.width}x${recordMeta.recordVideo.size.height}）`,
  );
}
report.push('');
report.push(
  'seg             codec  size        fps     dur(s)   frames  bytes    start(s) content(s) black(脚本内淡入/淡出)',
);
report.push(
  '--------------- ------ ----------- ------- -------- ------- -------- -------- ---------- ----------------------',
);

const recordings = {};
for (const seg of SEGMENTS) {
  const file = path.join(SEG_DIR, `${seg.name}.webm`);
  if (!fs.existsSync(file)) {
    report.push(`${seg.name.padEnd(15)}  (缺失，跳过)`);
    continue;
  }
  const probe = ffprobe(file);
  const stream = (probe.streams || []).find((s) => s.codec_type === 'video') || {};
  const durationSec = Number(probe.format?.duration || 0);
  const frames = Number(stream.nb_read_frames || 0);
  const [num, den] = String(stream.r_frame_rate || '0/1')
    .split('/')
    .map(Number);
  const fps = den ? num / den : 0;
  const black = blackIntervals(file);
  const series = frameLumaSeries(file, 3);
  const firstFrameYAvg = series.length ? Number(series[0].yAvg.toFixed(4)) : null;
  const startSec = contentStartSec(file);
  const bytes = Number(probe.format?.size || fs.statSync(file).size);

  const rec = {
    file: rel(file),
    codec: stream.codec_name,
    width: stream.width,
    height: stream.height,
    fps: Number(fps.toFixed(3)),
    frames,
    durationSec: Number(durationSec.toFixed(3)),
    bytes,
    targetSec: seg.targetSec,
    contentStartSec: startSec,
    contentDurationSec: Number((durationSec - startSec).toFixed(3)),
    contentStartNote: `动画从页面就绪时刻开始；webm 开头 ${startSec}s 为录制器启动黑场，P3 用 -ss ${startSec} 裁掉即可对齐 scene t=0`,
    firstFrameYAvg,
    blackIntervals: black,
    audio: (probe.streams || []).some((s) => s.codec_type === 'audio'),
  };
  recordings[seg.name] = rec;
  jsonReport.segments.push(rec);

  report.push(
    `${seg.name.padEnd(15)} ${String(rec.codec).padEnd(6)} ${`${rec.width}x${rec.height}`.padEnd(11)} ` +
      `${String(rec.fps).padEnd(7)} ${String(rec.durationSec).padEnd(8)} ${String(frames).padEnd(7)} ` +
      `${String(bytes).padEnd(8)} ${String(rec.contentStartSec).padEnd(8)} ${String(rec.contentDurationSec).padEnd(10)} ` +
      `${black.map((b) => `${b.startSec}-${b.endSec}`).join(',') || '-'}`,
  );

  if (layout[seg.name]) {
    layout[seg.name].recording = rec;
  }

  // 抽帧：时间点相对内容起点（startSec 之后），存 frames-cards/
  for (const relT of seg.frameTimesSec) {
    const absT = startSec + relT;
    const outPng = path.join(FRAMES_DIR, `${seg.name}-t${relT.toFixed(1)}s.png`);
    if (absT >= durationSec) continue;
    if (extractFrame(file, absT, outPng)) {
      jsonReport.frames.push({
        segment: seg.name,
        relSec: relT,
        absSec: Number(absT.toFixed(3)),
        file: rel(outPng),
      });
    }
  }
}

report.push('');
for (const seg of SEGMENTS) {
  const rec = recordings[seg.name];
  if (!rec) continue;
  const target = seg.targetSec;
  const expected = target + 1;
  const delta = rec.durationSec - expected;
  report.push(
    `${seg.name}: 目标 ${target}s（录制期望 ${expected}s ±1s）→ 实测 ${rec.durationSec}s（Δ${delta >= 0 ? '+' : ''}${delta.toFixed(2)}s），` +
      `内容起点 ${rec.contentStartSec}s / 内容时长 ${rec.contentDurationSec}s，` +
      `${rec.frames} 帧 @ ${rec.fps}fps，${rec.width}x${rec.height}，${rec.codec}，音轨=${rec.audio ? '有' : '无'}`,
  );
  if (delta > 1.5 || delta < -0.3) report.push(`  ⚠ 时长偏差超过 1s，请检查录制/裁头`);
  if (rec.contentDurationSec < target) report.push(`  ⚠ 去掉录制器启动黑场后内容不足 ${target}s`);
  if (Math.abs(rec.fps - 30) > 0.01) report.push(`  ⚠ 帧率不是 30fps`);
  if (rec.width !== 1920 || rec.height !== 1080) report.push(`  ⚠ 分辨率不是 1920x1080`);
  if (rec.audio) report.push(`  ⚠ webm 带了音轨（卡片段应为无声，音频由 P3 混入）`);
}
report.push('');
report.push(
  `抽帧目录: ${rel(FRAMES_DIR)}（${jsonReport.frames.length} 帧，逐帧目检中文/内容/敏感信息）`,
);

layout.finalizedAt = new Date().toISOString();
layout.recordings = recordings;
layout.ffprobe = { tool: rel(FFPROBE), generatedAt: jsonReport.generatedAt };
if (!layout.notes) layout.notes = [];
layout.notes = layout.notes.filter(
  (n) => !n.startsWith('录制 webm 开头') && !n.startsWith('webm 为 Playwright recordVideo'),
);
layout.notes.push(
  'webm 为 Playwright recordVideo 直出：VP9、无音轨。recording.contentStartSec 是录像器启动黑场长度（约 0.03–0.13s），P3 用 -ss contentStartSec 裁头即可让画面从 scene t=0 开始。',
);
layout.notes.push(
  'recording.contentDurationSec 是裁头后的可用内容时长（≥ 分镜时长方合格）；durationSec 比它多约 1s 余量。',
);
layout.notes.push(
  'recording.blackIntervals 是脚本内的黑场（片头文字淡入、片尾淡出），属于画面内容，不要裁掉。',
);

fs.writeFileSync(LAYOUT_PATH, `${JSON.stringify(layout, null, 2)}\n`);
fs.writeFileSync(REPORT_PATH, `${report.join('\n')}\n`);
fs.writeFileSync(
  path.join(SEG_DIR, 'ffprobe-cards.json'),
  `${JSON.stringify(jsonReport, null, 2)}\n`,
);

for (const line of report) log(line);
log(`card-layout.json 已回填 recording 字段 -> ${rel(LAYOUT_PATH)}`);
log(`ffprobe 报告 -> ${rel(REPORT_PATH)}`);
