import * as React from 'react';

import { cn } from '@/lib/utils';

function Input({ className, type, ...props }: React.ComponentProps<'input'>): React.JSX.Element {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'h-9 w-full min-w-0 rounded-[var(--radius-input)] border border-border bg-surface-2 px-3 py-1 text-sm text-text outline-none transition-colors placeholder:text-text-3 selection:bg-surface-3 focus-visible:border-border-strong disabled:pointer-events-none disabled:opacity-40',
        className,
      )}
      {...props}
    />
  );
}

export { Input };
