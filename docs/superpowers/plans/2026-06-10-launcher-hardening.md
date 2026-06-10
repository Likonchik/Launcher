# Launcher Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть критичные риски прода: бэкапы БД, rate limiting, fail-fast на дев-секретах, переживающие рестарт yggdrasil-сессии, health checks, CI, корректные таймауты скачивания в лаунчере.

**Architecture:** Точечные правки в существующих компонентах без новых сервисов: Go-бэкенд (Fiber v3 + GORM), Rust-лаунчер (reqwest blocking), docker compose, bash-скрипты, GitHub Actions. Yggdrasil-сессии остаются в памяти (быстрый путь), но пишутся write-through в GORM-таблицы и восстанавливаются при старте.

**Tech Stack:** Go 1.25 / Fiber v3 (`middleware/limiter`), GORM (sqlite+postgres), Rust reqwest 0.12, GitHub Actions, pg_dump.

---

### Task 1: Fail-fast на дев-секретах в проде + уровень логов

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/cmd/server/main.go:21-35`
- Modify: `backend/cmd/bot/main.go` (аналогичная проверка при старте)
- Test: `backend/internal/config/config_test.go` (создать)

- [ ] **Step 1: Написать падающий тест**

```go
package config

import "testing"

func TestValidateRejectsDevSecretsInProduction(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: "dev-only-change-me"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("production с dev JWT-секретом должен отклоняться")
	}
	cfg.JWTSecret = "change-me-in-production"
	if err := cfg.Validate(); err == nil {
		t.Fatal("production с compose-заглушкой JWT-секрета должен отклоняться")
	}
}

func TestValidateAllowsDevSecretsInDevelopment(t *testing.T) {
	cfg := Config{AppEnv: "development", JWTSecret: "dev-only-change-me"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development должен работать с дефолтным секретом: %v", err)
	}
}

func TestValidateAllowsRealSecretInProduction(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: "a-real-32-char-random-secret-value"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production с нормальным секретом должен проходить: %v", err)
	}
}
```

- [ ] **Step 2: Запустить, убедиться что FAIL** — `cd backend && go test ./internal/config/` → ошибка компиляции `cfg.Validate undefined`.

- [ ] **Step 3: Реализация.** В `config.go` добавить:

```go
// devSecrets — известные дефолты, с которыми нельзя выходить в прод.
var devSecrets = map[string]bool{
	"dev-only-change-me":       true,
	"change-me-in-production":  true,
}

// Validate отклоняет конфигурацию, с которой опасно стартовать в production.
func (c Config) Validate() error {
	if c.AppEnv != "production" {
		return nil
	}
	if devSecrets[c.JWTSecret] {
		return errors.New("APP_ENV=production требует настоящий JWT_SECRET (сейчас дев-заглушка)")
	}
	return nil
}
```

(добавить `"errors"` в импорты). В `Load()` добавить `LogLevel`-поле:

```go
LogLevel: strings.ToLower(env("LOG_LEVEL", "info")),
```

и поле `LogLevel string` в структуру. Хелпер:

```go
// SlogLevel переводит LOG_LEVEL в slog.Level (debug/info/warn/error).
func (c Config) SlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 4: Подключить в `cmd/server/main.go`** — после `cfg := config.Load()`:

```go
if err := cfg.Validate(); err != nil {
	slog.Error("invalid configuration", "error", err)
	os.Exit(1)
}
```

и заменить инициализацию логгера (логгер до Load — оставить Info, после Load переустановить):

```go
cfg := config.Load()
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: cfg.SlogLevel(),
})))
```

То же добавить в `cmd/bot/main.go` (Validate + выход).

- [ ] **Step 5: Тесты** — `cd backend && go test ./internal/config/ ./...` → PASS.
- [ ] **Step 6: Commit** — `git add backend && git commit -m "feat(config): fail-fast на дев-секретах в production, LOG_LEVEL"`.

---

### Task 2: Rate limiting на auth-эндпоинты

**Files:**
- Modify: `backend/cmd/server/main.go:37-40`

- [ ] **Step 1: Добавить limiter.** В `cmd/server/main.go`:

