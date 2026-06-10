'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { motion } from 'framer-motion';
import { AuthUser, api, getToken, setToken } from '../ui/auth';

export default function LoginPage() {
  const router = useRouter();
  const [form, setForm] = useState({ login: '', password: '', totp: '' });
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  // Если уже авторизованы — сразу внутрь.
  useEffect(() => {
    const t = getToken();
    if (!t) return;
    api<AuthUser>('/api/auth/me', { token: t })
      .then((u) => {
        if (u.role === 'admin') router.replace('/');
      })
      .catch(() => {});
  }, [router]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      const res = await api<{ token: string; user: AuthUser }>('/api/auth/login', {
        method: 'POST',
        body: { login: form.login, password: form.password, totp: form.totp || undefined }
      });
      if (res.user.role !== 'admin') {
        throw new Error('Аккаунт не имеет прав администратора');
      }
      setToken(res.token);
      router.replace('/');
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Ошибка входа');
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-screen">
      <motion.form
        onSubmit={submit}
        className="admin-panel glass login-card"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <div className="brand-lockup" style={{ marginBottom: 8 }}>
          <div className="brand-mark">LL</div>
          <div>
            <strong>Launcher</strong>
            <span>Admin</span>
          </div>
        </div>
        <p className="eyebrow">Вход в панель</p>
        <h2 style={{ marginTop: 0 }}>Авторизация</h2>

        <label>
          <span>Логин</span>
          <input
            autoFocus
            value={form.login}
            onChange={(e) => setForm({ ...form, login: e.target.value })}
            placeholder="nickname"
          />
        </label>
        <label>
          <span>Пароль</span>
          <input
            type="password"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            placeholder="••••••••"
          />
        </label>
        <label>
          <span>2FA (если включён)</span>
          <input
            value={form.totp}
            onChange={(e) => setForm({ ...form, totp: e.target.value })}
            placeholder="000000"
            inputMode="numeric"
          />
        </label>

        <button type="submit" className="primary-button" disabled={busy} style={{ marginTop: 12 }}>
          {busy ? 'Вход…' : 'Войти'}
        </button>

        {message && <p className="panel-message" style={{ color: '#f87171' }}>{message}</p>}
      </motion.form>
    </main>
  );
}
