import * as React from 'react';
import * as SwitchPrimitive from '@radix-ui/react-switch';

import { cn } from '@/lib/utils';

function Switch({
  className,
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root>): React.JSX.Element {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'peer inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-border bg-surface-3 outline-none transition-colors data-[state=checked]:border-accent data-[state=checked]:bg-accent focus-visible:border-border-strong disabled:cursor-not-allowed disabled:opacity-40',
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block size-3.5 translate-x-0.5 rounded-full bg-text-2 transition-transform data-[state=checked]:translate-x-[18px] data-[state=checked]:bg-black"
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
