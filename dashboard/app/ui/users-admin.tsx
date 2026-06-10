'use client';

import { FormEvent, useCallback, useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { api, getToken } from './auth';

type Stats = {
  totalUsers: number;
  telegramLinked: number;
  bannedUsers: number;
  hwidBanned: number;
  newUsers7d: number;
  authSuccess24h: number;
  authFailure24h: number;
};

type UserItem = {
  id: string;
  login: string;
  providerUuid: string;
  email: string;
  role: string;
  isBanned: boolean;
  isHwidBanned: boolean;
  totpEnabled: boolean;
  telegramId?: number | null;
  telegramUsername?: string;
  createdAt: string;
  lastLoginAt?: string | null;
};

type AuthLog = { id: number; username: string; ip: string; source: string; success: boolean; message: string; createdAt: string };
type AuditLog = { id: string; action: string; details: string; createdAt: string };

type Detail = { user: UserItem; authLogs: AuthLog[]; auditLogs: AuditLog[] };

const ROLES = ['user', 'moderator', 'admin'];

function errorMessage(e: unknown) {
  return e instanceof Error ? e.message : 'Неизвестная ошибка';
}

export function UsersAdmin() {
  const [token, setTok] = useState<string | null>(null);
  const [message, setMessage] = useState('Загрузка…');
  const [stats, setStats] = useState<Stats | null>(null);
  const [users, setUsers] = useState<UserItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<Detail | null>(null);

  useEffect(() => {
    const saved = getToken();
    if (!saved) {
      setMessage('Требуется авторизация администратора (войдите на странице профилей).');
      return;
    }
    setTok(saved);
  }, []);

  const loadStats = useCallback(async (t: string) => {
    try {
      setStats(await api<Stats>('/api/admin/stats', { token: t }));
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }, []);

  const loadUsers = useCallback(async (t: string, q: string, p: number) => {
    try {
      const res = await api<{ items: UserItem[]; total: number }>(
        `/api/admin/users?q=${encodeURIComponent(q)}&page=${p}`,
        { token: t }
      );
      setUsers(res.items ?? []);
      setTotal(res.total ?? 0);
      setMessage('');
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }, []);

  useEffect(() => {
    if (!token) return;
    void loadStats(token);
    void loadUsers(token, query, page);
  }, [token, page, loadStats, loadUsers]); // query применяется по сабмиту

  async function openDetail(id: string) {
    if (!token) return;
    try {
      const d = await api<Detail>(`/api/admin/users/${id}`, { token });
      setSelected(d);
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }

  function onSearch(e: FormEvent) {
    e.preventDefault();
    setPage(1);
    if (token) void loadUsers(token, query, 1);
  }

  async function act(path: string, method: string, body?: unknown) {
    if (!token || !selected) return;
    try {
      await api(path, { method, token, body });
      await openDetail(selected.user.id);
      await loadUsers(token, query, page);
      await loadStats(token);
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }

  async function deleteUser() {
    if (!token || !selected) return;
    if (!window.confirm(`Удалить пользователя ${selected.user.login}? Действие необратимо.`)) return;
    try {
      await api(`/api/admin/users/${selected.user.id}`, { method: 'DELETE', token });
      setSelected(null);
      await loadUsers(token, query, page);
      await loadStats(token);
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }

  if (!token) {
    return <div className="admin-panel glass" style={{ padding: 24 }}>{message}</div>;
  }

  const totalPages = Math.max(1, Math.ceil(total / 30));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {stats && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 12 }}>
          <StatCard label="Всего" value={stats.totalUsers} />
          <StatCard label="С Telegram" value={stats.telegramLinked} />
          <StatCard label="Баны" value={stats.bannedUsers} />
          <StatCard label="HWID-баны" value={stats.hwidBanned} />
          <StatCard label="Новых за 7д" value={stats.newUsers7d} />
          <StatCard label="Входы 24ч" value={stats.authSuccess24h} />
          <StatCard label="Ошибки 24ч" value={stats.authFailure24h} />
        </div>
      )}

      <form onSubmit={onSearch} style={{ display: 'flex', gap: 8 }}>
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Поиск: логин, e-mail, uuid"
          className="glass"
          style={{ flex: 1, padding: '10px 14px', borderRadius: 8, border: 'none', color: '#fff', background: 'rgba(255,255,255,0.05)' }}
        />
        <button type="submit" className="nav-item glass" style={{ cursor: 'pointer', border: 'none' }}>Искать</button>
      </form>

      {message && <p style={{ color: '#fbbf24' }}>{message}</p>}

      <div className="admin-panel glass" style={{ padding: 0, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left', color: '#a1a1aa' }}>
              <th style={th}>Логин</th>
              <th style={th}>E-mail</th>
              <th style={th}>Роль</th>
              <th style={th}>TG</th>
              <th style={th}>Флаги</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} onClick={() => openDetail(u.id)} style={{ cursor: 'pointer', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
                <td style={td}>{u.login}</td>
                <td style={td}>{u.email}</td>
                <td style={td}>{u.role}</td>
                <td style={td}>{u.telegramId ? '✓' : '—'}</td>
                <td style={td}>
                  {u.isBanned && <span title="бан">⛔</span>}
                  {u.isHwidBanned && <span title="hwid-бан">🚫</span>}
                  {u.totpEnabled && <span title="2FA">🔐</span>}
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td style={td} colSpan={5}>Ничего не найдено</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', justifyContent: 'center' }}>
        <button type="button" className="nav-item glass" disabled={page <= 1} onClick={() => setPage((p) => p - 1)} style={pagerBtn}>←</button>
        <span style={{ color: '#a1a1aa' }}>{page} / {totalPages}</span>
        <button type="button" className="nav-item glass" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)} style={pagerBtn}>→</button>
      </div>

      {selected && (
        <motion.div
          className="admin-panel glass"
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 20 }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <h2 style={{ margin: 0 }}>{selected.user.login}</h2>
            <button type="button" onClick={() => setSelected(null)} className="nav-item glass" style={{ cursor: 'pointer', border: 'none' }}>Закрыть</button>
          </div>

          <div>
            <h3 style={subhead}>Аккаунт</h3>
            <Row k="UUID" v={selected.user.providerUuid} />
            <Row k="E-mail" v={selected.user.email} />
            <Row k="Создан" v={new Date(selected.user.createdAt).toLocaleString()} />
            <Row k="Telegram" v={selected.user.telegramId ? `${selected.user.telegramId} ${selected.user.telegramUsername ? '@' + selected.user.telegramUsername : ''}` : '—'} />
            <Row k="2FA" v={selected.user.totpEnabled ? 'включена' : 'выключена'} />

            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 12 }}>
              <label style={{ color: '#a1a1aa' }}>Роль:</label>
              <select
                value={selected.user.role}
                onChange={(e) => act(`/api/admin/users/${selected.user.id}/role`, 'PATCH', { role: e.target.value })}
                className="glass"
                style={{ padding: '6px 10px', borderRadius: 8, border: 'none', color: '#fff', background: 'rgba(255,255,255,0.05)' }}
              >
                {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 12 }}>
              <ActionBtn label={selected.user.isBanned ? 'Разбанить' : 'Забанить'}
                onClick={() => act(`/api/admin/users/${selected.user.id}/${selected.user.isBanned ? 'unban' : 'ban'}`, 'POST')} />
              <ActionBtn label={selected.user.isHwidBanned ? 'Снять HWID-бан' : 'HWID-бан'}
                onClick={() => act(`/api/admin/users/${selected.user.id}/${selected.user.isHwidBanned ? 'hwid-unban' : 'hwid-ban'}`, 'POST')} />
              <ActionBtn label="Удалить" danger onClick={deleteUser} />
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
            <div>
              <h3 style={subhead}>Журнал входов</h3>
              <LogList rows={selected.authLogs.map((l) => `${new Date(l.createdAt).toLocaleString()} · ${l.source} · ${l.success ? 'ok' : 'fail'} · ${l.message}`)} />
            </div>
            <div>
              <h3 style={subhead}>Действия администраторов</h3>
              <LogList rows={selected.auditLogs.map((l) => `${new Date(l.createdAt).toLocaleString()} · ${l.action} · ${l.details}`)} />
            </div>
          </div>
        </motion.div>
      )}
    </div>
  );
}

