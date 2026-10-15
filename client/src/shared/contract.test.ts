// @vitest-environment node
/**
 * 契约测试：docs/architecture/cli-contract.md 中的 jsonl 样例与错误码表必须与 protocol.ts 一致。
 * 下面的 *Spec 常量由类型 Spec<T> 在编译期约束（漏键、多键、类型标签错误都会编译失败），
 * 运行期再用它们校验每条样例。
 */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  ERROR_CODES,
  EXIT_CODE_OF,
  FALLBACK_ERROR_CODE_OF_EXIT,
  type BackupInfo,
  type BackupResult,
  type ConfigResult,
  type DeleteResult,
  type DryRunResult,
  type ErrorEvent,
  type InfoResult,
  type ListResult,
  type LogEvent,
  type ProgressEvent,
  type RestoreResult,
  type VersionResult,
} from './protocol';

type Tag<V> = V extends number
  ? 'number'
  : V extends string
    ? 'string'
    : V extends boolean
      ? 'boolean'
      : V extends readonly unknown[]
        ? 'array'
        : 'object';

/** 每个键 → 值的类型标签；可选键的标签带 '?' 后缀。 */
type Spec<T> = {
  [K in keyof Required<T>]-?: Record<never, never> extends Pick<T, K>
    ? `${Tag<NonNullable<T[K]>>}?`
    : Tag<T[K]>;
};

const progressSpec: Spec<ProgressEvent> = {
  type: 'string',
  phase: 'string',
  files: 'number',
  bytes: 'number',
  totalFiles: 'number?',
  totalBytes: 'number?',
  current: 'string',
};
const logSpec: Spec<LogEvent> = { type: 'string', level: 'string', message: 'string' };
const errorSpec: Spec<ErrorEvent> = { type: 'string', code: 'string', message: 'string' };
const versionSpec: Spec<VersionResult> = { type: 'string', command: 'string', version: 'string' };
const backupSpec: Spec<BackupResult> = {
  type: 'string',
  command: 'string',
  name: 'string',
  path: 'string',
  files: 'number',
  dirs: 'number',
  bytes: 'number',
  outputBytes: 'number',
  compressed: 'boolean',
  encrypted: 'boolean',
  skipped: 'number',
  warnings: 'number',
  ms: 'number',
};
const dryRunSpec: Spec<DryRunResult> = {
  type: 'string',
  command: 'string',
  files: 'number',
  dirs: 'number',
  bytes: 'number',
  skipped: 'number',
};
const restoreSpec: Spec<RestoreResult> = {
  type: 'string',
  command: 'string',
  target: 'string',
  files: 'number',
  dirs: 'number',
  bytes: 'number',
  warnings: 'number',
  ms: 'number',
};
const backupInfoSpec: Spec<BackupInfo> = {
  name: 'string',
  path: 'string',
  outputBytes: 'number',
  createdAt: 'string',
  source: 'string',
  compressed: 'boolean',
  encrypted: 'boolean',
  files: 'number',
  dirs: 'number',
  bytes: 'number',
  formatVersion: 'number',
  corrupt: 'boolean',
};
const listSpec: Spec<ListResult> = {
  type: 'string',
  command: 'string',
  repo: 'string',
  backups: 'array',
};
const infoSpec: Spec<InfoResult> = { type: 'string', command: 'string', ...backupInfoSpec };
const deleteSpec: Spec<DeleteResult> = {
  type: 'string',
  command: 'string',
  name: 'string',
  path: 'string',
};
const configSpec: Spec<ConfigResult> = { type: 'string', command: 'string', repo: 'string' };

const specs: Record<string, Record<string, string>> = {
  progress: progressSpec,
  log: logSpec,
  error: errorSpec,
  'result/version': versionSpec,
  'result/backup': backupSpec,
  'result/dry-run': dryRunSpec,
  'result/restore': restoreSpec,
  'result/list': listSpec,
  'result/info': infoSpec,
  'result/delete': deleteSpec,
  'result/config': configSpec,
};

