'use client';

import { motion } from 'framer-motion';

export default function NewsPage() {
  return (
    <section className="content">
      <motion.header
        className="page-header"
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.1 }}
      >
        <div>
          <p className="eyebrow">Project launcher</p>
          <h1>Новости</h1>
        </div>
        <motion.span 
          className="env-pill glass"
          whileHover={{ scale: 1.05 }}
        >
          Beta
        </motion.span>
      </motion.header>

      <motion.div
        className="admin-panel glass"
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.5, delay: 0.2 }}
        style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '400px', textAlign: 'center' }}
      >
        <motion.span 
          className="material-symbols-rounded" 
          style={{ fontSize: 64, color: '#ffffff', textShadow: '0 0 20px rgba(255,255,255,0.5)', marginBottom: 20 }}
          animate={{ rotate: [0, -10, 10, -10, 10, 0] }}
          transition={{ duration: 1.5, repeat: Infinity, repeatDelay: 2 }}
        >
          campaign
        </motion.span>
        <h2>Страница в разработке</h2>
        <p style={{ color: '#a1a1aa' }}>Управление новостями появится в следующем обновлении.</p>
      </motion.div>
    </section>
  );
}
