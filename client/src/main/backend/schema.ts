import { z } from 'zod';
import {
  ERROR_CODES,
  type BackupInfo,
  type ErrorCode,
  type LogEvent,
  type ProgressEvent,
  type ResultEvent,
} from '../../shared/protocol';

/**
 * 后端 JSONL 事件的运行时校验，字段与 src/shared/protocol.ts（契约 §3）一一对应。
 * 对象默认剔除未知字段（契约 §5：客户端忽略未知字段）；error.code 先按字符串接收，
 * 未知错误码由 runner 按退出码归一化。
 */

const count = z.number().int().nonnegative();

const progressSchema = z.object({
  type: z.literal('progress'),
  phase: z.enum(['scan', 'pack', 'restore']),
  files: count,
  bytes: count,
  totalFiles: count.optional(),
  totalBytes: count.optional(),
  current: z.string(),
}) satisfies z.ZodType<ProgressEvent>;

const logSchema = z.object({
  type: z.literal('log'),
  level: z.enum(['info', 'warn', 'error']),
  message: z.string(),
}) satisfies z.ZodType<LogEvent>;

const rawErrorSchema = z.object({
  type: z.literal('error'),
  code: z.string(),
  message: z.string(),
});

const backupInfoShape = {
  name: z.string(),
  path: z.string(),
  outputBytes: count,
  createdAt: z.string(),
  source: z.string(),
  compressed: z.boolean(),
  encrypted: z.boolean(),
  files: count,
  dirs: count,
  bytes: count,
  formatVersion: z.number().int(),
  corrupt: z.boolean(),
};

const backupInfoSchema = z.object(backupInfoShape) satisfies z.ZodType<BackupInfo>;

const resultSchema = z.discriminatedUnion('command', [
  z.object({ type: z.literal('result'), command: z.literal('version'), version: z.string() }),
  z.object({
    type: z.literal('result'),
    command: z.literal('backup'),
    name: z.string(),
    path: z.string(),
    files: count,
    dirs: count,
    bytes: count,
    outputBytes: count,
    compressed: z.boolean(),
    encrypted: z.boolean(),
    skipped: count,
    warnings: count,
    ms: count,
  }),
  z.object({
    type: z.literal('result'),
    command: z.literal('dry-run'),
    files: count,
    dirs: count,
    bytes: count,
    skipped: count,
  }),
  z.object({
    type: z.literal('result'),
    command: z.literal('restore'),
    target: z.string(),
    files: count,
    dirs: count,
    bytes: count,
    warnings: count,
    ms: count,
  }),
  z.object({
    type: z.literal('result'),
    command: z.literal('list'),
    repo: z.string(),
    backups: z.array(backupInfoSchema),
  }),
  z.object({ type: z.literal('result'), command: z.literal('info'), ...backupInfoShape }),
  z.object({
    type: z.literal('result'),
    command: z.literal('delete'),
    name: z.string(),
    path: z.string(),
  }),
  z.object({ type: z.literal('result'), command: z.literal('config'), repo: z.string() }),
]) satisfies z.ZodType<ResultEvent>;

/** 后端原样输出的 error 事件（code 尚未归一化）。 */
export type RawErrorEvent = z.infer<typeof rawErrorSchema>;

/** 一行 stdout 的分类结果。 */
export type ParsedLine =
  | { kind: 'progress'; event: ProgressEvent }
  | { kind: 'log'; event: LogEvent }
  | { kind: 'error'; event: RawErrorEvent }
  | { kind: 'result'; event: ResultEvent }
  /** 未知事件类型：契约 §5 要求忽略。 */
  | { kind: 'unknown'; type: string }
  /** 不是 JSON 对象，或已知类型但字段不合法：协议错误。 */
  | { kind: 'invalid'; reason: string };

function invalid(type: string, error: z.ZodError): ParsedLine {
  const where = error.issues.map((i) => i.path.join('.') || '(root)').join(', ');
  return { kind: 'invalid', reason: `${type} 事件字段不合法：${where}` };
}

/** 解析并校验一行 JSONL。永不抛异常。 */
export function parseEventLine(line: string): ParsedLine {
  let value: unknown;
  try {
    value = JSON.parse(line);
  } catch {
    return { kind: 'invalid', reason: '不是合法的 JSON' };
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return { kind: 'invalid', reason: '不是 JSON 对象' };
  }
  const type = (value as { type?: unknown }).type;
  if (typeof type !== 'string') {
    return { kind: 'invalid', reason: '缺少 type 字段' };
  }
  switch (type) {
    case 'progress': {
      const r = progressSchema.safeParse(value);
      return r.success ? { kind: 'progress', event: r.data } : invalid(type, r.error);
    }
    case 'log': {
      const r = logSchema.safeParse(value);
      return r.success ? { kind: 'log', event: r.data } : invalid(type, r.error);
    }
    case 'error': {
      const r = rawErrorSchema.safeParse(value);
      return r.success ? { kind: 'error', event: r.data } : invalid(type, r.error);
    }
    case 'result': {
      const r = resultSchema.safeParse(value);
      return r.success ? { kind: 'result', event: r.data } : invalid(type, r.error);
    }
    default:
      return { kind: 'unknown', type };
  }
}

export function isErrorCode(code: string): code is ErrorCode {
  return (ERROR_CODES as readonly string[]).includes(code);
}
