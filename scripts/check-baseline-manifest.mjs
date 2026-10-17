#!/usr/bin/env node
/**
 * 校验截图基线清单（client/e2e/baselines.md）与磁盘上的基线文件一一对应。
 *
 * 目的：新增/删除基线时必须显式更新清单（迫使作者说明"这张图保护什么"），
 * 避免截图在无人过目的情况下被批量覆盖。见 docs/client/visual-regression.md。
 *
 * 用法：node scripts/check-baseline-manifest.mjs
 */
import { readFileSync, readdirSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const e2eDir = path.join(root, 'client', 'e2e');
const manifestPath = path.join(e2eDir, 'baselines.md');

const manifest = readFileSync(manifestPath, 'utf8');
const listed = new Set(
  [...manifest.matchAll(/`([^`]+\.png)`/g)].map((m) => m[1].replaceAll('\\', '/')),
);

const actual = new Set();
for (const entry of readdirSync(e2eDir)) {
  const dir = path.join(e2eDir, entry);
  if (!entry.endsWith('-snapshots') || !statSync(dir).isDirectory()) continue;
  for (const file of readdirSync(dir)) {
    if (file.endsWith('.png')) actual.add(`${entry}/${file}`);
  }
}

const missingInManifest = [...actual].filter((f) => !listed.has(f)).sort();
const missingOnDisk = [...listed].filter((f) => !actual.has(f)).sort();

if (missingInManifest.length === 0 && missingOnDisk.length === 0) {
  console.log(`baseline manifest OK (${actual.size} files)`);
  process.exit(0);
}

if (missingInManifest.length > 0) {
  console.error('基线未登记到 client/e2e/baselines.md：');
  for (const f of missingInManifest) console.error(`  + ${f}`);
}
if (missingOnDisk.length > 0) {
  console.error('清单里的基线在磁盘上不存在：');
  for (const f of missingOnDisk) console.error(`  - ${f}`);
}
process.exit(1);
