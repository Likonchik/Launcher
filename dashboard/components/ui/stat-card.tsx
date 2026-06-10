import { Card } from './card';

type Tone = 'default' | 'warn' | 'danger' | 'ok';

const tones: Record<Tone, string> = {
  default: 'text-fg',
  warn: 'text-warn',
  danger: 'text-danger',
  ok: 'text-ok'
};

export function StatCard({
  label,
  value,
  tone = 'default',
  hint
}: {
  label: string;
  value: string | number;
  tone?: Tone;
  hint?: string;
}) {
  return (
    <Card className="p-4">
      <div className="text-xs uppercase tracking-wide text-fg-muted">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${tones[tone]}`}>{value}</div>
      {hint && <div className="mt-0.5 text-xs text-fg-faint">{hint}</div>}
    </Card>
  );
}