```go
import (
	"time"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

app := fiber.New(fiber.Config{
	AppName: "Launcher Backend",
	// Бэкенд стоит за nginx: настоящий IP клиента приходит в X-Forwarded-For.
	ProxyHeader:      fiber.HeaderXForwardedFor,
	TrustProxy:       true,
	TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true, LinkLocal: true, Private: true},
})
app.Use(middleware.CORS(cfg.AllowedOrigins))

// Брутфорс-защита: лимит на эндпоинты, принимающие пароль.
authLimiter := limiter.New(limiter.Config{
	Max:        10,
	Expiration: time.Minute,
})
app.Use("/api/auth/login", authLimiter)
app.Use("/api/gml/auth", authLimiter)
app.Use("/api/yggdrasil/authserver/authenticate", authLimiter)
```

Если `TrustProxyConfig`/`TrustProxy` не совпадут с API Fiber v3.3 — проверить `go doc github.com/gofiber/fiber/v3 Config` и использовать фактические имена полей.

- [ ] **Step 2: Сборка и smoke-тест**

```bash
cd backend && go build ./... && go test ./...
go run ./cmd/server &  # SQLite-режим
for i in $(seq 1 12); do curl -s -o /dev/null -w "%{http_code}\n" -X POST 127.0.0.1:8080/api/auth/login -H 'Content-Type: application/json' -d '{"login":"x","password":"y"}'; done
```

Ожидание: первые 10 → 400/401, затем 429.

- [ ] **Step 3: Commit** — `git commit -m "feat(server): rate limiting на login-эндпоинты"`.

---

### Task 3: Yggdrasil-сессии переживают рестарт (write-through в БД)

**Files:**
- Create: `backend/internal/models/yggdrasil.go`
- Modify: `backend/internal/database/database.go:38-55`
- Modify: `backend/internal/yggdrasil/store.go`
- Modify: `backend/internal/yggdrasil/service.go:27-36`
- Test: `backend/internal/yggdrasil/store_persistence_test.go` (создать)

Семантика: память — источник истины (все текущие тесты с `NewStore(nil)`/`NewService(nil,…)` работают как раньше); при наличии БД операции дублируются в таблицы (best-effort, ошибки в лог), при старте живые записи загружаются обратно.

- [ ] **Step 1: Модели** — `backend/internal/models/yggdrasil.go`:

```go
package models

import "time"

// YggdrasilSession — персист игровой сессии: переживает рестарт backend,
// чтобы игроков не выкидывало с серверов при деплое.
type YggdrasilSession struct {
	AccessToken string    `gorm:"primaryKey;size:64"`
	ClientToken string    `gorm:"size:64"`
	UUID        string    `gorm:"size:64"`
	Name        string    `gorm:"size:64"`
	Nonce       string    `gorm:"size:64;index"`
	Verified    bool
	ExpiresAt   time.Time `gorm:"index"`
}

// YggdrasilJoin — персист join-записи (короткий TTL, hasJoined-проверка).
type YggdrasilJoin struct {
	ServerID  string    `gorm:"primaryKey;size:128"`
	UUID      string    `gorm:"size:64"`
	Name      string    `gorm:"size:64"`
	IP        string    `gorm:"size:64"`
	ExpiresAt time.Time `gorm:"index"`
}
```

Добавить обе модели в `database.AutoMigrate`.

- [ ] **Step 2: Падающий тест** — `store_persistence_test.go`:

```go
package yggdrasil

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"launcher-backend/internal/models"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.YggdrasilSession{}, &models.YggdrasilJoin{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestStoreSurvivesRestart(t *testing.T) {
	db := newTestDB(t)

	store := NewStore(db)
	store.PutSession(Session{AccessToken: "tok-1", ClientToken: "ct", UUID: "u", Name: "Liko", Nonce: "n-1"})
	if !store.MarkVerifiedByNonce("n-1") {
		t.Fatal("nonce должен находиться")
	}

	// «Рестарт»: новый Store на той же БД.
	restarted := NewStore(db)
	sess, ok := restarted.Session("tok-1")
	if !ok {
		t.Fatal("сессия должна восстановиться из БД после рестарта")
	}
	if !sess.Verified {
		t.Fatal("флаг Verified должен пережить рестарт")
	}
}

func TestInvalidateRemovesFromDB(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	store.PutSession(Session{AccessToken: "tok-2", UUID: "u", Name: "Liko"})
	store.Invalidate("tok-2")

	restarted := NewStore(db)
	if _, ok := restarted.Session("tok-2"); ok {
		t.Fatal("инвалидированная сессия не должна восстанавливаться")
	}
}

func TestJoinSurvivesRestart(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	store.PutJoin("srv-1", JoinRecord{UUID: "u", Name: "Liko", IP: "1.2.3.4"})

	restarted := NewStore(db)
	if _, ok := restarted.ConsumeJoin("srv-1"); !ok {
		t.Fatal("join-запись должна восстановиться из БД")
	}
	// ConsumeJoin одноразовый — повторный рестарт её не вернёт.
	again := NewStore(db)
	if _, ok := again.ConsumeJoin("srv-1"); ok {
		t.Fatal("использованный join не должен восстанавливаться")
	}
}
```

