/**
 * JSONL protocol events emitted by the backend on stdout.
 * Mirrors docs/architecture/cli-contract.md — keep in sync (compat rule:
 * additive changes only; unknown fields/event types must be ignored).
 */

export interface ProgressEvent {
  type: 'progress';
  phase: 'scan' | 'pack' | 'restore';
  files: number;
  bytes: number;
  current: string;
}

export interface LogEvent {
  type: 'log';
  level: 'info' | 'warn' | 'error';
  message: string;
}

export interface ResultEvent {
  type: 'result';
  command: string;
  [key: string]: unknown;
}

export interface ErrorEvent {
  type: 'error';
  code?: string;
  message: string;
}

export type BackendEvent = ProgressEvent | LogEvent | ResultEvent | ErrorEvent;
