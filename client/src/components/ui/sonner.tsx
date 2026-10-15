import * as React from 'react';
import { Toaster as Sonner, type ToasterProps } from 'sonner';

/** Sonner 通知（单色风格：黑底白字细边框）。 */
function Toaster(props: ToasterProps): React.JSX.Element {
  return (
    <Sonner
      theme="dark"
      toastOptions={{
        style: {
          background: 'var(--color-surface-2)',
          border: '1px solid var(--color-border)',
          color: 'var(--color-text)',
        },
      }}
      {...props}
    />
  );
}

export { Toaster };