- [ ] **Step 3: Запустить, убедиться что FAIL** — `go test ./internal/yggdrasil/` → ошибка: `NewStore` не принимает аргументов.

- [ ] **Step 4: Реализация в `store.go`.** Сигнатура `NewStore(db *gorm.DB)`; nil → чисто in-memory. Каждая мутация дублируется в БД:

```go
type Store struct {
	mu       sync.Mutex
	db       *gorm.DB // nil — без персиста (тесты, dev)
	sessions map[string]Session
	joins    map[string]JoinRecord
	nonces   map[string]string
}

func NewStore(db *gorm.DB) *Store {
	s := &Store{
		db:       db,
		sessions: make(map[string]Session),
		joins:    make(map[string]JoinRecord),
		nonces:   make(map[string]string),
	}
	s.restore()
	go s.collectGarbage()
	return s
}

// restore загружает живые сессии и join-записи после рестарта backend.
func (s *Store) restore() {
	if s.db == nil {
		return
	}
	now := time.Now()
	var sessions []models.YggdrasilSession
	if err := s.db.Where("expires_at > ?", now).Find(&sessions).Error; err != nil {
		slog.Warn("yggdrasil: restore sessions failed", "error", err)
	}
	for _, row := range sessions {
		sess := Session{
			AccessToken: row.AccessToken, ClientToken: row.ClientToken,
			UUID: row.UUID, Name: row.Name, Nonce: row.Nonce,
			Verified: row.Verified, expiresAt: row.ExpiresAt,
		}
		s.sessions[sess.AccessToken] = sess
		if sess.Nonce != "" {
			s.nonces[sess.Nonce] = sess.AccessToken
		}
	}
	var joins []models.YggdrasilJoin
	if err := s.db.Where("expires_at > ?", now).Find(&joins).Error; err != nil {
		slog.Warn("yggdrasil: restore joins failed", "error", err)
	}
	for _, row := range joins {
		s.joins[row.ServerID] = JoinRecord{UUID: row.UUID, Name: row.Name, IP: row.IP, expiresAt: row.ExpiresAt}
	}
	if len(sessions) > 0 || len(joins) > 0 {
		slog.Info("yggdrasil: sessions restored", "sessions", len(sessions), "joins", len(joins))
	}
}

// persistSession/deleteSession/persistJoin/deleteJoin — best-effort запись в БД.
func (s *Store) persistSession(sess Session) {
	if s.db == nil {
		return
	}
	row := models.YggdrasilSession{
		AccessToken: sess.AccessToken, ClientToken: sess.ClientToken,
		UUID: sess.UUID, Name: sess.Name, Nonce: sess.Nonce,
		Verified: sess.Verified, ExpiresAt: sess.expiresAt,
	}
	if err := s.db.Save(&row).Error; err != nil {
		slog.Warn("yggdrasil: persist session failed", "error", err)
	}
}

func (s *Store) deleteSession(token string) {
	if s.db == nil {
		return
	}
	if err := s.db.Delete(&models.YggdrasilSession{}, "access_token = ?", token).Error; err != nil {
		slog.Warn("yggdrasil: delete session failed", "error", err)
	}
}

func (s *Store) persistJoin(serverID string, rec JoinRecord) {
	if s.db == nil {
		return
	}
	row := models.YggdrasilJoin{ServerID: serverID, UUID: rec.UUID, Name: rec.Name, IP: rec.IP, ExpiresAt: rec.expiresAt}
	if err := s.db.Save(&row).Error; err != nil {
		slog.Warn("yggdrasil: persist join failed", "error", err)
	}
}

func (s *Store) deleteJoin(serverID string) {
	if s.db == nil {
		return
	}
	if err := s.db.Delete(&models.YggdrasilJoin{}, "server_id = ?", serverID).Error; err != nil {
		slog.Warn("yggdrasil: delete join failed", "error", err)
	}
}
```

