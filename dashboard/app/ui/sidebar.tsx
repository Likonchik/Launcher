'use client';

import { motion } from 'framer-motion';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { clearToken } from './auth';

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();

  function logout() {
    clearToken();
    router.replace('/login');
  }

  const navItems = [
    { name: 'Обзор', path: '/', icon: 'show_chart' },
    { name: 'Профили', path: '/profiles', icon: 'category' },
    { name: 'Пользователи', path: '/users', icon: 'group' },
    { name: 'Новости', path: '/news', icon: 'description' },
    { name: 'Античит', path: '/anticheat', icon: 'shield' },
  ];

  return (
    <motion.aside
      className="sidebar glass"
      aria-label="Admin navigation"
      initial={{ x: -50, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      transition={{ duration: 0.5, ease: 'easeOut' }}
    >
      <div className="brand-lockup">
        <motion.div 
          className="brand-mark"
          whileHover={{ scale: 1.05, rotate: 5 }}
          whileTap={{ scale: 0.95 }}
        >
          LL
        </motion.div>
        <div>
          <strong>Launcher</strong>
          <span>Admin</span>
        </div>
      </div>

      <nav className="nav-list">
        {navItems.map((item) => {
          const isActive = pathname === item.path;
          return (
            <Link key={item.path} href={item.path} className={`nav-item glass ${isActive ? 'active' : ''}`} style={{ position: 'relative' }}>
              {isActive && (
                <motion.div
                  layoutId="activeNavIndicator"
                  style={{ position: 'absolute', inset: 0, background: 'rgba(255,255,255,0.05)', borderRadius: 8, zIndex: -1 }}
                  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                />
              )}
              <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>
                {item.icon}
              </span>
              {item.name}
            </Link>
          );
        })}
      </nav>

      <button type="button" className="nav-item glass" onClick={logout} style={{ marginTop: 'auto', cursor: 'pointer', border: 'none', textAlign: 'left', width: '100%' }}>
        <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>
          logout
        </span>
        Выйти
      </button>
    </motion.aside>
  );
}
