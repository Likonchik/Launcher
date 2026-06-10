'use client';

import { ButtonHTMLAttributes } from 'react';
import { Spinner } from './spinner';

type Variant = 'primary' | 'ghost' | 'danger';

const variants: Record<Variant, string> = {
  primary: 'bg-fg text-bg hover:opacity-90',
  ghost: 'border border-edge bg-surface hover:bg-surface-strong text-fg',
  danger: 'border border-danger/40 text-danger hover:bg-danger/10'
};

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  loading?: boolean;
};

export function Button({ variant = 'ghost', loading = false, className = '', children, disabled, ...rest }: Props) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 h-10 text-sm font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed ${variants[variant]} ${className}`}
    >
      {loading && <Spinner size={14} />}
      {children}
    </button>
  );
}

export function IconButton({ className = '', children, ...rest }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...rest}
      className={`inline-flex items-center justify-center rounded-lg w-9 h-9 border border-edge bg-surface hover:bg-surface-strong transition text-fg-secondary hover:text-fg disabled:opacity-50 ${className}`}
    >
      {children}
    </button>
  );
}
