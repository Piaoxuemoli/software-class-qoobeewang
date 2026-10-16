/** 界面共用的格式化辅助（等宽数字语境，见 ui-design.md §1.5）。 */

/** 字节数 → "1.5 MB"（二进制单位，小数至多 2 位）。 */
export function formatBytes(n: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = Math.round(n);
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  const s = i === 0 ? String(v) : v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2);
  return `${s} ${units[i]}`;
}

/** 千分位数字。 */
export function formatCount(n: number): string {
  return n.toLocaleString('en-US');
}

/** 毫秒 → "1.8 s" / "2.4 min"（摘要展示用）。 */
export function formatMs(ms: number): string {
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)} s`;
  return `${(s / 60).toFixed(1)} min`;
}

/** 秒 → "m:ss"（任务已用时间）。 */
export function formatElapsed(sec: number): string {
  const s = Math.max(0, Math.floor(sec));
  const m = Math.floor(s / 60);
  return `${m}:${String(s % 60).padStart(2, '0')}`;
}

/** RFC 3339 → 本地 "YYYY-MM-DD HH:MM"（列表与详情用）。 */
export function formatTime(rfc3339: string): string {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return rfc3339;
  const p = (n: number): string => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
