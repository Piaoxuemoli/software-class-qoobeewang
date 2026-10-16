import type { BackupFilters, BackupRequest, RestoreRequest } from '../../shared/api';

/**
 * 请求字段 → CLI 参数的纯函数转换（契约 §2；interaction.md §2）。
 * 只做一一映射，不做任何业务判断：取值是否合法（正则、时间格式、区间方向、用户名……）
 * 全部交给后端校验并以 E_USAGE 返回。返回值不含 --json（由 runner 统一添加）。
 */

function pushValue(out: string[], flag: string, value: string | number | undefined): void {
  if (value !== undefined) out.push(flag, String(value));
}

function pushEach(out: string[], flag: string, values: readonly string[] | undefined): void {
  for (const v of values ?? []) out.push(flag, v);
}

export function filterArgs(f: BackupFilters): string[] {
  const out: string[] = [];
  if (f.types !== undefined) out.push('--types', f.types.join(','));
  pushEach(out, '--exclude-dir', f.excludeDirs);
  pushEach(out, '--exclude-name', f.excludeNames);
  pushValue(out, '--mtime-from', f.mtimeFrom);
  pushValue(out, '--mtime-to', f.mtimeTo);
  pushValue(out, '--ctime-from', f.ctimeFrom);
  pushValue(out, '--ctime-to', f.ctimeTo);
  pushValue(out, '--size-min', f.sizeMin);
  pushValue(out, '--size-max', f.sizeMax);
  pushEach(out, '--exclude-user', f.excludeUsers);
  pushEach(out, '--exclude-group', f.excludeGroups);
  return out;
}

/** backup：加密时追加 --encrypt aes256 --password-stdin，口令由 runner 写入 stdin。 */
export function backupArgs(req: BackupRequest): string[] {
  const out = ['backup', '--source', req.source];
  pushValue(out, '--name', req.name);
  out.push(...filterArgs(req.filters));
  if (req.compress) out.push('--compress', 'huffman');
  if (req.encrypt) out.push('--encrypt', 'aes256', '--password-stdin');
  return out;
}

/** backup --dry-run：只需源目录与过滤条件；name/compress/encrypt 后端会忽略，不传，也不需要口令。 */
export function dryRunArgs(req: BackupRequest): string[] {
  return ['backup', '--dry-run', '--source', req.source, ...filterArgs(req.filters)];
}

export function restoreArgs(req: RestoreRequest, withPassword: boolean): string[] {
  const out = ['restore', '--backup', req.backup, '--target', req.target];
  if (withPassword) out.push('--password-stdin');
  return out;
}

export const versionArgs = (): string[] => ['--version'];
export const listArgs = (): string[] => ['list'];
export const infoArgs = (name: string): string[] => ['info', name];
export const deleteArgs = (name: string): string[] => ['delete', name];
export const configGetArgs = (): string[] => ['config', 'get'];
export const configSetArgs = (key: 'repo', value: string): string[] => [
  'config',
  'set',
  key,
  value,
];
