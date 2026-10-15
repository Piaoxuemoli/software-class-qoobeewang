/**
 * 预加载脚本暴露给渲染进程的 window.databackup 接口（docs/client/interaction.md §2）。
 * 请求字段与 CLI 参数一一对应（见各字段注释）；主进程只做"字段 → 参数"的纯转换。
 * 除 window.* 与 jobs.onEvent 外，所有方法失败时 reject，错误格式见 formatApiError。
 */

import {
  ERROR_CODES,
  type BackendEvent,
  type BackupInfo,
  type ConfigResult,
  type DryRunResult,
  type ErrorCode,
  type FileTypeCode,
} from './protocol';

export type JobId = string;

/**
 * 客户端自身的错误码（不来自后端）：
 * E_BUSY 已有备份/还原任务在运行；E_BACKEND_UNAVAILABLE 后端二进制缺失或无法启动。
 */
export type ClientErrorCode = ErrorCode | 'E_BUSY' | 'E_BACKEND_UNAVAILABLE';

export interface ApiError {
  code: ClientErrorCode;
  message: string;
}

const CLIENT_ONLY_CODES: readonly string[] = ['E_BUSY', 'E_BACKEND_UNAVAILABLE'];

/**
 * window.databackup 的 Promise 失败时 reject 一个 Error，message 为 `[<code>] <说明>`。
 * Error 跨 contextBridge 会丢失自定义属性，所以错误码编码在 message 中。
 */
export function formatApiError(e: ApiError): string {
  return `[${e.code}] ${e.message}`;
}

/** 解析 formatApiError 的结果；无法识别时归为 E_INTERNAL。容忍 Electron 加在前面的前缀。 */
export function parseApiError(err: unknown): ApiError {
  const text = err instanceof Error ? err.message : String(err);
  const m = /\[(E_[A-Z_]+)\] ([\s\S]*)$/.exec(text);
  if (m) {
    const code = m[1];
    if ((ERROR_CODES as readonly string[]).includes(code) || CLIENT_ONLY_CODES.includes(code)) {
      return { code: code as ClientErrorCode, message: m[2] };
    }
  }
  return { code: 'E_INTERNAL', message: text };
}

/** 过滤条件，字段与 CLI 过滤参数一一对应（契约 §2.1）。省略的字段表示不过滤。 */
export interface BackupFilters {
  /** --types（逗号连接）。 */
  types?: FileTypeCode[];
  /** --exclude-dir（每项一次）；可为源目录内的绝对路径。 */
  excludeDirs?: string[];
  /** --exclude-name（每项一次），RE2 正则。 */
  excludeNames?: string[];
  /** --mtime-from / --mtime-to：'YYYY-MM-DD' 或 'YYYY-MM-DDTHH:MM:SS'。 */
  mtimeFrom?: string;
  mtimeTo?: string;
  /** --ctime-from / --ctime-to：格式同上。 */
  ctimeFrom?: string;
  ctimeTo?: string;
  /** --size-min / --size-max：字节，非负整数。 */
  sizeMin?: number;
  sizeMax?: number;
  /** --exclude-user / --exclude-group（每项一次），名称或数字 ID。 */
  excludeUsers?: string[];
  excludeGroups?: string[];
}

export interface BackupRequest {
  /** --source */
  source: string;
  /** --name；省略时后端生成默认名。 */
  name?: string;
  filters: BackupFilters;
  /** true → --compress huffman */
  compress: boolean;
  /** true → --encrypt aes256 --password-stdin（口令单独传入 start）。 */
  encrypt: boolean;
}

export interface RestoreRequest {
  /** --backup：备份名或路径。 */
  backup: string;
  /** --target */
  target: string;
}

export type DryRunSummary = Omit<DryRunResult, 'type' | 'command'>;

export type Config = Omit<ConfigResult, 'type' | 'command'>;

export type { BackupInfo };

export interface Versions {
  client: string;
  backend: string;
}

export interface DataBackupApi {
  app: { versions(): Promise<Versions> };
  window: {
    minimize(): void;
    toggleMaximize(): void;
    close(): void;
    onMaximizedChange(cb: (maximized: boolean) => void): () => void;
  };
  dialog: { pickDirectory(opts?: { title?: string }): Promise<string | null> };
  backup: {
    /** backup --dry-run；新调用会取消尚未完成的上一次。 */
    dryRun(req: BackupRequest): Promise<DryRunSummary>;
    /** req.encrypt 为 true 时必须提供 password（经 stdin 传给后端）。 */
    start(req: BackupRequest, password?: string): Promise<JobId>;
  };
  restore: {
    /** 加密备份必须提供 password。 */
    start(req: RestoreRequest, password?: string): Promise<JobId>;
  };
  jobs: {
    cancel(id: JobId): Promise<void>;
    /** 订阅任务事件（progress/log/result/error），返回取消订阅函数。 */
    onEvent(id: JobId, cb: (e: BackendEvent) => void): () => void;
  };
  repo: {
    list(): Promise<BackupInfo[]>;
    info(name: string): Promise<BackupInfo>;
    remove(name: string): Promise<void>;
  };
  config: {
    get(): Promise<Config>;
    set(patch: Partial<Config>): Promise<Config>;
  };
}
