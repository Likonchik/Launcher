'use client';

// Таблица релизов лаунчера: платформы, флаги «обязательный»/«активен»,
// переключение флагов и удаление.

import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card } from '../ui/card';
import { useConfirm } from '../ui/confirm';
import { useToast } from '../ui/toast';
import { api, errorMessage } from '../../app/lib/api';
import type { LauncherRelease } from '../../app/lib/types';

function formatSize(bytes: number): string {
  return `${(bytes / 1024 / 1024).toFixed(1)} МБ`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('ru-RU', { dateStyle: 'medium', timeStyle: 'short' });
}

export function ReleaseList({
  releases,
  onChanged
}: {
  releases: LauncherRelease[];
  onChanged: () => void;
}) {
  const toast = useToast();
  const confirm = useConfirm();

  async function patch(release: LauncherRelease, body: { mandatory?: boolean; isActive?: boolean }) {
    try {
      await api(`/api/admin/releases/${release.id}`, { method: 'PATCH', body });
      onChanged();
    } catch (error) {
      toast('error', errorMessage(error));
    }
  }

  async function remove(release: LauncherRelease) {
    const ok = await confirm({
      title: `Удалить релиз ${release.version}?`,
      message: 'Бинарники будут удалены с диска. Лаунчеры перестанут видеть эту версию.',
      confirmLabel: 'Удалить',
      danger: true
    });
    if (!ok) return;
    try {
      await api(`/api/admin/releases/${release.id}`, { method: 'DELETE' });
      toast('success', `Релиз ${release.version} удалён`);
      onChanged();
    } catch (error) {
      toast('error', errorMessage(error));
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {releases.map((release) => (
        <Card key={release.id} className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-base font-bold">{release.version}</span>
            {release.mandatory && <Badge tone="warn">обязательный</Badge>}
            <Badge tone={release.isActive ? 'ok' : 'danger'}>
              {release.isActive ? 'активен' : 'снят с раздачи'}
            </Badge>
            <span className="ml-auto text-xs text-fg-faint">{formatDate(release.createdAt)}</span>
          </div>

          {release.changelog && (
            <p className="whitespace-pre-wrap text-sm text-fg-secondary">{release.changelog}</p>
          )}

          <div className="flex flex-wrap gap-2">
            {release.files.map((file) => (
              <Badge key={file.id}>
                {file.platform} • {formatSize(file.size)}
              </Badge>
            ))}
          </div>

          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void patch(release, { mandatory: !release.mandatory })}>
              {release.mandatory ? 'Сделать необязательным' : 'Сделать обязательным'}
            </Button>
            <Button onClick={() => void patch(release, { isActive: !release.isActive })}>
              {release.isActive ? 'Снять с раздачи' : 'Вернуть в раздачу'}
            </Button>
            <Button variant="danger" onClick={() => void remove(release)}>
              Удалить
            </Button>
          </div>
        </Card>
      ))}
    </div>
  );
}
