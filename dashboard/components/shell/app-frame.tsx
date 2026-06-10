'use client';

import { ReactNode, useEffect, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { api, clearToken, getToken } from '../../app/lib/api';
import type { AuthUser } from '../../app/lib/types';
import { ToastProvider } from '../ui/toast';
import { ConfirmProvider } from '../ui/confirm';
import { Spinner } from '../ui/spinner';
import { Sidebar } from './sidebar';
import { Topbar } from './topbar';
import { CommandPalette } from './command-palette';

/**
 * Оболочка дашборда с guard'ом авторизации. /login рендерится без shell и проверки.
 * Остальные страницы — только для role=admin: иначе редирект на /login.
 */
export function AppFrame({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);

  const isLogin = pathname === '/login';

  useEffect(() => {
    if (isLogin) {
      setReady(true);
      return;
    }
    const token = getToken();
    if (!token) {
      router.replace('/login');
      return;
    }
    let cancelled = false;
    api<AuthUser>('/api/auth/me')
      .then((u) => {
        if (cancelled) return;
        if (u.role !== 'admin') {
          clearToken();
          router.replace('/login');
          return;
        }
        setUser(u);
        setReady(true);
      })
      .catch(() => {
        if (cancelled) return;
        clearToken();
        router.replace('/login');
      });
    return () => {
      cancelled = true;
    };
  }, [pathname, isLogin, router]);

  // Глобальный хоткей палитры.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setPaletteOpen((v) => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  if (isLogin) {
    return (
      <ToastProvider>
        <ConfirmProvider>{children}</ConfirmProvider>
      </ToastProvider>
    );
  }

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center gap-2 text-sm text-fg-muted">
        <Spinner size={16} /> Проверка доступа…
      </div>
    );
  }

  return (
    <ToastProvider>
      <ConfirmProvider>
        <div className="min-h-screen">
          <div className="max-[819px]:hidden">
            <Sidebar />
          </div>
          {mobileNav && (
            <div className="min-[820px]:hidden">
              <Sidebar forceOpen onNavigate={() => setMobileNav(false)} />
            </div>
          )}
          <div className="min-[820px]:pl-16">
            <Topbar
              user={user}
              onOpenPalette={() => setPaletteOpen(true)}
              onToggleMobileNav={() => setMobileNav((v) => !v)}
            />
            <main className="mx-auto w-full max-w-6xl p-6 max-[560px]:p-4">{children}</main>
          </div>
          <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
        </div>
      </ConfirmProvider>
    </ToastProvider>
  );
}
