/**
 * Incremental JSON Lines parser for the backend's stdout protocol.
 * One JSON object per line; blank lines are ignored; malformed lines are
 * surfaced as error events instead of throwing.
 */

export interface JsonlErrorEvent {
  type: 'error';
  message: string;
  line: string;
  [key: string]: unknown;
}

export type JsonlEvent = Record<string, unknown> | JsonlErrorEvent;

export interface JsonlParser {
  /** Feed a chunk of text; emits one event per complete line. */
  feed(chunk: string): void;
  /** Emit any trailing buffered text (on stream close). */
  flush(): void;
}

export interface LineSplitter {
  /** 追加一段文本；每凑齐一行（不含换行符，已去掉结尾 \r）回调一次，空白行跳过。 */
  feed(chunk: string): void;
  /** 流结束时输出缓冲区中没有换行结尾的最后一行。 */
  flush(): void;
}

/** 按 \n 切分文本流（跨 chunk 拼接），runner 与 createJsonlParser 共用。 */
export function createLineSplitter(onLine: (line: string) => void): LineSplitter {
  let buffer = '';

  const emit = (raw: string): void => {
    const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw;
    if (line.trim() !== '') onLine(line);
  };

  return {
    feed(chunk: string): void {
      buffer += chunk;
      let idx: number;
      while ((idx = buffer.indexOf('\n')) !== -1) {
        emit(buffer.slice(0, idx));
        buffer = buffer.slice(idx + 1);
      }
    },
    flush(): void {
      emit(buffer);
      buffer = '';
    },
  };
}

export function createJsonlParser(onEvent: (event: JsonlEvent) => void): JsonlParser {
  return createLineSplitter((line) => {
    try {
      onEvent(JSON.parse(line) as Record<string, unknown>);
    } catch (err) {
      onEvent({
        type: 'error',
        message: `invalid JSON line: ${(err as Error).message}`,
        line,
      });
    }
  });
}