Вызовы по операциям (внутри существующих методов, после изменения map):
- `PutSession` → `s.persistSession(sess)`
- `MarkVerifiedByNonce` → после `s.sessions[token] = sess` → `s.persistSession(sess)`
- `ReplaceToken` → `s.deleteSession(oldToken)` + `s.persistSession(sess)`
- `InvalidateByNonce`, `Invalidate` → `s.deleteSession(token)`
- `TouchSession` → `s.persistSession(sess)`
- `PutJoin` → `s.persistJoin(serverID, record)`
- `ConsumeJoin` → `s.deleteJoin(serverID)`
- `collectGarbage` → после очистки map: `s.db.Delete(... "expires_at <= ?", now)` для обеих таблиц (если db != nil)

Импорты store.go: добавить `"log/slog"`, `"gorm.io/gorm"`, `"launcher-backend/internal/models"`.

- [ ] **Step 5: `service.go`** — `store: NewStore(db)` в `NewService` (db уже передаётся первым аргументом).

- [ ] **Step 6: Тесты** — `cd backend && go test ./...` → PASS (включая anticheat-интеграционные и flow_test с nil db).

- [ ] **Step 7: Commit** — `git commit -m "feat(yggdrasil): сессии переживают рестарт backend (write-through в БД)"`.

---

### Task 4: docker-compose hardening + чистка неиспользуемых сервисов

**Files:**
- Modify: `docker-compose.yml`
- Modify: `dev.sh:179-181` (убрать ожидание redis/minio)

