import * as React from 'react';
import * as ProgressPrimitive from '@radix-ui/react-progress';

import { cn } from '@/lib/utils';

function Progress({
  className,
  value,
  ...props
}: React.ComponentProps<typeof ProgressPrimitive.Root>): React.JSX.Element {
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      className={cn('relative h-1 w-full overflow-hidden rounded-full bg-surface-3', className)}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className="h-full flex-1 bg-accent transition-transform"
        style={{ transform: `translateX(-${100 - (value ?? 0)}%)` }}
      />
    </ProgressPrimitive.Root>
  );
}

export { Progress };
