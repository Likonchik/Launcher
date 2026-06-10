type Tone = 'default' | 'ok' | 'warn' | 'danger';

const tones: Record<Tone, string> = {
  default: 'text-fg-secondary border-edge',
  ok: 'text-ok border-ok/30',
  warn: 'text-warn border-warn/30',
  danger: 'text-danger border-danger/30'
};

export function Badge({
  tone = 'default',
  className = '',
  children
}: {
  tone?: Tone;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <span className={`inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs ${tones[tone]} ${className}`}>
      {children}
    </span>
  );
}
