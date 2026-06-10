'use client';

import { motion } from 'framer-motion';

export type TabItem = { key: string; label: string; badge?: number };

export function Tabs({
  items,
  active,
  onChange
}: {
  items: TabItem[];
  active: string;
  onChange: (key: string) => void;
}) {
  return (
    <div className="inline-flex items-center gap-1 rounded-lg border border-edge bg-surface p-1">
      {items.map((item) => {
        const isActive = item.key === active;
        return (
          <button
            key={item.key}
            onClick={() => onChange(item.key)}
            className={`relative rounded-md px-3.5 h-8 text-sm font-medium transition ${
              isActive ? 'text-bg' : 'text-fg-secondary hover:text-fg'
            }`}
          >
            {isActive && (
              <motion.span
                layoutId="tab-pill"
                className="absolute inset-0 rounded-md bg-fg"
                transition={{ type: 'spring', stiffness: 500, damping: 35 }}
              />
            )}
            <span className="relative z-10 flex items-center gap-1.5">
              {item.label}
              {item.badge !== undefined && item.badge > 0 && (
                <span className={`rounded px-1 text-xs ${isActive ? 'bg-bg/15' : 'bg-surface-strong'}`}>{item.badge}</span>
              )}
            </span>
          </button>
        );
      })}
    </div>
  );
}
