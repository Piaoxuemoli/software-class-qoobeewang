#!/usr/bin/env node
/**
 * 给当前基线盖章：记录更新原因、时间与每张图的 sha256。
 * 只应由 `make e2e-update BASELINE_REASON="..."` 调用；visual-audit 会校验
 * 哈希，手工替换基线会被发现。
 */
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const e2eDir = path.join(root, 'client', 'e2e');
const reason = (process.argv[2] ?? process.env.BASELINE_REASON ?? '').trim();
if (reason === '') {
  console.error('usage: node scripts/baseline-stamp.mjs "为什么更新基线"');
  process.exit(1);
}

const manifest = JSON.parse(readFileSync(path.join(e2eDir, 'baselines.json'), 'utf8'));
const stamp = { reason, updatedAt: new Date().toISOString(), files: {} };
for (const rel of Object.keys(manifest.files)) {
  const hash = createHash('sha256').update(readFileSync(path.join(e2eDir, rel))).digest('hex');
  stamp.files[rel] = { sha256: hash };
}
writeFileSync(path.join(e2eDir, 'baseline-stamp.json'), `${JSON.stringify(stamp, null, 2)}\n`);
console.log(`stamped ${Object.keys(stamp.files).length} baselines: ${reason}`);
