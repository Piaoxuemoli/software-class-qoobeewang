/**
 * 后端 --json 模式下 stdout 的 JSONL 线上类型。
 * 与 docs/architecture/cli-contract.md §3、§4 以及 backend/internal/jsonl/events.go 一一对应，
 * 由 contract.test.ts 校验。修改须走"契约"PR（只增不改；客户端忽略未知字段与事件）。
 */

export type FileTypeCode = 'f' | 'd' | 'l' | 'c' | 'b' | 's' | 'p';

/** 按 --types 字母顺序列出全部文件类型。 */
export const FILE_TYPE_CODES: readonly FileTypeCode[] = ['f', 'd', 'l', 'c', 'b', 's', 'p'];

export type Phase = 'scan' | 'pack' | 'restore';

export type LogLevel = 'info' | 'warn' | 'error';

export const ERROR_CODES = [
  'E_IO',
  'E_NOT_FOUND',
  'E_TARGET_NOT_EMPTY',
  'E_CORRUPT',
  'E_EXISTS',
  'E_INTERNAL',
  'E_USAGE',
  'E_AUTH',
  'E_PASSWORD_REQUIRED',
  'E_CANCELLED',
] as const;

export type ErrorCode = (typeof ERROR_CODES)[number];

/** 每个错误码对应的进程退出码（契约 §4）。 */
export const EXIT_CODE_OF: Readonly<Record<ErrorCode, number>> = {
  E_IO: 1,
  E_NOT_FOUND: 1,
  E_TARGET_NOT_EMPTY: 1,
  E_CORRUPT: 1,
  E_EXISTS: 1,
  E_INTERNAL: 1,
  E_USAGE: 2,
  E_AUTH: 3,
  E_PASSWORD_REQUIRED: 3,
  E_CANCELLED: 4,
};

/**
 * 非零退出却没有 error 事件时（被 SIGKILL、崩溃）按退出码推断错误码（契约 §4）。
 * 被信号终止的进程一律视为 E_CANCELLED。
 */
export const FALLBACK_ERROR_CODE_OF_EXIT: Readonly<Record<number, ErrorCode>> = {
  1: 'E_INTERNAL',
  2: 'E_USAGE',
  3: 'E_AUTH',
  4: 'E_CANCELLED',
};

export interface ProgressEvent {
  type: 'progress';
  phase: Phase;
  /** 已处理的非目录项数。 */
  files: number;
  /** 已处理的普通文件原始字节。 */
  bytes: number;
  /** 只在 pack、restore 阶段出现。 */
  totalFiles?: number;
  totalBytes?: number;
  /** 当前项相对路径。 */
  current: string;
}

export interface LogEvent {
  type: 'log';
  level: LogLevel;
  message: string;
}

export interface ErrorEvent {
  type: 'error';
  /** 未知错误码由调用层（runner）按退出码归一化后再交给界面。 */
  code: ErrorCode;
  message: string;
}

export interface VersionResult {
  type: 'result';
  command: 'version';
  version: string;
}

export interface BackupResult {
  type: 'result';
  command: 'backup';
  name: string;
  path: string;
  files: number;
  dirs: number;
  bytes: number;
  outputBytes: number;
  compressed: boolean;
  encrypted: boolean;
  skipped: number;
  warnings: number;
  ms: number;
}

export interface DryRunResult {
  type: 'result';
  command: 'dry-run';
  files: number;
  dirs: number;
  bytes: number;
  skipped: number;
}

export interface RestoreResult {
  type: 'result';
  command: 'restore';
  target: string;
  files: number;
  dirs: number;
  bytes: number;
  warnings: number;
  ms: number;
}

/** 仓库中的一个备份（list 的数组项，info 的平铺字段）。 */
export interface BackupInfo {
  name: string;
  path: string;
  outputBytes: number;
  /** RFC 3339；corrupt 时为文件修改时间。 */
  createdAt: string;
  /** 备份时源目录绝对路径。 */
  source: string;
  compressed: boolean;
  encrypted: boolean;
  files: number;
  dirs: number;
  bytes: number;
  formatVersion: number;
  corrupt: boolean;
}

export interface ListResult {
  type: 'result';
  command: 'list';
  repo: string;
  backups: BackupInfo[];
}

export interface InfoResult extends BackupInfo {
  type: 'result';
  command: 'info';
}

export interface DeleteResult {
  type: 'result';
  command: 'delete';
  name: string;
  path: string;
}

export interface ConfigResult {
  type: 'result';
  command: 'config';
  repo: string;
}

export type ResultEvent =
  | VersionResult
  | BackupResult
  | DryRunResult
  | RestoreResult
  | ListResult
  | InfoResult
  | DeleteResult
  | ConfigResult;

export type BackendEvent = ProgressEvent | LogEvent | ErrorEvent | ResultEvent;
