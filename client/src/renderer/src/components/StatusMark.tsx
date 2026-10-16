import { cn } from '@/lib/utils';

export type StatusMarkKind = 'idle' | 'running' | 'success' | 'error' | 'warning';

const COLORS: Record<StatusMarkKind, string> = {
  idle: 'var(--text-3)',
  running: 'var(--accent)',
  success: 'var(--success)',
  error: 'var(--danger)',
  warning: 'var(--warning)',
};

/**
 * 任务状态图标（ui-design.md §6）：圆环 → 进度弧 → 对勾 / 叉。
 * running 态用 SVG 旋转弧线表示进行中；--motion 关闭时全局样式会停掉动画。
 */
export function StatusMark({
  kind,
  size = 20,
  className,
}: {
  kind: StatusMarkKind;
  size?: number;
  className?: string;
}): React.JSX.Element {
  const c = COLORS[kind];
  return (
    <span
      className={cn('inline-flex shrink-0 items-center justify-center', className)}
      aria-label={
        { idle: '空闲', running: '进行中', success: '完成', error: '失败', warning: '警告' }[kind]
      }
      data-testid={`status-${kind}`}
    >
      {kind === 'running' ? (
        <svg width={size} height={size} viewBox="0 0 20 20" className="status-spin">
          <circle cx="10" cy="10" r="8" fill="none" stroke="var(--border-strong)" strokeWidth="2" />
          <circle
            cx="10"
            cy="10"
            r="8"
            fill="none"
            stroke={c}
            strokeWidth="2"
            strokeLinecap="round"
            strokeDasharray="28 22"
          />
          <style>{`.status-spin{animation:spin 1s linear infinite;transform-origin:center}@keyframes spin{to{transform:rotate(360deg)}}`}</style>
        </svg>
      ) : (
        <svg width={size} height={size} viewBox="0 0 20 20">
          <circle cx="10" cy="10" r="8" fill="none" stroke={c} strokeWidth="2" />
          {kind === 'success' && (
            <path
              d="M6.5 10.2l2.4 2.4 4.6-5"
              fill="none"
              stroke={c}
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          )}
          {kind === 'error' && (
            <path
              d="M7 7l6 6M13 7l-6 6"
              fill="none"
              stroke={c}
              strokeWidth="2"
              strokeLinecap="round"
            />
          )}
          {kind === 'warning' && (
            <path d="M10 6.4v4.4" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" />
          )}
          {kind === 'warning' && <circle cx="10" cy="13.8" r="1.1" fill={c} />}
        </svg>
      )}
    </span>
  );
}