- [ ] **Step 1: Переписать docker-compose.yml:**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: launcher
      POSTGRES_USER: launcher
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD:-launcher_dev_password}"
    ports:
      # dev: открыт на 5432 (нужно dev.sh). prod задаёт POSTGRES_BIND=127.0.0.1:5432 в .env
      - "${POSTGRES_BIND:-5432}:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      # Схема БД создаётся приложением через GORM AutoMigrate при старте server/bot.
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U launcher -d launcher"]
      interval: 5s
      timeout: 3s
      retries: 12

  # API-сервер (Fiber). Схема БД создаётся GORM AutoMigrate при старте.
  server:
    build: ./backend
    command: ["/app/server"]
    restart: unless-stopped
    environment:
      SERVER_ADDR: "0.0.0.0:8080"
      DATABASE_URL: "postgres://launcher:${POSTGRES_PASSWORD:-launcher_dev_password}@postgres:5432/launcher?sslmode=disable"
      AUTH_MODE: "${AUTH_MODE:-local}"
      JWT_SECRET: "${JWT_SECRET:-change-me-in-production}"
      ALLOWED_ORIGINS: "${ALLOWED_ORIGINS:-http://127.0.0.1:3000,http://localhost:3000}"
      # prod (.env на VPS): APP_ENV=production — сервер откажется стартовать с дев-секретами.
      APP_ENV: "${APP_ENV:-development}"
      LOG_LEVEL: "${LOG_LEVEL:-info}"
    ports:
      # dev: 0.0.0.0:8080. prod задаёт SERVER_BIND=127.0.0.1:8082 в .env (за nginx)
      - "${SERVER_BIND:-8080}:8080"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1"]
      interval: 15s
      timeout: 5s
      retries: 4
      start_period: 10s

  # Telegram-бот. Делит ту же БД, что и server.
  bot:
    build: ./backend
    command: ["/app/bot"]
    restart: unless-stopped
    environment:
      DATABASE_URL: "postgres://launcher:${POSTGRES_PASSWORD:-launcher_dev_password}@postgres:5432/launcher?sslmode=disable"
      AUTH_MODE: "${AUTH_MODE:-local}"
      APP_ENV: "${APP_ENV:-development}"
      TELEGRAM_BOT_TOKEN: "${TELEGRAM_BOT_TOKEN:-}"
      ADMIN_TELEGRAM_IDS: "${ADMIN_TELEGRAM_IDS:-}"
      TOTP_ISSUER: "${TOTP_ISSUER:-Launcher Accounts}"
      DONATE_SHOP_URL: "${DONATE_SHOP_URL:-https://shop.likonchik.xyz}"
      LAUNCHER_EXE_PATH: "${LAUNCHER_EXE_PATH:-}"
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  postgres_data:
```

Redis и MinIO удалены — кодом не используются. ВАЖНО: проверить, что в рантайм-образе backend/Dockerfile есть `wget` (alpine: есть busybox wget; debian-slim: добавить `apt-get install wget` или заменить healthcheck на `curl`). Проверить базовый образ перед коммитом.

- [ ] **Step 2: dev.sh** — удалить строки `wait_for_port "REDIS" ...` и `wait_for_port "MINIO" ...` и упоминания в логах (строки 171-172, 180-181).

- [ ] **Step 3: Проверка** — `docker compose config -q` (валидность), затем `docker compose up -d --build && docker compose ps` → все сервисы healthy; `curl 127.0.0.1:8080/health` → `{"ok":true,...}`. Остановить: `docker compose down`.

- [ ] **Step 4: Commit** — `git commit -m "ops(compose): healthchecks, restart-политики, удалены неиспользуемые redis/minio"`.

---

### Task 5: Лаунчер — корректные таймауты скачивания

**Files:**
- Modify: `launcher-slint/src/main.rs:2232-2237` (добавить download_client рядом с http_client)
- Modify: вызовы в путях скачивания: `main.rs:1250` (файлы профиля), `main.rs:1387` (java runtime) — проверить контекст каждого call-site `http_client()` и заменить только те, что качают файлы.

Проблема: `http_client()` имеет **общий** таймаут 30 с на весь запрос — большой файл на медленном канале обрывается. Для скачивания нужен connect_timeout + read_timeout (таймаут простоя чтения), без общего лимита.

- [ ] **Step 1: Добавить download_client** (рядом с `http_client()`):

```rust
/// Клиент для скачивания файлов: без общего таймаута (большие файлы качаются
/// дольше 30 с), но связь контролируется connect/read-таймаутами.
fn download_client() -> Result<Client, String> {
    Client::builder()
        .connect_timeout(Duration::from_secs(15))
        .read_timeout(Duration::from_secs(60))
        .tcp_keepalive(Duration::from_secs(20))
        .build()
        .map_err(|_| "Не удалось создать HTTP клиент.".to_string())
}
```

- [ ] **Step 2: Заменить call-sites.** Прочитать контекст каждого `let client = http_client()` (строки 1139, 1250, 1387 и соседние) и заменить на `download_client()` там, где идёт скачивание manifest-файлов/файлов профиля/java. API-запросы (login, профили, SSE-вспомогательные) оставить на `http_client()`.

- [ ] **Step 3: Проверка** — `cd launcher-slint && cargo check` → без ошибок. Если `read_timeout` отсутствует в reqwest 0.12.x из lockfile — `cargo update -p reqwest` или убрать `read_timeout`, оставив connect_timeout+keepalive.

- [ ] **Step 4: Commit** — `git commit -m "fix(launcher): большие файлы не обрываются 30-секундным таймаутом"`.

---

### Task 6: deploy.sh — параметризация VPS + тесты перед выкаткой

**Files:**
- Modify: `deploy.sh:19-33`

- [ ] **Step 1: Правки:**

```bash
VPS="${DEPLOY_VPS:-root@13.140.17.105}"
DIR="${DEPLOY_DIR:-/root/Launcher}"
BRANCH="${DEPLOY_BRANCH:-main}"
```

и перед пушем добавить шаг 0:

```bash
cyan "→ [0/3] Тесты backend..."
( cd "$(dirname "$0")/backend" && go test ./... )
```

(нумерацию шагов в выводе обновить на 0/3..3/3).

- [ ] **Step 2: Проверка** — `bash -n deploy.sh` и `cd backend && go test ./...`.
- [ ] **Step 3: Commit** — `git commit -m "ops(deploy): тесты перед выкаткой, VPS/DIR/BRANCH из env"`.

---

### Task 7: Бэкапы PostgreSQL

**Files:**
- Create: `scripts/prod/backup-db.sh`
- Modify: `docs/vps-production.md` (раздел про бэкапы)

- [ ] **Step 1: Скрипт** `scripts/prod/backup-db.sh`:

```bash
#!/usr/bin/env bash
#
# Бэкап PostgreSQL из docker compose. Запускается на VPS по cron:
#   0 4 * * * /root/Launcher/scripts/prod/backup-db.sh >> /var/log/launcher-backup.log 2>&1
#
# Хранит 14 последних дампов в /root/backups/launcher.

