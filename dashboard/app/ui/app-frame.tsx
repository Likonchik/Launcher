'use client';

import { ReactNode, useEffect, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { Sidebar } from './sidebar';
import { AuthUser, api, clearToken, getToken } from './auth';

/**
 * Оболочка дашборда с guard'ом авторизации. Страница /login рендерится без сайдбара
 * и без проверки. Остальные страницы доступны только админу: при отсутствии/невалидном
 * токене — редирект на /login.
 */
export function AppFrame({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [ready, setReady] = useState(false);

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
    api<AuthUser>('/api/auth/me', { token })
      .then((u) => {
        if (cancelled) return;
        if (u.role !== 'admin') {
          clearToken();
          router.replace('/login');
          return;
        }
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

  if (isLogin) {
    return <>{children}</>;
  }
  if (!ready) {
    return (
      <div className="dashboard-shell">
        <main className="content">
          <p className="panel-message">Проверка доступа…</p>
        </main>
      </div>
    );
  }
  return (
    <div className="dashboard-shell">
      <Sidebar />
      {children}
    </div>
  );
}
