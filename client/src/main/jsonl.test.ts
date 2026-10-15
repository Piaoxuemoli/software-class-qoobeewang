// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { createJsonlParser, type JsonlEvent } from './jsonl';

function collect(): { events: JsonlEvent[]; feed: (s: string) => void; flush: () => void } {
  const events: JsonlEvent[] = [];
  const p = createJsonlParser((e) => events.push(e));
  return { events, feed: p.feed.bind(p), flush: p.flush.bind(p) };
}

describe('createJsonlParser', () => {
  it('parses one event per line', () => {
    const c = collect();
    c.feed('{"type":"result","version":"0.1.0"}\n{"type":"progress","n":1}\n');
    expect(c.events).toEqual([
      { type: 'result', version: '0.1.0' },
      { type: 'progress', n: 1 },
    ]);
  });

  it('reassembles events split across chunks', () => {
    const c = collect();
    c.feed('{"type":"re');
    c.feed('sult","ver');
    c.feed('sion":"0.1.0"}\n');
    expect(c.events).toEqual([{ type: 'result', version: '0.1.0' }]);
  });

  it('handles CRLF line endings', () => {
    const c = collect();
    c.feed('{"a":1}\r\n{"b":2}\r\n');
    expect(c.events).toEqual([{ a: 1 }, { b: 2 }]);
  });

  it('ignores blank lines', () => {
    const c = collect();
    c.feed('\n  \n{"a":1}\n\n');
    expect(c.events).toEqual([{ a: 1 }]);
  });

  it('surfaces malformed lines as error events', () => {
    const c = collect();
    c.feed('not json\n{"ok":true}\n');
    expect(c.events).toHaveLength(2);
    expect(c.events[0]).toMatchObject({ type: 'error', line: 'not json' });
    expect(c.events[1]).toEqual({ ok: true });
  });

  it('flushes a trailing line without newline', () => {
    const c = collect();
    c.feed('{"a":1}');
    expect(c.events).toHaveLength(0);
    c.flush();
    expect(c.events).toEqual([{ a: 1 }]);
  });
});
