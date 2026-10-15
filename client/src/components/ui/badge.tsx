import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex w-fit items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium [&_svg]:size-3',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-accent text-black',
        secondary: 'border-border bg-surface-2 text-text-2',
        success: 'border-transparent bg-success/15 text-success',
        danger: 'border-transparent bg-danger/15 text-danger',
        warning: 'border-transparent bg-warning/15 text-warning',
        outline: 'border-border text-text-2',
      },
    },
    defaultVariants: { variant: 'secondary' },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants>): React.JSX.Element {
  return (
    <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge, badgeVariants };
