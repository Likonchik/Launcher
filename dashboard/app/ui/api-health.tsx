'use client';

import { useEffect, useMemo, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

type HealthState = {
  ok?: boolean;
  provider?: string;
  message?: string;
};

const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? 'http://127.0.0.1:8080';

export function ApiHealth() {
  const [health, setHealth] = useState<HealthState>({});
  const [isLoading, setIsLoading] = useState(true);

  const statusText = useMemo(() => {
    if (isLoading) {
      return 'Проверка';
    }
    if (health.ok) {
      return 'Online';
    }
    return 'Offline';
  }, [health.ok, isLoading]);

  async function loadHealth() {
    setIsLoading(true);
    try {
      const response = await fetch(`${apiUrl.replace(/\/$/, '')}/health`, {
        headers: { Accept: 'application/json' }
      });
      const data = (await response.json()) as HealthState;
      setHealth(response.ok ? data : { ok: false, message: `HTTP ${response.status}` });
    } catch {
      setHealth({ ok: false, message: 'Backend недоступен' });
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void loadHealth();
  }, []);

  return (
    <section className="health-panel glass" aria-label="API health">
      <div className="health-heading">
        <div className="health-icon">
          <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 22 }}>dns</span>
        </div>
        <div>
          <h2>Backend API</h2>
          <p>{apiUrl}</p>
        </div>
        <motion.button 
          type="button" 
          onClick={() => void loadHealth()} 
          title="Обновить статус"
          whileHover={{ scale: 1.1 }}
          whileTap={{ scale: 0.9 }}
          animate={{ rotate: isLoading ? 360 : 0 }}
          transition={{ rotate: { duration: 1, repeat: isLoading ? Infinity : 0, ease: "linear" } }}
        >
          <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>refresh</span>
        </motion.button>
      </div>

      <div className="health-status">
        <motion.span 
          className={health.ok ? 'dot online' : 'dot'} 
          animate={{ scale: health.ok ? [1, 1.3, 1] : 1, opacity: health.ok ? [1, 0.6, 1] : 1 }}
          transition={{ duration: 2, repeat: Infinity, ease: "easeInOut" }}
        />
        <strong>{statusText}</strong>
        <AnimatePresence mode="wait">
          <motion.span
            key={health.provider ?? health.message ?? 'Ожидание ответа'}
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 10 }}
            transition={{ duration: 0.2 }}
          >
            {health.provider ?? health.message ?? 'Ожидание ответа'}
          </motion.span>
        </AnimatePresence>
      </div>
    </section>
  );
}