const th: React.CSSProperties = { padding: '12px 16px', fontWeight: 500 };
const td: React.CSSProperties = { padding: '10px 16px', color: '#e4e4e7' };
const subhead: React.CSSProperties = { color: '#fff', marginTop: 0 };
const pagerBtn: React.CSSProperties = { cursor: 'pointer', border: 'none', minWidth: 44 };

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="glass" style={{ padding: '14px 16px', borderRadius: 12 }}>
      <div style={{ fontSize: 24, fontWeight: 600, color: '#fff' }}>{value}</div>
      <div style={{ fontSize: 12, color: '#a1a1aa' }}>{label}</div>
    </div>
  );
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: '4px 0', fontSize: 14 }}>
      <span style={{ color: '#a1a1aa' }}>{k}</span>
      <span style={{ color: '#e4e4e7', textAlign: 'right', wordBreak: 'break-all' }}>{v}</span>
    </div>
  );
}

function ActionBtn({ label, onClick, danger }: { label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button type="button" onClick={onClick} className="nav-item glass"
      style={{ cursor: 'pointer', border: 'none', color: danger ? '#f87171' : undefined }}>
      {label}
    </button>
  );
}

function LogList({ rows }: { rows: string[] }) {
  if (rows.length === 0) return <p style={{ color: '#a1a1aa', fontSize: 13 }}>Пусто</p>;
  return (
    <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: '#a1a1aa', maxHeight: 220, overflowY: 'auto' }}>
      {rows.map((r, i) => <li key={i} style={{ wordBreak: 'break-all' }}>{r}</li>)}
    </ul>
  );
}
