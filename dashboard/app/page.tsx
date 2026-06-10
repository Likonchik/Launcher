'use client';

import { motion } from 'framer-motion';
import { ApiHealth } from './ui/api-health';

const metrics = [
  { label: 'Профили', value: 'API', icon: 'category' },
  { label: 'Пользователи', value: 'JWT', icon: 'group' },
  { label: 'Новости', value: 'Later', icon: 'description' }
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1
    }
  }
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.4 } }
};

export default function DashboardHome() {
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
          <h1>Обзор системы</h1>
        </div>
        <motion.span 
          className="env-pill glass glow-active"
          whileHover={{ scale: 1.05 }}
        >
          Live
        </motion.span>
      </motion.header>

      <motion.section
        className="metrics-grid"
        aria-label="Dashboard metrics"
        variants={containerVariants}
        initial="hidden"
        animate="visible"
      >
        {metrics.map((metric) => (
          <motion.article 
            className="metric-card glass" 
            key={metric.label}
            variants={itemVariants}
          >
            <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 24, textShadow: '0 0 10px rgba(255,255,255,0.4)' }}>
              {metric.icon}
            </span>
            <span>{metric.label}</span>
            <strong style={{ textShadow: '0 0 10px rgba(255,255,255,0.2)' }}>{metric.value}</strong>
          </motion.article>
        ))}
      </motion.section>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.3 }}
      >
        <ApiHealth />
      </motion.div>
    </section>
  );
}
