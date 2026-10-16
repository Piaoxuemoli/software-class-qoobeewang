import { z } from 'zod';
import type { BackupFilters, BackupRequest, Config, RestoreRequest } from '../../shared/api';

/**
 * 渲染进程入参的 zod 校验（interaction.md §7）。只校验"形状"——类型、必填、
 * 不含 NUL 等会破坏参数传递的字符；取值语义（正则、时间格式、区间……）由后端校验。
 * 校验失败的提示不得回显入参内容（口令也走这里）。
 */

/** 与 Linux PATH_MAX 同量级的单个字符串上限，防止超长入参。 */
const MAX_TEXT = 4096;

const noNul = (s: string): boolean => !s.includes('\0');

const text = z.string().max(MAX_TEXT).refine(noNul, '不能包含 NUL 字符');
const required = text.refine((s) => s.length > 0, '不能为空');
/** 作为位置参数传给 CLI 的值：以 - 开头会被当成参数解析。 */
const positional = required.refine((s) => !s.startsWith('-'), '不能以 - 开头');

export const backupFiltersSchema = z.strictObject({
  types: z.array(z.enum(['f', 'd', 'l', 'c', 'b', 's', 'p'])).optional(),
  excludeDirs: z.array(text).optional(),
  excludeNames: z.array(text).optional(),
  mtimeFrom: text.optional(),
  mtimeTo: text.optional(),
  ctimeFrom: text.optional(),
  ctimeTo: text.optional(),
  sizeMin: z.number().int().nonnegative().optional(),
  sizeMax: z.number().int().nonnegative().optional(),
  excludeUsers: z.array(text).optional(),
  excludeGroups: z.array(text).optional(),
}) satisfies z.ZodType<BackupFilters>;

export const backupRequestSchema = z.strictObject({
  source: required,
  name: required.optional(),
  filters: backupFiltersSchema,
  compress: z.boolean(),
  encrypt: z.boolean(),
}) satisfies z.ZodType<BackupRequest>;

export const restoreRequestSchema = z.strictObject({
  backup: required,
  target: required,
}) satisfies z.ZodType<RestoreRequest>;

/** 后端读到第一个 \n 为止，口令里的换行会被静默截断，必须拒绝。 */
export const passwordSchema = z
  .string()
  .max(MAX_TEXT)
  .refine((s) => !/[\r\n\0]/.test(s), '口令不能包含换行或 NUL 字符');

export const backupNameSchema = positional;

export const jobIdSchema = z.string().min(1).max(128);

export const configPatchSchema = z.strictObject({
  repo: positional.optional(),
}) satisfies z.ZodType<Partial<Config>>;

export const pickDirectoryOptsSchema = z
  .strictObject({ title: z.string().max(200).optional() })
  .optional();

/** 各 IPC 通道的参数元组。 */
export const argsSchemas = {
  none: z.tuple([]),
  pickDirectory: z.tuple([pickDirectoryOptsSchema]),
  dryRun: z.tuple([backupRequestSchema]),
  backupStart: z.tuple([backupRequestSchema, passwordSchema.optional()]),
  restoreStart: z.tuple([restoreRequestSchema, passwordSchema.optional()]),
  jobId: z.tuple([jobIdSchema]),
  backupName: z.tuple([backupNameSchema]),
  configPatch: z.tuple([configPatchSchema]),
} as const;
