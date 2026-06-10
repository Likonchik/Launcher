'use client';

import { useEffect, useState } from 'react';
import { usePathname } from 'next/navigation';
import { Menu, Search } from 'lucide-react';
import { apiUrl } from '../../app/lib/api';
import { navItems } from './sidebar';
import type { AuthUser } from '../../app/lib/types';

/** Топбар: заголовок раздела, поиск (Ctrl+K), живой статус API, инициал админа. */
export function Topbar({
  user,
  onOpenPalette,
  onToggleMobileNav
}: {
  user: AuthUser | null;
  onOpenPalette: () => void;
  onToggleMobileNav: () => void;
}) {
  const pathname = usePathname();
  const [apiOk, setApiOk] = useState<boolean | null>(null);

  const title = navItems.find((item) => item.href === pathname)?.label ?? 'PJM Admin';

  useEffect(() => {
    let cancelled = false;
    const check = () =>
      fetch(`${apiUrl}/health`)
        .then((r) => !cancelled && setApiOk(r.ok))
        .catch(() => !cancelled && setApiOk(false));
    check();
    const timer = setInterval(check, 30000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  return (
    <header className="sticky top-0 z-20 flex h-14 items-center gap-4 border-b border-edge bg-bg/80 px-5 backdrop-blur-xl">
      <button onClick={onToggleMobileNav} className="text-fg-secondary transition hover:text-fg min-[820px]:hidden">
        <Menu size={20} />
      </button>
      <h1 className="text-sm font-bold">{title}</h1>

      <div className="ml-auto flex items-center gap-3">
        <button
          onClick={onOpenPalette}
          className="flex h-8 items-center gap-2 rounded-lg border border-edge bg-surface px-3 text-xs text-fg-faint transition hover:border-edge-strong hover:text-fg-secondary"
        >
          <Search size={13} />
          <span className="max-[560px]:hidden">Поиск</span>
          <kbd className="rounded border border-edge bg-bg px-1.5 py-0.5 text-[10px] max-[560px]:hidden">Ctrl K</kbd>
        </button>

        <div className="flex items-center gap-1.5 text-xs text-fg-muted" title="Статус backend API">
          <span
            className={`h-2 w-2 rounded-full ${apiOk === null ? 'bg-fg-faint' : apiOk ? 'bg-ok' : 'bg-danger'}`}
          />
          <span className="max-[560px]:hidden">{apiOk === null ? 'API…' : apiOk ? 'API online' : 'API offline'}</span>
        </div>

        {user && (
          <div
            className="flex h-8 w-8 items-center justify-center rounded-full border border-edge bg-surface text-xs font-bold uppercase"
            title={user.login}
          >
            {user.login.slice(0, 1)}
          </div>
        )}
      </div>
    </header>
  );
}
