import { HTMLAttributes } from 'react';

export function Card({ className = '', children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...rest} className={`rounded-xl border border-edge bg-surface backdrop-blur-xl p-5 ${className}`}>
      {children}
    </div>
  );
}