set -euo pipefail

DIR="${LAUNCHER_DIR:-/root/Launcher}"
BACKUP_DIR="${BACKUP_DIR:-/root/backups/launcher}"
KEEP="${BACKUP_KEEP:-14}"

mkdir -p "${BACKUP_DIR}"
stamp="$(date +%Y%m%d-%H%M%S)"
target="${BACKUP_DIR}/launcher-${stamp}.sql.gz"

cd "${DIR}"
docker compose exec -T postgres pg_dump -U launcher -d launcher | gzip > "${target}"

# Пустой дамп — признак проблемы: не затираем им ротацию.
if [ ! -s "${target}" ]; then
  echo "ERROR: пустой дамп ${target}" >&2
  rm -f "${target}"
  exit 1
fi

# Ротация: оставляем KEEP свежих.
ls -1t "${BACKUP_DIR}"/launcher-*.sql.gz 2>/dev/null | tail -n "+$((KEEP + 1))" | xargs -r rm -f

echo "OK: ${target} ($(du -h "${target}" | cut -f1))"
```

`chmod +x scripts/prod/backup-db.sh`.

- [ ] **Step 2: Документация** — в `docs/vps-production.md` добавить раздел «Бэкапы БД»: установка cron-строки, восстановление (`gunzip -c dump.sql.gz | docker compose exec -T postgres psql -U launcher -d launcher`), рекомендация копировать каталог бэкапов за пределы VPS (rclone/scp).

- [ ] **Step 3: Проверка** — `bash -n scripts/prod/backup-db.sh`; локально при поднятом compose: `LAUNCHER_DIR="$PWD" BACKUP_DIR=/tmp/launcher-backups ./scripts/prod/backup-db.sh` → файл с ненулевым размером.

- [ ] **Step 4: Commit** — `git commit -m "ops(backup): скрипт бэкапа PostgreSQL с ротацией"`.

- [ ] **Step 5 (после деплоя, на VPS):** установить cron: `ssh root@13.140.17.105 '(crontab -l 2>/dev/null | grep -v backup-db.sh; echo "0 4 * * * /root/Launcher/scripts/prod/backup-db.sh >> /var/log/launcher-backup.log 2>&1") | crontab -'` и прогнать скрипт вручную один раз. Также убедиться, что в `/root/Launcher/.env` есть `APP_ENV=production` и настоящий `JWT_SECRET` (иначе сервер по новой проверке не стартует!).

---

### Task 8: CI — GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Workflow:**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  backend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: backend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - run: go vet ./...
      - run: go test ./...

  dashboard:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: dashboard
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: dashboard/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run build

  launcher:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: sudo apt-get update && sudo apt-get install -y libfontconfig1-dev libfreetype6-dev
      - uses: dtolnay/rust-toolchain@stable
      - uses: Swatinem/rust-cache@v2
        with:
          workspaces: launcher-slint
      - run: cargo check --locked
        working-directory: launcher-slint
```

Перед коммитом проверить наличие `dashboard/package-lock.json` и `launcher-slint/Cargo.lock` (для `--locked`); если lock-файлов нет — убрать соответствующие опции кеша/`--locked`.

- [ ] **Step 2: Локальная репетиция** — `cd backend && go vet ./... && go test ./...`; `cd dashboard && npm run lint && npm run build`; `cd launcher-slint && cargo check`.

- [ ] **Step 3: Commit** — `git commit -m "ci: GitHub Actions (go test, next build, cargo check)"`.

---

## Вне скоупа (отдельным планом, если захочется)

- Delta-обновления в лаунчере (manifest diff).
- Разбиение `main.rs` (2700 строк) на модули.
- httpOnly-cookie вместо localStorage в dashboard + refresh-токены.
- golang-migrate вместо AutoMigrate.
