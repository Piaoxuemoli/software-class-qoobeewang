import { useEffect, useRef, useState } from 'react';

function reduceMotion(): boolean {
  if (document.documentElement.dataset.motion === 'reduce') return true;
  try {
    return matchMedia('(prefers-reduced-motion: reduce)').matches;
  } catch {
    return false;
  }
}

/**
 * 数字滚动（ui-design.md §6 / §7 300ms）：数值变化时 300ms 内平滑过渡；
 * 减少动效时直接跳到目标值。
 */
export function Counter({
  value,
  format = (n: number): string => n.toLocaleString('en-US'),
  className,
}: {
  value: number;
  format?: (n: number) => string;
  className?: string;
}): React.JSX.Element {
  const [shown, setShown] = useState(value);
  const from = useRef(value);
  const raf = useRef(0);

  useEffect(() => {
    if (reduceMotion() || from.current === value) {
      from.current = value;
      setShown(value);
      return;
    }
    const start = performance.now();
    const prev = from.current;
    const tick = (now: number): void => {
      const t = Math.min(1, (now - start) / 300);
      const cur = prev + (value - prev) * (1 - Math.pow(1 - t, 3));
      setShown(cur);
      if (t < 1) raf.current = requestAnimationFrame(tick);
      else from.current = value;
    };
    raf.current = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(raf.current);
      from.current = shown;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  return (
    <span className={className} data-testid="counter">
      {format(shown)}
    </span>
  );
}
