import type { LucideIcon } from 'lucide-react';

export function EmptyState({
  icon: Icon,
  title,
  hint,
  action
}: {
  icon?: LucideIcon;
  title: string;
  hint?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-edge px-6 py-14 text-center">
      {Icon && <Icon size={28} className="text-fg-faint" />}
      <div className="text-sm font-semibold text-fg-secondary">{title}</div>
      {hint && <div className="max-w-sm text-xs text-fg-faint">{hint}</div>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
