import * as React from 'react';

import { cn } from '@/lib/utils';

function Card({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return (
    <div
      data-slot="card"
      className={cn(
        'rounded-[var(--radius-card)] border border-border bg-surface-1 text-text',
        className,
      )}
      {...props}
    />
  );
}

function CardHeader({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return (
    <div data-slot="card-header" className={cn('flex flex-col gap-1 p-6', className)} {...props} />
  );
}

function CardTitle({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return (
    <div data-slot="card-title" className={cn('text-base font-medium', className)} {...props} />
  );
}

function CardDescription({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return (
    <div data-slot="card-description" className={cn('text-sm text-text-2', className)} {...props} />
  );
}

function CardContent({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return <div data-slot="card-content" className={cn('p-6 pt-0', className)} {...props} />;
}

function CardFooter({ className, ...props }: React.ComponentProps<'div'>): React.JSX.Element {
  return (
    <div
      data-slot="card-footer"
      className={cn('flex items-center p-6 pt-0', className)}
      {...props}
    />
  );
}

export { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter };
