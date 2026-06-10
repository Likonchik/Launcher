import type { NextConfig } from 'next';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const dashboardRoot = dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  outputFileTracingRoot: dashboardRoot,
  poweredByHeader: false,
  // Компактный самодостаточный билд для прод/Docker (.next/standalone).
  output: 'standalone'
};

export default nextConfig;
