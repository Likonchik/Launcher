'use client';

// Страница «Релизы»: список версий лаунчера + форма публикации новой.

import { useCallback, useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Rocket } from 'lucide-react';
import { EmptyState } from '../../components/ui/empty-state';
import { SkeletonTable } from '../../components/ui/skeleton';
import { useToast } from '../../components/ui/toast';
import { api, errorMessage } from '../lib/api';
import type { LauncherRelease } from '../lib/types';
import { ReleaseForm } from '../../components/releases/release-form';
import { ReleaseList } from '../../components/releases/release-list';

export default function ReleasesPage() {
  const toast = useToast();
  const [releases, setReleases] = useState<LauncherRelease[] | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await api<LauncherRelease[]>('/api/admin/releases/');
      setReleases(data);
    } catch (error) {
      toast('error', errorMessage(error));
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="grid items-start gap-5 lg:grid-cols-[380px_1fr]"
    >
      <ReleaseForm onCreated={() => void load()} />

      {releases === null ? (
        <SkeletonTable rows={4} cols={2} />
      ) : releases.length === 0 ? (
        <EmptyState
          icon={Rocket}
          title="Релизов пока нет"
          hint="Соберите лаунчер (scripts/prod/build-player-launcher.sh) и опубликуйте первый релиз — лаунчеры игроков обновятся автоматически."
        />
      ) : (
        <ReleaseList releases={releases} onChanged={() => void load()} />
      )}
    </motion.div>
  );
}
