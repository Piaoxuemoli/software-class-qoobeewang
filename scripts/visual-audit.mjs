#!/usr/bin/env node
/**
 * 截图基线自动审计（docs/client/visual-regression.md §4）。
 *
 * 不依赖第三方库：自己解 PNG（zlib + 反滤波），检查每张基线是否仍然
 * "像这个应用"——尺寸、背景占比、内容占比、唯一色数、清单覆盖，以及
 * 基线是否由 `make e2e-update`（带 reason 的盖章）产生。
 *
 * 它不能判断文案/布局是否"业务上正确"，但能自动拦住本次事故那一类问题：
 * 空白图、白底（样式整体丢失）、元素尺寸错误、手工替换基线。
 *
 * 用法：node scripts/visual-audit.mjs [--verbose]
 */
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { inflateSync } from 'node:zlib';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const e2eDir = path.join(root, 'client', 'e2e');
const manifestPath = path.join(e2eDir, 'baselines.json');
const stampPath = path.join(e2eDir, 'baseline-stamp.json');
const verbose = process.argv.includes('--verbose');

/** 解 8-bit RGB/RGBA 非隔行 PNG，返回 { width, height, data(RGBA) }。 */
function decodePng(buf) {
  if (buf.readUInt32BE(0) !== 0x89504e47) throw new Error('not a PNG');
  let pos = 8;
  let width = 0;
  let height = 0;
  let colorType = 0;
  const idat = [];
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos);
    const type = buf.toString('ascii', pos + 4, pos + 8);
    const data = buf.subarray(pos + 8, pos + 8 + len);
    pos += 12 + len;
    if (type === 'IHDR') {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      const bitDepth = data[8];
      colorType = data[9];
      const interlace = data[12];
      if (bitDepth !== 8 || interlace !== 0 || (colorType !== 2 && colorType !== 6)) {
        throw new Error(`unsupported PNG (bitDepth=${bitDepth}, colorType=${colorType}, interlace=${interlace})`);
      }
    } else if (type === 'IDAT') {
      idat.push(data);
    } else if (type === 'IEND') {
      break;
    }
  }
  const raw = inflateSync(Buffer.concat(idat));
  const channels = colorType === 6 ? 4 : 3;
  const stride = width * channels;
  const out = Buffer.alloc(width * height * 4);
  let prev = Buffer.alloc(stride);
  let offset = 0;
  for (let y = 0; y < height; y++) {
    const filter = raw[offset++];
    const line = Buffer.from(raw.subarray(offset, offset + stride));
    offset += stride;
    for (let x = 0; x < stride; x++) {
      const a = x >= channels ? line[x - channels] : 0;
      const b = prev[x];
      const c = x >= channels ? prev[x - channels] : 0;
      let v = line[x];
      if (filter === 1) v = (v + a) & 0xff;
      else if (filter === 2) v = (v + b) & 0xff;
      else if (filter === 3) v = (v + ((a + b) >> 1)) & 0xff;
      else if (filter === 4) {
        const p = a + b - c;
        const pa = Math.abs(p - a);
        const pb = Math.abs(p - b);
        const pc = Math.abs(p - c);
        v = (v + (pa <= pb && pa <= pc ? a : pb <= pc ? b : c)) & 0xff;
      }
    }
    prev = line;
    for (let x = 0; x < width; x++) {
      const si = x * channels;
      const di = (y * width + x) * 4;
      out[di] = line[si];
      out[di + 1] = line[si + 1];
      out[di + 2] = line[si + 2];
      out[di + 3] = channels === 4 ? line[si + 3] : 255;
    }
  }
  return { width, height, data: out };
}

function stats(img) {
  const { width, height, data } = img;
  let dark = 0;
  let light = 0;
  let sum = 0;
  const colors = new Set();
  for (let i = 0; i < data.length; i += 4) {
    const r = data[i];
    const g = data[i + 1];
    const b = data[i + 2];
    const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b;
    sum += lum;
    if (r <= 28 && g <= 28 && b <= 28) dark++;
    if (lum >= 80) light++;
    colors.add(((r >> 4) << 8) | ((g >> 4) << 4) | (b >> 4));
  }
  const n = width * height;
  return {
    width,
    height,
    darkRatio: dark / n,
    contentRatio: light / n,
    meanLuminance: sum / n,
    uniqueColors: colors.size,
  };
}

