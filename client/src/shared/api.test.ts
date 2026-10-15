import { describe, expect, it } from 'vitest';
import { formatApiError, parseApiError } from './api';

describe('API 错误编码', () => {
  it('format → parse 往返', () => {
    const e = { code: 'E_NOT_FOUND' as const, message: '备份不存在：old' };
    expect(parseApiError(new Error(formatApiError(e)))).toEqual(e);
  });

  it('容忍 Electron 加在前面的前缀', () => {
    const err = new Error(
      "Error invoking remote method 'repo:info': Error: [E_CORRUPT] 备份文件已损坏",
    );
    expect(parseApiError(err)).toEqual({ code: 'E_CORRUPT', message: '备份文件已损坏' });
  });

  it('识别客户端自身错误码', () => {
    expect(parseApiError(new Error('[E_BUSY] 已有任务在运行')).code).toBe('E_BUSY');
  });

  it('无法识别时归为 E_INTERNAL', () => {
    expect(parseApiError(new Error('boom'))).toEqual({ code: 'E_INTERNAL', message: 'boom' });
    expect(parseApiError(new Error('[E_NOPE] x')).code).toBe('E_INTERNAL');
    expect(parseApiError('plain').message).toBe('plain');
  });
});
