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

export function createJsonlParser(onEvent: (event: JsonlEvent) => void): JsonlParser {
  let buffer = '';

  const emitLine = (raw: string): void => {
    const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw;
    if (line.trim() === '') return;
    try {
      onEvent(JSON.parse(line) as Record<string, unknown>);
    } catch (err) {
      onEvent({
        type: 'error',
        message: `invalid JSON line: ${(err as Error).message}`,
        line,
      });
    }
  };

  return {
    feed(chunk: string): void {
      buffer += chunk;
      let idx: number;
      while ((idx = buffer.indexOf('\n')) !== -1) {
        emitLine(buffer.slice(0, idx));
        buffer = buffer.slice(idx + 1);
      }
    },
    flush(): void {
      emitLine(buffer);
      buffer = '';
    },
  };
}
