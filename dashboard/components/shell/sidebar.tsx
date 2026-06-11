'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { AnimatePresence, motion } from 'framer-motion';
import { LayoutDashboard, LogOut, Newspaper, Package, Pickaxe, Rocket, Shield, Users } from 'lucide-react';
import { clearToken } from '../../app/lib/api';

export const navItems = [
  { href: '/', label: 'Обзор', icon: LayoutDashboard },
  { href: '/profiles', label: 'Профили', icon: Package },
  { href: '/users', label: 'Пользователи', icon: Users },
  { href: '/news', label: 'Новости', icon: Newspaper },
  { href: '/releases', label: 'Релизы', icon: Rocket },
  { href: '/anticheat', label: 'Античит', icon: Shield }
];

/**
 * Сайдбар-рейка 64px: при наведении плавно раскрывается до 240px поверх контента
 * (контент не сдвигается — у main постоянный pl-16).
 */
export function Sidebar({ forceOpen = false, onNavigate }: { forceOpen?: boolean; onNavigate?: () => void }) {
  const pathname = usePathname();
  const router = useRouter();
  const [hovered, setHovered] = useState(false);
  const expanded = forceOpen || hovered;

  const logout = () => {
    clearToken();
    router.replace('/login');
  };

  return (
    <>
      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="pointer-events-none fixed inset-0 z-30 bg-black/30"
          />
        )}
      </AnimatePresence>
      <motion.nav
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        animate={{ width: expanded ? 240 : 64 }}
        transition={{ type: 'spring', stiffness: 400, damping: 36 }}
        className={`fixed inset-y-0 left-0 z-40 flex flex-col overflow-hidden border-r border-edge ${
          expanded ? 'bg-bg/90 shadow-2xl backdrop-blur-2xl' : 'bg-bg/60 backdrop-blur-xl'
        }`}
      >
        <div className="flex h-14 items-center gap-3 border-b border-edge px-[18px]">
          <Pickaxe size={22} className="shrink-0 text-fg" />
          <AnimatePresence>
            {expanded && (
              <motion.span
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="whitespace-nowrap text-sm font-bold"
              >
                PJM Admin
              </motion.span>
            )}
          </AnimatePresence>
        </div>

        <div className="flex flex-1 flex-col gap-1 p-3">
          {navItems.map((item) => {
            const active = pathname === item.href;
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={onNavigate}
                className={`relative flex h-10 items-center gap-3 rounded-lg px-2.5 text-sm font-medium transition ${
                  active ? 'text-bg' : 'text-fg-secondary hover:text-fg'
                }`}
              >
                {active && (
                  <motion.span
                    layoutId="nav-pill"
                    className="absolute inset-0 rounded-lg bg-fg"
                    transition={{ type: 'spring', stiffness: 500, damping: 35 }}
                  />
                )}
                <Icon size={18} className="relative z-10 shrink-0" />
                <AnimatePresence>
                  {expanded && (
                    <motion.span
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      className="relative z-10 whitespace-nowrap"
                    >
                      {item.label}
                    </motion.span>
                  )}
                </AnimatePresence>
              </Link>
            );
          })}
        </div>

        <div className="border-t border-edge p-3">
          <button
            onClick={logout}
            className="flex h-10 w-full items-center gap-3 rounded-lg px-2.5 text-sm font-medium text-fg-secondary transition hover:bg-surface hover:text-fg"
          >
            <LogOut size={18} className="shrink-0" />
            <AnimatePresence>
              {expanded && (
                <motion.span
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="whitespace-nowrap"
                >
                  Выйти
                </motion.span>
              )}
            </AnimatePresence>
          </button>
        </div>
      </motion.nav>
    </>
  );
}
