import { formatApiError, type ApiError, type ClientErrorCode } from '../shared/api';

/**
 * 主进程内部抛出的面向用户错误。message 已按 formatApiError 编码为 `[<code>] <说明>`，
 * 所以 ipcMain.handle 直接抛出即可被渲染进程的 parseApiError 还原。
 */
export class ApiFailure extends Error {
  readonly apiError: ApiError;

  constructor(apiError: ApiError) {
    super(formatApiError(apiError));
    this.name = 'ApiFailure';
    this.apiError = apiError;
  }
}

export function apiFailure(code: ClientErrorCode, message: string): ApiFailure {
  return new ApiFailure({ code, message });
}