const contract = readFileSync(
  fileURLToPath(new URL('../../../docs/architecture/cli-contract.md', import.meta.url)),
  'utf8',
);

function jsonlSamples(doc: string): Record<string, unknown>[] {
  const out: Record<string, unknown>[] = [];
  let inBlock = false;
  for (const raw of doc.split('\n')) {
    const line = raw.replace(/\r$/, '');
    if (!inBlock && line.trim() === '```jsonl') inBlock = true;
    else if (inBlock && line.trim() === '```') inBlock = false;
    else if (inBlock && line.trim() !== '') out.push(JSON.parse(line) as Record<string, unknown>);
  }
  return out;
}

function section4(doc: string): string {
  const start = doc.indexOf('## 4.');
  const end = doc.indexOf('## 5.');
  expect(start).toBeGreaterThanOrEqual(0);
  expect(end).toBeGreaterThan(start);
  return doc.slice(start, end);
}

function tagOf(v: unknown): string {
  if (Array.isArray(v)) return 'array';
  return typeof v;
}

function checkShape(sample: Record<string, unknown>, spec: Record<string, string>): void {
  const where = JSON.stringify(sample);
  for (const key of Object.keys(sample)) {
    expect(Object.keys(spec), `多余字段 ${key}：${where}`).toContain(key);
  }
  for (const [key, tag] of Object.entries(spec)) {
    const optional = tag.endsWith('?');
    if (!(key in sample)) {
      expect(optional, `缺少必选字段 ${key}：${where}`).toBe(true);
      continue;
    }
    expect(tagOf(sample[key]), `字段 ${key} 类型：${where}`).toBe(tag.replace(/\?$/, ''));
  }
}

describe('CLI 契约', () => {
  const samples = jsonlSamples(contract);

  it('每条 jsonl 样例与 TS 类型一致', () => {
    expect(samples.length).toBeGreaterThan(0);
    const seen = new Set<string>();
    for (const s of samples) {
      const kind = s.type === 'result' ? `result/${String(s.command)}` : String(s.type);
      const spec = specs[kind];
      expect(spec, `样例种类 ${kind} 没有对应的 TS 类型`).toBeDefined();
      seen.add(kind);
      checkShape(s, spec);

      if (s.type === 'progress') expect(['scan', 'pack', 'restore']).toContain(s.phase);
      if (s.type === 'log') expect(['info', 'warn', 'error']).toContain(s.level);
      if (s.type === 'error') expect(ERROR_CODES).toContain(s.code);
      if (kind === 'result/list') {
        for (const item of s.backups as Record<string, unknown>[]) checkShape(item, backupInfoSpec);
      }
    }
    expect([...seen].sort()).toEqual(Object.keys(specs).sort());
  });

  it('错误码与退出码与契约 §4 一致', () => {
    const exitOf: Record<string, number> = {};
    for (const line of section4(contract).split('\n')) {
      const row = /^\| (\d) \|/.exec(line);
      if (!row) continue;
      for (const m of line.matchAll(/`(E_[A-Z_]+)`/g)) exitOf[m[1]] = Number(row[1]);
    }
    expect(Object.keys(exitOf).sort()).toEqual([...ERROR_CODES].sort());
    expect(exitOf).toEqual(EXIT_CODE_OF);
  });

  it('无 error 事件时的退出码兜底映射与契约 §4 一致', () => {
    const sentence = section4(contract)
      .split('\n')
      .find((l) => l.includes('按退出码映射'));
    expect(sentence).toBeDefined();
    const fallback: Record<number, string> = {};
    for (const m of (sentence ?? '').matchAll(/(\d)[^，。\d`]*→ `(E_[A-Z_]+)`/g)) {
      fallback[Number(m[1])] = m[2];
    }
    expect(fallback).toEqual(FALLBACK_ERROR_CODE_OF_EXIT);
  });
});
