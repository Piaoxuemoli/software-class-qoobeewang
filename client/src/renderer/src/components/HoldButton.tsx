import { useRef, useState } from 'react';
import { cn } from '@/lib/utils';

const HOLD_MS = 1200;

/**
 * 长按确认按钮（interaction.md §3：删除需长按 1200ms）。
 * 按住时底色从左向右填充，填满触发 onConfirm；提前松手不生效。
 */
export function HoldButton({
  onConfirm,
  children,
  disabled,
  className,
}: {
  onConfirm: () => void;
  children: React.ReactNode;
  disabled?: boolean;
  className?: string;
}): React.JSX.Element {
  const [holding, setHolding] = useState(false);
  const timer = useRef<number | null>(null);

  const release = (): void => {
    if (timer.current !== null) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    setHolding(false);
  };

  const press = (e: React.PointerEvent): void => {
    if (disabled) return;
    e.preventDefault();
    setHolding(true);
    timer.current = window.setTimeout(() => {
      timer.current = null;
      setHolding(false);
      onConfirm();
    }, HOLD_MS);
  };

  return (
    <button
      type="button"
      disabled={disabled}
      aria-label={`长按${HOLD_MS / 1000}秒${typeof children === 'string' ? children : '确认'}`}
      className={cn(
        'relative overflow-hidden rounded-full border border-border px-4 py-2 text-[13px]',
        'text-danger transition-colors disabled:opacity-40',
        className,
      )}
      onPointerDown={press}
      onPointerUp={release}
      onPointerLeave={release}
      onPointerCancel={release}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') press(e as unknown as React.PointerEvent);
      }}
      onKeyUp={release}
    >
      <span
        className="absolute inset-y-0 left-0 bg-danger/20"
        style={{
          width: holding ? '100%' : '0%',
          transition: holding ? `width ${HOLD_MS}ms linear` : 'width 120ms',
        }}
      />
      <span className="relative">{holding ? '按住不放…' : children}</span>
    </button>
  );
}
