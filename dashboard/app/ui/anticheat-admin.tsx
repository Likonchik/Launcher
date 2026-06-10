'use client';

import { FormEvent, useCallback, useEffect, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';

const apiUrl = (process.env.NEXT_PUBLIC_API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
const tokenKey = 'launcher.admin.token';

type AuthUser = { id: string; login: string; role: string };

type Detection = {
  id: string;
  userUuid: string;
  login: string;
  hwidHash: string;
  source: string;
  type: string;
  signature: string;
  severity: number;
  createdAt: string;
};

type HwidBan = { id: string; hwidHash: string; reason: string; bannedBy: string; createdAt: string };
type AccountBan = { id: string; userUuid: string; login: string; reason: string; bannedBy: string; createdAt: string };
type CheatSignature = {
  id: string;
  kind: string;
  pattern: string;
  hashHex: string;
  severity: number;
  note: string;
  enabled: boolean;
};

type ApiError = { message?: string };

type Tab = 'detections' | 'bans' | 'signatures';

async function request<T = unknown>(
  path: string,
  options: { method?: string; token?: string; body?: unknown } = {}
): Promise<T> {
  const response = await fetch(`${apiUrl}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      Accept: 'application/json',
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.token ? { Authorization: `Bearer ${options.token}` } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  });
  if (response.status === 204) {
    return undefined as T;
  }
  const data = (await response.json().catch(() => ({}))) as ApiError & T;
  if (!response.ok) {
    throw new Error(data.message ?? `HTTP ${response.status}`);
  }
  return data;
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Неизвестная ошибка';
}

function severityColor(severity: number) {
  if (severity >= 8) return '#f87171';
  if (severity >= 5) return '#fbbf24';
  return '#a1a1aa';
}

export function AnticheatAdmin() {
  const [token, setToken] = useState('');
  const [user, setUser] = useState<AuthUser | null>(null);
  const [tab, setTab] = useState<Tab>('detections');
  const [message, setMessage] = useState('Загрузка…');

  const [detections, setDetections] = useState<Detection[]>([]);
  const [hwidBans, setHwidBans] = useState<HwidBan[]>([]);
  const [accountBans, setAccountBans] = useState<AccountBan[]>([]);
  const [signatures, setSignatures] = useState<CheatSignature[]>([]);

  const [newSig, setNewSig] = useState({ kind: 'process', pattern: '', hashHex: '', severity: 5, note: '' });

  useEffect(() => {
    const saved = localStorage.getItem(tokenKey);
    if (!saved) {
      setMessage('Требуется авторизация администратора (войдите на странице профилей).');
      return;
    }
    setToken(saved);
    request<AuthUser>('/api/auth/me', { token: saved })
      .then((u) => {
        if (u.role !== 'admin') throw new Error('Аккаунт не является администратором');
        setUser(u);
        setMessage(`Администратор: ${u.login}`);
      })
      .catch((e) => {
        setMessage(errorMessage(e));
      });
  }, []);

  const reload = useCallback(async () => {
    if (!token) return;
    try {
      const [det, hb, ab, sigs] = await Promise.all([
        request<Detection[]>('/api/admin/anticheat/detections?limit=200', { token }),
        request<HwidBan[]>('/api/admin/anticheat/bans/hwid', { token }),
        request<AccountBan[]>('/api/admin/anticheat/bans/account', { token }),
        request<CheatSignature[]>('/api/admin/anticheat/signatures', { token })
      ]);
      setDetections(det ?? []);
      setHwidBans(hb ?? []);
      setAccountBans(ab ?? []);
      setSignatures(sigs ?? []);
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }, [token]);

  useEffect(() => {
    if (user) void reload();
  }, [user, reload]);

  async function banHwidFromDetection(hwidHash: string) {
    if (!hwidHash) return;
    try {
      await request('/api/admin/anticheat/bans/hwid', { method: 'POST', token, body: { hwidHash, reason: 'manual from detection' } });
      await reload();
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }

  async function banAccountFromDetection(d: Detection) {
    try {
      await request('/api/admin/anticheat/bans/account', { method: 'POST', token, body: { userUuid: d.userUuid, login: d.login, reason: `manual: ${d.signature}` } });
      await reload();
    } catch (e) {
      setMessage(errorMessage(e));
    }
  }

  async function unbanHwid(hash: string) {
    await request(`/api/admin/anticheat/bans/hwid/${encodeURIComponent(hash)}`, { method: 'DELETE', token });
    await reload();
  }

  async function unbanAccount(uuid: string) {
    await request(`/api/admin/anticheat/bans/account/${encodeURIComponent(uuid)}`, { method: 'DELETE', token });
    await reload();
  }

  async function createSignature(e: FormEvent) {
    e.preventDefault();
    try {
      await request('/api/admin/anticheat/signatures', { method: 'POST', token, body: { ...newSig, severity: Number(newSig.severity), enabled: true } });
      setNewSig({ kind: 'process', pattern: '', hashHex: '', severity: 5, note: '' });
      await reload();
    } catch (err) {
      setMessage(errorMessage(err));
    }
  }

  async function toggleSignature(sig: CheatSignature) {
    await request(`/api/admin/anticheat/signatures/${sig.id}`, { method: 'PATCH', token, body: { enabled: !sig.enabled } });
    await reload();
  }

  async function deleteSignature(id: string) {
    await request(`/api/admin/anticheat/signatures/${id}`, { method: 'DELETE', token });
    await reload();
  }

  if (!user) {
    return (
      <section className="admin-panel glass">
        <p className="panel-message">{message}</p>
      </section>
    );
  }

  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'detections', label: 'Детекты', count: detections.length },
    { id: 'bans', label: 'Баны', count: hwidBans.length + accountBans.length },
    { id: 'signatures', label: 'Сигнатуры', count: signatures.length }
  ];

  const stats = {
    total: detections.length,
    high: detections.filter((d) => d.severity >= 8).length,
    accounts: new Set(detections.map((d) => d.userUuid)).size
  };

  return (
    <section className="admin-panel glass" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div className="panel-heading">
        <div>
          <p className="eyebrow">Anticheat</p>
          <h2>Обнаружения и баны</h2>
        </div>
        <button className="secondary-button" onClick={() => void reload()}>Обновить</button>
      </div>

      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        <StatCard label="Детектов (last 200)" value={stats.total} />
        <StatCard label="Критичных (sev≥8)" value={stats.high} color="#f87171" />
        <StatCard label="Затронуто аккаунтов" value={stats.accounts} />
        <StatCard label="Активных HWID-банов" value={hwidBans.length} />
        <StatCard label="Активных банов аккаунтов" value={accountBans.length} />
      </div>

      <div style={{ display: 'flex', gap: 8 }}>
        {tabs.map((t) => (
          <button
            key={t.id}
            className={`secondary-button ${tab === t.id ? 'active' : ''}`}
            onClick={() => setTab(t.id)}
            style={tab === t.id ? { background: 'rgba(255,255,255,0.1)' } : undefined}
          >
            {t.label} ({t.count})
          </button>
        ))}
      </div>

      <AnimatePresence mode="wait">
        {tab === 'detections' && (
          <motion.div key="det" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            {detections.length === 0 && <p className="panel-message">Детектов пока нет.</p>}
            {detections.map((d) => (
              <div key={d.id} className="glass" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 12, borderRadius: 8, marginBottom: 8 }}>
                <div>
                  <strong style={{ color: severityColor(d.severity) }}>[{d.severity}] {d.type}</strong>
                  <span style={{ marginLeft: 8 }}>{d.signature}</span>
                  <div style={{ fontSize: '0.85rem', color: '#a1a1aa' }}>
                    {d.login || d.userUuid} · {d.source} · {new Date(d.createdAt).toLocaleString('ru-RU')}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <button className="secondary-button" onClick={() => void banAccountFromDetection(d)}>Бан акк</button>
                  {d.hwidHash && <button className="secondary-button" onClick={() => void banHwidFromDetection(d.hwidHash)}>Бан HWID</button>}
                </div>
              </div>
            ))}
          </motion.div>
        )}

        {tab === 'bans' && (
          <motion.div key="bans" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <h3>Аккаунты</h3>
            {accountBans.length === 0 && <p className="panel-message">Нет банов аккаунтов.</p>}
            {accountBans.map((b) => (
              <div key={b.id} className="glass" style={{ display: 'flex', justifyContent: 'space-between', padding: 10, borderRadius: 8, marginBottom: 6 }}>
                <span>{b.login || b.userUuid} — {b.reason || 'без причины'} <em style={{ color: '#a1a1aa' }}>({b.bannedBy})</em></span>
                <button className="secondary-button" onClick={() => void unbanAccount(b.userUuid)}>Разбан</button>
              </div>
            ))}
            <h3 style={{ marginTop: 16 }}>Устройства (HWID)</h3>
            {hwidBans.length === 0 && <p className="panel-message">Нет HWID-банов.</p>}
            {hwidBans.map((b) => (
              <div key={b.id} className="glass" style={{ display: 'flex', justifyContent: 'space-between', padding: 10, borderRadius: 8, marginBottom: 6 }}>
                <span style={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{b.hwidHash.slice(0, 24)}… — {b.reason || 'без причины'}</span>
                <button className="secondary-button" onClick={() => void unbanHwid(b.hwidHash)}>Разбан</button>
              </div>
            ))}
          </motion.div>
        )}

        {tab === 'signatures' && (
          <motion.div key="sig" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <form onSubmit={createSignature} className="form-grid" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'flex-end', marginBottom: 16 }}>
              <label>
                <span>Тип</span>
                <select value={newSig.kind} onChange={(e) => setNewSig({ ...newSig, kind: e.target.value })}>
                  <option value="process">process</option>
                  <option value="class">class</option>
                  <option value="jar">jar</option>
                  <option value="file">file</option>
                </select>
              </label>
              <label>
                <span>Паттерн (имя/подстрока)</span>
                <input value={newSig.pattern} onChange={(e) => setNewSig({ ...newSig, pattern: e.target.value })} placeholder="cheatengine" />
              </label>
              <label>
                <span>SHA-256 (опц.)</span>
                <input value={newSig.hashHex} onChange={(e) => setNewSig({ ...newSig, hashHex: e.target.value })} placeholder="hex" />
              </label>
              <label>
                <span>Severity</span>
                <input type="number" min={1} max={10} value={newSig.severity} onChange={(e) => setNewSig({ ...newSig, severity: Number(e.target.value) })} />
              </label>
              <button type="submit" className="secondary-button">Добавить</button>
            </form>
            {signatures.length === 0 && <p className="panel-message">Блэклист пуст.</p>}
            {signatures.map((s) => (
              <div key={s.id} className="glass" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 10, borderRadius: 8, marginBottom: 6, opacity: s.enabled ? 1 : 0.5 }}>
                <span>
                  <strong>{s.kind}</strong> · {s.pattern || s.hashHex.slice(0, 16)} · sev {s.severity}
                </span>
                <div style={{ display: 'flex', gap: 6 }}>
                  <button className="secondary-button" onClick={() => void toggleSignature(s)}>{s.enabled ? 'Выкл' : 'Вкл'}</button>
                  <button className="secondary-button" onClick={() => void deleteSignature(s.id)}>Удалить</button>
                </div>
              </div>
            ))}
          </motion.div>
        )}
      </AnimatePresence>

      <p className="panel-message">{message}</p>
    </section>
  );
}

function StatCard({ label, value, color }: { label: string; value: number; color?: string }) {
  return (
    <div className="glass" style={{ padding: '12px 16px', borderRadius: 8, minWidth: 120 }}>
      <div style={{ fontSize: '0.8rem', color: '#a1a1aa' }}>{label}</div>
      <strong style={{ fontSize: '1.6rem', color: color ?? '#fff' }}>{value}</strong>
    </div>
  );
}