const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
const files = Object.keys(manifest.files ?? {});
const failures = [];
const rows = [];

for (const rel of files) {
  const abs = path.join(e2eDir, rel);
  const spec = { ...(manifest.defaults ?? {}), ...(manifest.files[rel] ?? {}) };
  let img;
  try {
    img = decodePng(readFileSync(abs));
  } catch (err) {
    failures.push(`${rel}: 无法读取/解码（${err.message}）`);
    continue;
  }
  const s = stats(img);
  const bad = [];
  if (spec.width && s.width !== spec.width) bad.push(`宽 ${s.width}≠${spec.width}`);
  if (spec.height && s.height !== spec.height) bad.push(`高 ${s.height}≠${spec.height}`);
  if (spec.minDarkRatio != null && s.darkRatio < spec.minDarkRatio) bad.push(`暗背景占比 ${s.darkRatio.toFixed(3)}<${spec.minDarkRatio}`);
  if (spec.maxDarkRatio != null && s.darkRatio > spec.maxDarkRatio) bad.push(`暗背景占比 ${s.darkRatio.toFixed(3)}>${spec.maxDarkRatio}`);
  if (spec.minContentRatio != null && s.contentRatio < spec.minContentRatio) bad.push(`内容占比 ${s.contentRatio.toFixed(3)}<${spec.minContentRatio}（疑似空白/样式丢失）`);
  if (spec.maxContentRatio != null && s.contentRatio > spec.maxContentRatio) bad.push(`内容占比 ${s.contentRatio.toFixed(3)}>${spec.maxContentRatio}`);
  if (spec.minUniqueColors != null && s.uniqueColors < spec.minUniqueColors) bad.push(`唯一色数 ${s.uniqueColors}<${spec.minUniqueColors}`);
  if (spec.maxMeanLuminance != null && s.meanLuminance > spec.maxMeanLuminance) bad.push(`平均亮度 ${s.meanLuminance.toFixed(1)}>${spec.maxMeanLuminance}（疑似白底）`);
  rows.push({ rel, ...s, ok: bad.length === 0 });
  if (bad.length) failures.push(`${rel}: ${bad.join('；')}`);
}

// 磁盘上的基线必须都在清单里（防止新增截图绕过审计）。
const { readdirSync, statSync } = await import('node:fs');
const actual = new Set();
for (const entry of readdirSync(e2eDir)) {
  const dir = path.join(e2eDir, entry);
  if (!entry.endsWith('-snapshots') || !statSync(dir).isDirectory()) continue;
  for (const f of readdirSync(dir)) if (f.endsWith('.png')) actual.add(`${entry}/${f}`);
}
for (const f of actual) if (!files.includes(f)) failures.push(`${f}: 未登记到 client/e2e/baselines.json`);
for (const f of files) if (!actual.has(f)) failures.push(`${f}: 清单里的文件不存在`);

// 基线必须由 make e2e-update 盖章（reason + 每张图的 sha256），手工替换会被发现。
if (!failures.length) {
  const stamp = JSON.parse(readFileSync(stampPath, 'utf8'));
  if (!stamp.reason || stamp.reason.trim() === '') failures.push('baseline-stamp.json 缺少 reason');
  for (const rel of files) {
    const hash = createHash('sha256').update(readFileSync(path.join(e2eDir, rel))).digest('hex');
    if (stamp.files?.[rel]?.sha256 !== hash) {
      failures.push(`${rel}: 与 baseline-stamp.json 的哈希不符（请用 make e2e-update 更新）`);
    }
  }
}

if (verbose) {
  for (const r of rows) {
    console.log(
      `${r.ok ? 'ok  ' : 'FAIL'} ${r.rel} ${r.width}x${r.height} dark=${r.darkRatio.toFixed(2)} content=${r.contentRatio.toFixed(3)} colors=${r.uniqueColors}`,
    );
  }
}

if (failures.length) {
  console.error('视觉基线审计失败：');
  for (const f of failures) console.error(`  - ${f}`);
  process.exit(1);
}
console.log(`visual audit OK (${files.length} baselines)`);
