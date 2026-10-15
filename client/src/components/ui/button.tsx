import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-full text-sm font-medium transition-colors outline-none disabled:pointer-events-none disabled:opacity-40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 focus-visible:border-border-strong",
  {
    variants: {
      variant: {
        default: 'bg-accent text-black hover:bg-accent/90',
        secondary: 'bg-surface-2 text-text hover:bg-surface-3',
        ghost: 'text-text-2 hover:bg-surface-2 hover:text-text',
        destructive: 'bg-danger text-white hover:bg-danger/90',
        outline:
          'border border-border bg-transparent text-text hover:border-border-strong hover:bg-surface-2',
      },
      size: {
        default: 'h-9 px-5',
        sm: 'h-7 px-3 text-xs',
        lg: 'h-11 px-6',
        icon: 'size-8 rounded-md',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean }): React.JSX.Element {
  const Comp = asChild ? Slot : 'button';
  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
