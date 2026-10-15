import * as React from 'react';
import * as ScrollAreaPrimitive from '@radix-ui/react-scroll-area';

import { cn } from '@/lib/utils';

function ScrollArea({
  className,
  children,
  ...props
}: React.ComponentProps<typeof ScrollAreaPrimitive.Root>): React.JSX.Element {
  return (
    <ScrollAreaPrimitive.Root
      data-slot="scroll-area"
      className={cn('relative overflow-hidden', className)}
      {...props}
    >
      <ScrollAreaPrimitive.Viewport className="size-full rounded-[inherit]">
        {children}
      </ScrollAreaPrimitive.Viewport>
      <ScrollBar />
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}

function ScrollBar({
  className,
  orientation = 'vertical',
  ...props
}: React.ComponentProps<typeof ScrollAreaPrimitive.ScrollAreaScrollbar>): React.JSX.Element {
  return (
    <ScrollAreaPrimitive.ScrollAreaScrollbar
      data-slot="scroll-bar"
      orientation={orientation}
      className={cn(
        'flex touch-none select-none transition-colors',
        orientation === 'vertical' && 'h-full w-1.5',
        orientation === 'horizontal' && 'h-1.5 flex-col',
        className,
      )}
      {...props}
    >
      <ScrollAreaPrimitive.ScrollAreaThumb className="relative flex-1 rounded-full bg-surface-3" />
    </ScrollAreaPrimitive.ScrollAreaScrollbar>
  );
}

export { ScrollArea, ScrollBar };
