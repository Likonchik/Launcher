#!/usr/bin/env bash
#
# Ежедневная очистка диска на VPS (cron, 04:30).
# Убирает то, что накапливается само: build-кэш и висячие образы Docker,
# раздутые journald-журналы, apt-кэш и логи контейнеров крупнее 100 МБ.
# Запущенные контейнеры, тома и используемые образы не трогает.

set -euo pipefail

echo "=== cleanup $(date -Is) ==="
df -h / | tail -1

docker builder prune -af
docker image prune -af
journalctl --vacuum-size=200M

apt-get clean

# Логи контейнеров > 100 МБ — обнуляем (docker без log-rotation растёт бесконечно).
for log in /var/lib/docker/containers/*/*-json.log; do
  [ -f "$log" ] || continue
  size=$(stat -c%s "$log")
  if [ "$size" -gt $((100 * 1024 * 1024)) ]; then
    echo "truncate $log ($((size / 1024 / 1024)) MB)"
    truncate -s 0 "$log"
  fi
done

df -h / | tail -1
echo "=== done ==="
