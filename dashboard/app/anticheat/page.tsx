'use client';

import { motion } from 'framer-motion';
import { AnticheatAdmin } from '../ui/anticheat-admin';

export default function AnticheatPage() {
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
          <h1>Античит</h1>
        </div>
        <motion.span className="env-pill glass" whileHover={{ scale: 1.05 }}>
          GravitGuard-style
        </motion.span>
      </motion.header>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.2 }}
      >
        <AnticheatAdmin />
      </motion.div>
    </section>
  );
}
