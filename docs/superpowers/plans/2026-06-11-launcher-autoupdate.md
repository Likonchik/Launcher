# Автообновление лаунчера — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Лаунчер проверяет обновления при запуске и во время работы, скачивает бинарник в фоне, подменяет себя и перезапускается по кнопке; релизы заливаются через дашборд; обязательные релизы блокируют запуск игры на старых версиях (клиентски и серверно через 426 на launch-token).

**Architecture:** Новый Go-домен `internal/launcherrelease` (модели `LauncherRelease`/`LauncherReleaseFile`, файлы в `backend/storage/releases/<version>/<platform>/`), публичные эндпоинты `/api/launcher/update|download`, админские `/api/admin/releases`. Уведомления через существующий SSE-брокер (`/api/profiles/events`, payload `launcher-release`). В лаунчере — модуль `src/updater.rs` (самозамена бинарника: Linux atomic rename, Windows rename-to-.old). Серверный форс-апдейт — 426 в `anticheat handshake/init` по заголовку `X-Launcher-Version`.

**Tech Stack:** Go + Fiber v3 + GORM (AutoMigrate), Rust + reqwest blocking + Slint, Next.js 15 + Tailwind 4.

**Спек:** `docs/superpowers/specs/2026-06-11-launcher-autoupdate-design.md`

**Верификация (нет локального Go — только Docker):**
```bash
# Go-тесты и vet (из корня репо):
docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm sh -c "go vet ./... && go test ./..."
# Rust (из корня репо):
cargo test -p launcher-slint && cargo build -p launcher-slint
# Dashboard (тайпчек):
cd dashboard && npm run build
```

---

## Структура файлов

| Действие | Файл | Ответственность |
|---|---|---|
| Create | `backend/internal/models/launcher_release.go` | GORM-модели релизов |
| Modify | `backend/internal/database/database.go` | AutoMigrate новых моделей |
| Modify | `backend/internal/config/config.go` | `LAUNCHER_RELEASE_ROOT` |
| Create | `backend/internal/launcherrelease/version.go` | semver-компаратор |
| Create | `backend/internal/launcherrelease/version_test.go` | тесты компаратора |
| Create | `backend/internal/launcherrelease/service.go` | CRUD релизов, CheckUpdate, Download |
| Create | `backend/internal/launcherrelease/service_test.go` | тесты сервиса (sqlite in-memory) |
| Create | `backend/internal/launcherrelease/handler.go` | роуты + multipart + SSE publish |
| Create | `backend/internal/launcherrelease/handler_test.go` | тесты хендлера |
| Modify | `backend/cmd/server/main.go` | регистрация + BodyLimit |
| Modify | `backend/internal/anticheat/handler.go` | 426-гейт по X-Launcher-Version |
| Create | `backend/internal/anticheat/version_gate_test.go` | тест гейта |
| Create | `launcher-slint/src/updater.rs` | проверка/скачивание/подмена/перезапуск |
| Modify | `launcher-slint/src/main.rs` | интеграция: старт, SSE, периодика, UI-провода |
| Modify | `launcher-slint/src/anticheat/mod.rs` | заголовок версии + обработка 426 |
| Modify | `launcher-slint/ui/app.slint` | баннер обновления, блокировка «Играть» |
| Modify | `dashboard/app/lib/types.ts` | тип LauncherRelease |
| Modify | `dashboard/app/lib/api.ts` | `apiUpload` (multipart + прогресс) |
| Create | `dashboard/app/releases/page.tsx` | страница «Релизы» |
| Create | `dashboard/components/releases/release-form.tsx` | форма создания релиза |
| Create | `dashboard/components/releases/release-list.tsx` | таблица релизов |
| Modify | `dashboard/components/shell/sidebar.tsx` | пункт меню «Релизы» |
| Modify | `CLAUDE.md` | упоминание нового домена |

---

### Task 1: Модели + AutoMigrate + конфиг

**Files:**
- Create: `backend/internal/models/launcher_release.go`
- Modify: `backend/internal/database/database.go:38-58`
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Создать модели**

`backend/internal/models/launcher_release.go`:

```go
package models

import "time"

// LauncherRelease — версия десктоп-лаунчера, заливается через дашборд.
// Mandatory: клиенты ниже этой версии не получают launch-token (форс-апдейт).
// IsActive=false снимает релиз с раздачи (откат публикации).
type LauncherRelease struct {
	ID        string                `gorm:"type:uuid;primaryKey" json:"id"`
	Version   string                `gorm:"size:32;uniqueIndex;not null" json:"version"`
	Changelog string                `json:"changelog"`
	Mandatory bool                  `gorm:"not null;default:false" json:"mandatory"`
	IsActive  bool                  `gorm:"not null;default:true" json:"isActive"`
	Files     []LauncherReleaseFile `gorm:"foreignKey:ReleaseID" json:"files"`
	CreatedAt time.Time             `json:"createdAt"`
}

// LauncherReleaseFile — бинарник релиза под конкретную платформу.
// Лежит на диске в storage/releases/<version>/<platform>/<FileName>.
type LauncherReleaseFile struct {
	ID         string `gorm:"type:uuid;primaryKey" json:"id"`
	ReleaseID  string `gorm:"type:uuid;index;not null" json:"releaseId"`
	Platform   string `gorm:"size:32;not null" json:"platform"`
	FileName   string `gorm:"size:255;not null" json:"fileName"`
	HashSHA256 string `gorm:"size:64;not null" json:"hashSha256"`
	Size       int64  `gorm:"not null" json:"size"`
}
```

- [ ] **Step 2: Добавить в AutoMigrate**

В `backend/internal/database/database.go` в список `db.AutoMigrate(...)` после `&models.YggdrasilJoin{},` добавить:

```go
		// Релизы лаунчера (автообновление).
		&models.LauncherRelease{},
		&models.LauncherReleaseFile{},
```

- [ ] **Step 3: Добавить конфиг**

В `backend/internal/config/config.go`: в структуру `Config` после `ProfileStorageRoot  string` добавить:

```go
	LauncherReleaseRoot string
```

В `Load()` после блока `ProfileStorageRoot: env(...)` добавить:

```go
		LauncherReleaseRoot: env(
			"LAUNCHER_RELEASE_ROOT",
			filepath.Join("storage", "releases"),
		),
```

- [ ] **Step 4: Проверить компиляцию**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go vet ./...`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/launcher_release.go backend/internal/database/database.go backend/internal/config/config.go
git commit -m "feat(releases): модели LauncherRelease + конфиг хранилища"
```

---

### Task 2: Semver-компаратор (TDD)

**Files:**
- Create: `backend/internal/launcherrelease/version.go`
- Test: `backend/internal/launcherrelease/version_test.go`

- [ ] **Step 1: Написать падающий тест**

`backend/internal/launcherrelease/version_test.go`:

```go
package launcherrelease

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"0.10.0", "0.9.0", 1},   // числовое, не лексикографическое
		{"1.2", "1.2.0", 0},      // отсутствующий сегмент = 0
		{"1.2.1", "1.2", 1},
		{"abc", "0.0.1", -1},     // мусор = 0.0.0
		{"", "0.0.0", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestValidVersion(t *testing.T) {
	valid := []string{"0.1.0", "1.0.0", "10.20.30"}
	invalid := []string{"", "1.0", "1.0.0.0", "v1.0.0", "1.0.x", "1..0", " 1.0.0"}
	for _, v := range valid {
		if !ValidVersion(v) {
			t.Errorf("ValidVersion(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if ValidVersion(v) {
			t.Errorf("ValidVersion(%q) = true, want false", v)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/launcherrelease/`
Expected: FAIL (undefined: CompareVersions).

- [ ] **Step 3: Реализация**

`backend/internal/launcherrelease/version.go`:

```go
// Package launcherrelease — релизы десктоп-лаунчера: хранение бинарников,
// проверка обновлений и вычисление обязательной минимальной версии.
package launcherrelease

import (
	"strconv"
	"strings"
)

// CompareVersions сравнивает версии "X.Y.Z" посегментно: -1 если a < b,
// 0 если равны, 1 если a > b. Отсутствующие или нечисловые сегменты
// считаются нулями ("1.2" == "1.2.0"; мусор в заголовке = "0.0.0").
func CompareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := segment(as, i), segment(bs, i)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}

func segment(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ValidVersion — строгий формат X.Y.Z (только цифры) для входных данных админки.
func ValidVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 4: Тест проходит**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/launcherrelease/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/launcherrelease/
git commit -m "feat(releases): semver-компаратор"
```

---

### Task 3: Service — CheckUpdate / MinMandatoryVersion / Download (TDD)

**Files:**
- Create: `backend/internal/launcherrelease/service.go`
- Test: `backend/internal/launcherrelease/service_test.go`

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/launcherrelease/service_test.go` (паттерн `newTestService` — как в `backend/internal/profiles/service_test.go:213`):

```go
package launcherrelease

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"launcher-backend/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) Service {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.LauncherRelease{}, &models.LauncherReleaseFile{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return NewService(db, t.TempDir())
}

// createRelease — хелпер: создаёт релиз с бинарником под обе платформы.
func createRelease(t *testing.T, s Service, version string, mandatory bool) models.LauncherRelease {
	t.Helper()
	release, err := s.Create(context.Background(),
		CreateRequest{Version: version, Changelog: "чейнджлог " + version, Mandatory: mandatory},
		[]UploadedFile{
			{Platform: "linux-x64", FileName: "launcher", Reader: bytes.NewReader([]byte("bin-" + version))},
			{Platform: "windows-x64", FileName: "launcher.exe", Reader: bytes.NewReader([]byte("exe-" + version))},
		})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", version, err)
	}
	return release
}

func TestCheckUpdate(t *testing.T) {
	s := newTestService(t)
	createRelease(t, s, "0.2.0", false)
	createRelease(t, s, "0.3.0", true)
	createRelease(t, s, "0.4.0", false)

	// Старый клиент: есть обновление, обязательное (0.3.0 в интервале).
	info, err := s.CheckUpdate(context.Background(), "linux-x64", "0.1.0")
	if err != nil {
		t.Fatalf("CheckUpdate() error = %v", err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "0.4.0" || !info.Mandatory {
		t.Fatalf("info = %+v, want available 0.4.0 mandatory", info)
	}
	if info.DownloadURL != "/api/launcher/download/0.4.0/linux-x64" {
		t.Fatalf("DownloadURL = %q", info.DownloadURL)
	}
	if info.SHA256 == "" || info.Size == 0 {
		t.Fatalf("file meta missing: %+v", info)
	}

	// Клиент новее mandatory-границы: обновление есть, но не обязательное.
	info, _ = s.CheckUpdate(context.Background(), "linux-x64", "0.3.0")
	if !info.UpdateAvailable || info.Mandatory {
		t.Fatalf("info = %+v, want available, not mandatory", info)
	}

	// Актуальный клиент: обновления нет.
	info, _ = s.CheckUpdate(context.Background(), "linux-x64", "0.4.0")
	if info.UpdateAvailable {
		t.Fatalf("info = %+v, want no update", info)
	}

	// Неизвестная платформа — ошибка.
	if _, err := s.CheckUpdate(context.Background(), "macos", "0.1.0"); err == nil {
		t.Fatal("CheckUpdate() accepted unknown platform")
	}
}

func TestCheckUpdateIgnoresInactive(t *testing.T) {
	s := newTestService(t)
	createRelease(t, s, "0.2.0", false)
	bad := createRelease(t, s, "0.3.0", true)

	inactive := false
	if _, err := s.Update(context.Background(), bad.ID, PatchRequest{IsActive: &inactive}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	info, err := s.CheckUpdate(context.Background(), "linux-x64", "0.1.0")
	if err != nil {
		t.Fatalf("CheckUpdate() error = %v", err)
	}
	if info.LatestVersion != "0.2.0" || info.Mandatory {
		t.Fatalf("info = %+v, want latest 0.2.0 without mandatory", info)
	}
}

func TestMinMandatoryVersion(t *testing.T) {
	s := newTestService(t)
	if v, err := s.MinMandatoryVersion(context.Background()); err != nil || v != "" {
		t.Fatalf("empty store: v=%q err=%v, want \"\"", v, err)
	}
	createRelease(t, s, "0.2.0", true)
	createRelease(t, s, "0.3.0", false)
	createRelease(t, s, "0.5.0", true)

	v, err := s.MinMandatoryVersion(context.Background())
	if err != nil || v != "0.5.0" {
		t.Fatalf("MinMandatoryVersion() = %q, %v; want 0.5.0", v, err)
	}
}

func TestDownload(t *testing.T) {
	s := newTestService(t)
	createRelease(t, s, "0.2.0", false)

	abs, file, err := s.Download(context.Background(), "0.2.0", "linux-x64")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if file.FileName != "launcher" || abs == "" {
		t.Fatalf("Download() = %q, %+v", abs, file)
	}
	// Путь должен указывать на реально записанный файл.
	if !strings.HasSuffix(abs, "/0.2.0/linux-x64/launcher") {
		t.Fatalf("abs = %q", abs)
	}
	if _, _, err := s.Download(context.Background(), "../../etc", "linux-x64"); err == nil {
		t.Fatal("Download() accepted path traversal in version")
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/launcherrelease/`
Expected: FAIL (undefined: Service и т.д.).

- [ ] **Step 3: Реализация сервиса**

`backend/internal/launcherrelease/service.go`:

```go
package launcherrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"launcher-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AllowedPlatforms — платформы, под которые собирается лаунчер (см. спек).
var AllowedPlatforms = []string{"linux-x64", "windows-x64"}

// maxReleaseFileSize — лимит размера одного бинарника (бэкенд-защита; в проде
// nginx client_max_body_size тоже должен пропускать такой запрос).
const maxReleaseFileSize = 200 << 20

type Service struct {
	db          *gorm.DB
	storageRoot string
}

func NewService(db *gorm.DB, storageRoot string) Service {
	return Service{db: db, storageRoot: storageRoot}
}

type CreateRequest struct {
	Version   string
	Changelog string
	Mandatory bool
}

// UploadedFile — один бинарник из multipart-формы админки.
type UploadedFile struct {
	Platform string
	FileName string
	Reader   io.Reader
}

// PatchRequest — частичное обновление флагов релиза.
type PatchRequest struct {
	Mandatory *bool `json:"mandatory"`
	IsActive  *bool `json:"isActive"`
}

// UpdateInfo — ответ /api/launcher/update для лаунчера.
type UpdateInfo struct {
	UpdateAvailable bool   `json:"updateAvailable"`
	LatestVersion   string `json:"latestVersion"`
	Mandatory       bool   `json:"mandatory"`
	Changelog       string `json:"changelog"`
	DownloadURL     string `json:"downloadUrl"`
	SHA256          string `json:"sha256"`
	Size            int64  `json:"size"`
}

func isAllowedPlatform(platform string) bool {
	for _, allowed := range AllowedPlatforms {
		if platform == allowed {
			return true
		}
	}
	return false
}

func (s Service) Create(ctx context.Context, req CreateRequest, files []UploadedFile) (models.LauncherRelease, error) {
	version := strings.TrimSpace(req.Version)
	if !ValidVersion(version) {
		return models.LauncherRelease{}, errors.New("версия должна быть в формате X.Y.Z")
	}
	if len(files) == 0 {
		return models.LauncherRelease{}, errors.New("прикрепите бинарник хотя бы для одной платформы")
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&models.LauncherRelease{}).
		Where("version = ?", version).Count(&count).Error; err != nil {
		return models.LauncherRelease{}, err
	}
	if count > 0 {
		return models.LauncherRelease{}, fmt.Errorf("релиз %s уже существует", version)
	}

	release := models.LauncherRelease{
		ID:        uuid.NewString(),
		Version:   version,
		Changelog: strings.TrimSpace(req.Changelog),
		Mandatory: req.Mandatory,
		IsActive:  true,
	}

	seen := map[string]bool{}
	for _, file := range files {
		if !isAllowedPlatform(file.Platform) {
			return models.LauncherRelease{}, fmt.Errorf("неизвестная платформа: %s", file.Platform)
		}
		if seen[file.Platform] {
			return models.LauncherRelease{}, fmt.Errorf("платформа %s указана дважды", file.Platform)
		}
		seen[file.Platform] = true

		stored, err := s.storeFile(release.ID, version, file)
		if err != nil {
			_ = os.RemoveAll(filepath.Join(s.storageRoot, version))
			return models.LauncherRelease{}, err
		}
		release.Files = append(release.Files, stored)
	}

	if err := s.db.WithContext(ctx).Create(&release).Error; err != nil {
		_ = os.RemoveAll(filepath.Join(s.storageRoot, version))
		return models.LauncherRelease{}, err
	}
	return release, nil
}

// storeFile пишет бинарник на диск, считая SHA-256 на лету.
func (s Service) storeFile(releaseID, version string, file UploadedFile) (models.LauncherReleaseFile, error) {
	dir := filepath.Join(s.storageRoot, version, file.Platform)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return models.LauncherReleaseFile{}, err
	}
	name := filepath.Base(strings.TrimSpace(file.FileName))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "launcher"
	}

	dst := filepath.Join(dir, name)
	out, err := os.Create(dst)
	if err != nil {
		return models.LauncherReleaseFile{}, err
	}

	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(file.Reader, maxReleaseFileSize+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dst)
		return models.LauncherReleaseFile{}, errors.New("не удалось сохранить файл релиза")
	}
	if size > maxReleaseFileSize {
		_ = os.Remove(dst)
		return models.LauncherReleaseFile{}, errors.New("файл релиза превышает 200 МБ")
	}
	if size == 0 {
		_ = os.Remove(dst)
		return models.LauncherReleaseFile{}, errors.New("файл релиза пуст")
	}

	return models.LauncherReleaseFile{
		ID:         uuid.NewString(),
		ReleaseID:  releaseID,
		Platform:   file.Platform,
		FileName:   name,
		HashSHA256: hex.EncodeToString(hasher.Sum(nil)),
		Size:       size,
	}, nil
}

func (s Service) List(ctx context.Context) ([]models.LauncherRelease, error) {
	releases := make([]models.LauncherRelease, 0)
	err := s.db.WithContext(ctx).Preload("Files").
		Order("created_at DESC").Find(&releases).Error
	return releases, err
}

func (s Service) Update(ctx context.Context, id string, req PatchRequest) (models.LauncherRelease, error) {
	var release models.LauncherRelease
	if err := s.db.WithContext(ctx).Preload("Files").First(&release, "id = ?", id).Error; err != nil {
		return models.LauncherRelease{}, err
	}
	if req.Mandatory != nil {
		release.Mandatory = *req.Mandatory
	}
	if req.IsActive != nil {
		release.IsActive = *req.IsActive
	}
	if err := s.db.WithContext(ctx).Model(&models.LauncherRelease{ID: release.ID}).
		Updates(map[string]any{"mandatory": release.Mandatory, "is_active": release.IsActive}).Error; err != nil {
		return models.LauncherRelease{}, err
	}
	return release, nil
}

func (s Service) Delete(ctx context.Context, id string) error {
	var release models.LauncherRelease
	if err := s.db.WithContext(ctx).First(&release, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).
		Where("release_id = ?", release.ID).Delete(&models.LauncherReleaseFile{}).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&release).Error; err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.storageRoot, release.Version))
}

// CheckUpdate — есть ли обновление для клиента version на платформе platform.
// Mandatory=true, если в интервале (version, latest] есть активный mandatory-релиз.
func (s Service) CheckUpdate(ctx context.Context, platform, clientVersion string) (UpdateInfo, error) {
	if !isAllowedPlatform(platform) {
		return UpdateInfo{}, errors.New("неизвестная платформа")
	}
	var releases []models.LauncherRelease
	if err := s.db.WithContext(ctx).Preload("Files").
		Where("is_active = ?", true).Find(&releases).Error; err != nil {
		return UpdateInfo{}, err
	}

	// Самый новый активный релиз, у которого есть бинарник под платформу.
	var latest *models.LauncherRelease
	var latestFile models.LauncherReleaseFile
	for i := range releases {
		for _, file := range releases[i].Files {
			if file.Platform != platform {
				continue
			}
			if latest == nil || CompareVersions(releases[i].Version, latest.Version) > 0 {
				latest = &releases[i]
				latestFile = file
			}
		}
	}
	if latest == nil || CompareVersions(latest.Version, clientVersion) <= 0 {
		return UpdateInfo{UpdateAvailable: false, LatestVersion: clientVersion}, nil
	}

	mandatory := false
	for _, release := range releases {
		if release.Mandatory &&
			CompareVersions(release.Version, clientVersion) > 0 &&
			CompareVersions(release.Version, latest.Version) <= 0 {
			mandatory = true
			break
		}
	}

	return UpdateInfo{
		UpdateAvailable: true,
		LatestVersion:   latest.Version,
		Mandatory:       mandatory,
		Changelog:       latest.Changelog,
		DownloadURL:     "/api/launcher/download/" + latest.Version + "/" + platform,
		SHA256:          latestFile.HashSHA256,
		Size:            latestFile.Size,
	}, nil
}

// MinMandatoryVersion — максимальная версия среди активных обязательных
// релизов; клиенты ниже неё не получают launch-token (426 в anticheat).
// Пустая строка — обязательных релизов нет.
func (s Service) MinMandatoryVersion(ctx context.Context) (string, error) {
	var releases []models.LauncherRelease
	if err := s.db.WithContext(ctx).
		Where("is_active = ? AND mandatory = ?", true, true).Find(&releases).Error; err != nil {
		return "", err
	}
	min := ""
	for _, release := range releases {
		if min == "" || CompareVersions(release.Version, min) > 0 {
			min = release.Version
		}
	}
	return min, nil
}

// Download — абсолютный путь к бинарнику активного релиза. Версия проходит
// строгую валидацию (только цифры и точки), платформа — allowlist, имя файла
// берётся из БД (сохранялось через filepath.Base) — traversal невозможен.
func (s Service) Download(ctx context.Context, version, platform string) (string, models.LauncherReleaseFile, error) {
	if !ValidVersion(version) || !isAllowedPlatform(platform) {
		return "", models.LauncherReleaseFile{}, errors.New("некорректный запрос")
	}
	var release models.LauncherRelease
	if err := s.db.WithContext(ctx).
		Where("version = ? AND is_active = ?", version, true).First(&release).Error; err != nil {
		return "", models.LauncherReleaseFile{}, err
	}
	var file models.LauncherReleaseFile
	if err := s.db.WithContext(ctx).
		Where("release_id = ? AND platform = ?", release.ID, platform).First(&file).Error; err != nil {
		return "", models.LauncherReleaseFile{}, err
	}
	abs, err := filepath.Abs(filepath.Join(s.storageRoot, version, platform, file.FileName))
	if err != nil {
		return "", models.LauncherReleaseFile{}, err
	}
	return abs, file, nil
}
```

- [ ] **Step 4: Тесты проходят**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/launcherrelease/`
Expected: PASS (включая Task 2 тесты).

Примечание: в тесте `TestDownload` проверка `strings.HasSuffix(abs, "/0.2.0/linux-x64/launcher")` — путь абсолютный, suffix сохраняется.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/launcherrelease/
git commit -m "feat(releases): сервис релизов — CRUD, CheckUpdate, MinMandatoryVersion"
```

---

### Task 4: Handler + регистрация в main.go + SSE publish

**Files:**
- Create: `backend/internal/launcherrelease/handler.go`
- Test: `backend/internal/launcherrelease/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Написать падающий тест хендлера**

`backend/internal/launcherrelease/handler_test.go`:

```go
package launcherrelease

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"launcher-backend/internal/events"

	"github.com/gofiber/fiber/v3"
)

func newTestApp(t *testing.T) (*fiber.App, Service, *events.Broker) {
	t.Helper()
	service := newTestService(t)
	broker := events.NewBroker()
	app := fiber.New(fiber.Config{BodyLimit: 512 * 1024 * 1024})
	passthrough := func(c fiber.Ctx) error { return c.Next() }
	NewHandler(service, broker).RegisterRoutes(app, passthrough)
	return app, service, broker
}

func TestCreateAndCheckUpdateViaHTTP(t *testing.T) {
	app, _, broker := newTestApp(t)

	// Подписываемся на брокер: создание релиза должно публиковать событие.
	subID, ch := broker.Subscribe()
	defer broker.Unsubscribe(subID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("version", "0.2.0")
	_ = writer.WriteField("changelog", "первый авто-релиз")
	_ = writer.WriteField("mandatory", "true")
	part, _ := writer.CreateFormFile("linux-x64", "launcher")
	_, _ = part.Write([]byte("fake-binary"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/admin/releases/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != 201 {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create status = %d, body = %s", res.StatusCode, raw)
	}

	select {
	case msg := <-ch:
		if msg != "launcher-release" {
			t.Fatalf("broker event = %q, want launcher-release", msg)
		}
	default:
		t.Fatal("broker event not published on release create")
	}

	// Проверка обновления старым клиентом.
	req = httptest.NewRequest("GET", "/api/launcher/update?platform=linux-x64&version=0.1.0", nil)
	res, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	var info UpdateInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "0.2.0" || !info.Mandatory {
		t.Fatalf("info = %+v", info)
	}

	// Скачивание бинарника.
	req = httptest.NewRequest("GET", "/api/launcher/download/0.2.0/linux-x64", nil)
	res, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("download status = %d", res.StatusCode)
	}
	raw, _ := io.ReadAll(res.Body)
	if string(raw) != "fake-binary" {
		t.Fatalf("downloaded = %q", raw)
	}
}

func TestCreateRejectsBadVersion(t *testing.T) {
	app, _, _ := newTestApp(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("version", "не-версия")
	part, _ := writer.CreateFormFile("linux-x64", "launcher")
	_, _ = part.Write([]byte("x"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/admin/releases/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/launcherrelease/`
Expected: FAIL (undefined: NewHandler).

- [ ] **Step 3: Реализация хендлера**

`backend/internal/launcherrelease/handler.go` (паттерн — `backend/internal/profiles/handler.go`):

```go
package launcherrelease

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

	"launcher-backend/internal/auth"
	"launcher-backend/internal/events"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// releaseEvent — payload SSE-события: лаунчер по нему запускает проверку
// обновления (см. stream_profile_events в launcher-slint).
const releaseEvent = "launcher-release"

type Handler struct {
	service Service
	broker  *events.Broker
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func NewHandler(service Service, broker *events.Broker) Handler {
	return Handler{service: service, broker: broker}
}

func (h Handler) RegisterRoutes(app *fiber.App, authMiddleware fiber.Handler) {
	// Публичные: проверка и скачивание обновления работают до логина.
	group := app.Group("/api/launcher")
	group.Get("/update", h.checkUpdate)
	group.Get("/download/:version/:platform", h.download)

	admin := app.Group("/api/admin/releases")
	admin.Use(authMiddleware, auth.RequireAdmin)
	admin.Get("/", h.list)
	admin.Post("/", h.create)
	admin.Patch("/:id", h.patch)
	admin.Delete("/:id", h.delete)
}

func (h Handler) notifyReleaseChanged() {
	if h.broker != nil {
		h.broker.Publish(releaseEvent)
	}
}

func (h Handler) checkUpdate(c fiber.Ctx) error {
	info, err := h.service.CheckUpdate(c.Context(), c.Query("platform"), c.Query("version", "0.0.0"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: err.Error()})
	}
	return c.JSON(info)
}

func (h Handler) download(c fiber.Ctx) error {
	abs, file, err := h.service.Download(c.Context(), c.Params("version"), c.Params("platform"))
	if err != nil {
		return h.writeError(c, err)
	}
	c.Set(fiber.HeaderContentDisposition, "attachment; filename=\""+safeHeaderFilename(file.FileName)+"\"")
	return c.SendFile(abs)
}

func (h Handler) list(c fiber.Ctx) error {
	releases, err := h.service.List(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{Message: "Не удалось получить релизы"})
	}
	return c.JSON(releases)
}

func (h Handler) create(c fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: "Некорректная multipart-форма"})
	}

	req := CreateRequest{
		Version:   formValue(form, "version"),
		Changelog: formValue(form, "changelog"),
		Mandatory: formValue(form, "mandatory") == "true",
	}

	files := make([]UploadedFile, 0, len(AllowedPlatforms))
	for _, platform := range AllowedPlatforms {
		headers := form.File[platform]
		if len(headers) == 0 {
			continue
		}
		opened, err := headers[0].Open()
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: "Не удалось прочитать файл " + platform})
		}
		defer opened.Close()
		files = append(files, UploadedFile{
			Platform: platform,
			FileName: headers[0].Filename,
			Reader:   opened,
		})
	}

	release, err := h.service.Create(c.Context(), req, files)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: err.Error()})
	}
	h.notifyReleaseChanged()
	return c.Status(http.StatusCreated).JSON(release)
}

func (h Handler) patch(c fiber.Ctx) error {
	var req PatchRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: "Некорректный JSON"})
	}
	release, err := h.service.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		return h.writeError(c, err)
	}
	h.notifyReleaseChanged()
	return c.JSON(release)
}

func (h Handler) delete(c fiber.Ctx) error {
	if err := h.service.Delete(c.Context(), c.Params("id")); err != nil {
		return h.writeError(c, err)
	}
	h.notifyReleaseChanged()
	return c.SendStatus(http.StatusNoContent)
}

func (h Handler) writeError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return c.Status(http.StatusNotFound).JSON(ErrorResponse{Message: "Запись не найдена"})
	case err != nil:
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: err.Error()})
	default:
		return nil
	}
}

func formValue(form *multipart.Form, key string) string {
	values := form.Value[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func safeHeaderFilename(name string) string {
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\"", "")
	if name == "" {
		return "launcher"
	}
	return name
}
```

- [ ] **Step 4: Регистрация в main.go**

В `backend/cmd/server/main.go`:

1. Импорт: добавить `"launcher-backend/internal/launcherrelease"` в блок импортов.
2. В `fiber.New(fiber.Config{...})` добавить поле (бинарники до 200 МБ; дефолтный BodyLimit fiber — 4 МБ):

```go
		// Лимит тела запроса: загрузка бинарников релизов лаунчера через админку.
		BodyLimit: 512 * 1024 * 1024,
```

3. После блока `profiles.NewHandler(...)` (строка ~84) добавить:

```go
	releaseService := launcherrelease.NewService(db, cfg.LauncherReleaseRoot)
	launcherrelease.NewHandler(releaseService, profilesBroker).
		RegisterRoutes(app, authService.RequireAuth())
```

(`releaseService` дальше используется в Task 5 для anticheat-гейта — пока компилятор не ругается, т.к. она передана в NewHandler.)

- [ ] **Step 5: Тесты проходят**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm sh -c "go vet ./... && go test ./internal/launcherrelease/"`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/launcherrelease/ backend/cmd/server/main.go
git commit -m "feat(releases): HTTP API релизов + SSE-событие launcher-release"
```

---

### Task 5: Серверный форс-апдейт — 426 в anticheat handshake/init (TDD)

**Files:**
- Modify: `backend/internal/anticheat/handler.go`
- Test: `backend/internal/anticheat/version_gate_test.go`
- Modify: `backend/cmd/server/main.go:104`

- [ ] **Step 1: Написать падающий тест**

`backend/internal/anticheat/version_gate_test.go`:

```go
package anticheat

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type fakeGate struct{ min string }

func (g fakeGate) MinMandatoryVersion(_ context.Context) (string, error) {
	return g.min, nil
}

func newGateApp(t *testing.T, min string) *fiber.App {
	t.Helper()
	app := fiber.New()
	passthrough := func(c fiber.Ctx) error { return c.Next() }
	NewHandler(nil).WithVersionGate(fakeGate{min: min}).RegisterRoutes(app, passthrough)
	return app
}

func postInit(t *testing.T, app *fiber.App, version string) int {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/anticheat/handshake/init", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if version != "" {
		req.Header.Set("X-Launcher-Version", version)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return res.StatusCode
}

func TestVersionGateBlocksOutdatedLauncher(t *testing.T) {
	app := newGateApp(t, "0.2.0")

	if status := postInit(t, app, "0.1.0"); status != 426 {
		t.Fatalf("outdated client status = %d, want 426", status)
	}
	// Без заголовка = легаси-лаунчер 0.1.0 — тоже блокируется.
	if status := postInit(t, app, ""); status != 426 {
		t.Fatalf("legacy client status = %d, want 426", status)
	}
	// Актуальная версия проходит гейт (и падает дальше на отсутствии юзера = 401).
	if status := postInit(t, app, "0.2.0"); status != 401 {
		t.Fatalf("current client status = %d, want 401 (прошёл гейт)", status)
	}
}

func TestVersionGateInactiveWithoutMandatory(t *testing.T) {
	// Нет обязательных релизов — гейт пропускает даже без заголовка.
	app := newGateApp(t, "")
	if status := postInit(t, app, ""); status != 401 {
		t.Fatalf("status = %d, want 401 (гейт неактивен)", status)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm go test ./internal/anticheat/ -run TestVersionGate`
Expected: FAIL (no method WithVersionGate).

- [ ] **Step 3: Реализация гейта**

В `backend/internal/anticheat/handler.go`:

1. В импорты добавить `"context"` и `"launcher-backend/internal/launcherrelease"`.
2. После `type Handler struct {...}` заменить определение и добавить:

```go
// VersionGate сообщает минимальную обязательную версию лаунчера
// (реализуется launcherrelease.Service). nil — форс-апдейт выключен.
type VersionGate interface {
	MinMandatoryVersion(ctx context.Context) (string, error)
}

type Handler struct {
	service     *Service
	versionGate VersionGate
}

// WithVersionGate включает серверный форс-апдейт: клиенты ниже минимальной
// обязательной версии не получают launch-token (426 Upgrade Required).
func (h Handler) WithVersionGate(gate VersionGate) Handler {
	h.versionGate = gate
	return h
}
```

(существующее поле `service *Service` сохраняется; `NewHandler` не меняется.)

3. В начало `func (h Handler) init(c fiber.Ctx) error` — ДО `auth.CurrentUser(c)` — добавить:

```go
	// Форс-апдейт: старый лаунчер не получает launch-token, пока не обновится.
	// Запрос без заголовка — легаси-версия (≤0.1.0), считается "0.0.0".
	if h.versionGate != nil {
		minVersion, err := h.versionGate.MinMandatoryVersion(c.Context())
		if err == nil && minVersion != "" {
			clientVersion := c.Get("X-Launcher-Version")
			if clientVersion == "" {
				clientVersion = "0.0.0"
			}
			if launcherrelease.CompareVersions(clientVersion, minVersion) < 0 {
				return c.Status(http.StatusUpgradeRequired).JSON(ErrorResponse{
					Message: "Требуется обновление лаунчера до версии " + minVersion,
				})
			}
		}
	}
```

- [ ] **Step 4: Подключить в main.go**

В `backend/cmd/server/main.go` заменить строку 104:

```go
	anticheat.NewHandler(acService).RegisterRoutes(app, authService.RequireAuth())
```

на:

```go
	anticheat.NewHandler(acService).
		WithVersionGate(releaseService).
		RegisterRoutes(app, authService.RequireAuth())
```

- [ ] **Step 5: Все Go-тесты проходят**

Run: `docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm sh -c "go vet ./... && go test ./..."`
Expected: PASS (в т.ч. существующие anticheat-тесты — гейт у них не задан, поведение не меняется).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/anticheat/ backend/cmd/server/main.go
git commit -m "feat(anticheat): 426-гейт по X-Launcher-Version для форс-апдейта"
```

---

### Task 6: Лаунчер — модуль updater.rs (TDD на сравнение версий)

**Files:**
- Create: `launcher-slint/src/updater.rs`
- Modify: `launcher-slint/src/main.rs:19` (объявление модуля)

- [ ] **Step 1: Создать модуль с тестами**

`launcher-slint/src/updater.rs`:

```rust
//! Самообновление лаунчера: проверка версии на бэкенде, фоновая загрузка
//! бинарника, проверка SHA-256 и подмена себя с перезапуском.
//!
//! Подмена: Linux — атомарный rename поверх работающего бинарника;
//! Windows — rename текущего exe в .old (разрешено) + rename нового на место.

use std::cmp::Ordering;
use std::fs;
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;

use reqwest::blocking::Client;
use serde::Deserialize;
use sha2::{Digest, Sha256};

/// Версия лаунчера, зашитая при сборке (Cargo.toml).
pub const CURRENT_VERSION: &str = env!("CARGO_PKG_VERSION");

/// Платформа в терминах бэкенда (storage/releases/<version>/<platform>).
pub fn platform() -> &'static str {
    if cfg!(target_os = "windows") {
        "windows-x64"
    } else {
        "linux-x64"
    }
}

#[derive(Debug, Deserialize, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct UpdateInfo {
    pub update_available: bool,
    #[serde(default)]
    pub latest_version: String,
    #[serde(default)]
    pub mandatory: bool,
    #[serde(default)]
    pub changelog: String,
    #[serde(default)]
    pub download_url: String,
    #[serde(default)]
    pub sha256: String,
    #[serde(default)]
    pub size: i64,
}

/// Посегментное сравнение версий "X.Y.Z"; отсутствующие и нечисловые
/// сегменты считаются нулями (зеркало CompareVersions на бэкенде).
pub fn compare_versions(a: &str, b: &str) -> Ordering {
    fn parse(version: &str) -> Vec<u64> {
        version
            .split('.')
            .map(|seg| seg.trim().parse::<u64>().unwrap_or(0))
            .collect()
    }
    let (a, b) = (parse(a), parse(b));
    for i in 0..a.len().max(b.len()) {
        let x = a.get(i).copied().unwrap_or(0);
        let y = b.get(i).copied().unwrap_or(0);
        if x != y {
            return x.cmp(&y);
        }
    }
    Ordering::Equal
}

/// Запрашивает у бэкенда сведения об обновлении для текущей версии и платформы.
pub fn check_update(api_url: &str) -> Result<UpdateInfo, String> {
    let client = Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
        .map_err(|_| "Не удалось создать HTTP-клиент.".to_string())?;
    let url = format!(
        "{}/api/launcher/update?platform={}&version={}",
        api_url.trim_end_matches('/'),
        platform(),
        CURRENT_VERSION
    );
    let response = client
        .get(url)
        .send()
        .map_err(|_| "Сервер обновлений недоступен.".to_string())?;
    if !response.status().is_success() {
        return Err(format!(
            "Проверка обновлений: HTTP {}",
            response.status().as_u16()
        ));
    }
    response
        .json::<UpdateInfo>()
        .map_err(|_| "Некорректный ответ сервера обновлений.".to_string())
}

fn exe_path() -> Result<PathBuf, String> {
    std::env::current_exe().map_err(|_| "Не удалось определить путь лаунчера.".to_string())
}

/// Временный файл рядом с бинарником: launcher(.exe) -> launcher.update.partial.
fn staging_path(exe: &Path) -> PathBuf {
    exe.with_extension("update.partial")
}

/// Скачивает обновление во временный файл рядом с бинарником и сверяет SHA-256.
/// Возвращает путь к подготовленному файлу. Ошибка создания временного файла
/// означает, что каталог лаунчера не доступен на запись (fallback на ручное
/// обновление).
pub fn download_and_stage(api_url: &str, info: &UpdateInfo) -> Result<PathBuf, String> {
    let exe = exe_path()?;
    let staged = staging_path(&exe);
    let mut out = fs::File::create(&staged).map_err(|_| {
        "Каталог лаунчера недоступен для записи — скачайте новую версию вручную.".to_string()
    })?;

    let client = Client::builder()
        .connect_timeout(Duration::from_secs(15))
        .tcp_keepalive(Duration::from_secs(20))
        .build()
        .map_err(|_| "Не удалось создать HTTP-клиент.".to_string())?;
    let url = format!("{}{}", api_url.trim_end_matches('/'), info.download_url);
    let mut response = client
        .get(url)
        .send()
        .map_err(|_| "Не удалось скачать обновление.".to_string())?;
    if !response.status().is_success() {
        let _ = fs::remove_file(&staged);
        return Err(format!(
            "Скачивание обновления: HTTP {}",
            response.status().as_u16()
        ));
    }

    let mut hasher = Sha256::new();
    let mut buffer = [0u8; 64 * 1024];
    loop {
        let read = match response.read(&mut buffer) {
            Ok(read) => read,
            Err(_) => {
                let _ = fs::remove_file(&staged);
                return Err("Обрыв скачивания обновления.".to_string());
            }
        };
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
        if out.write_all(&buffer[..read]).is_err() {
            let _ = fs::remove_file(&staged);
            return Err("Не удалось записать обновление на диск.".to_string());
        }
    }
    drop(out);

    let actual = format!("{:x}", hasher.finalize());
    if !actual.eq_ignore_ascii_case(info.sha256.trim()) {
        let _ = fs::remove_file(&staged);
        return Err("Контрольная сумма обновления не совпала.".to_string());
    }
    Ok(staged)
}

/// Подменяет текущий бинарник подготовленным файлом и перезапускает лаунчер.
/// При успехе не возвращается (process::exit).
pub fn apply_and_restart(staged: &Path) -> Result<(), String> {
    let exe = exe_path()?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(staged, fs::Permissions::from_mode(0o755))
            .map_err(|_| "Не удалось выставить права на обновление.".to_string())?;
        // На Linux rename поверх запущенного бинарника атомарен и разрешён.
        fs::rename(staged, &exe)
            .map_err(|_| "Не удалось заменить бинарник лаунчера.".to_string())?;
    }
    #[cfg(windows)]
    {
        // Windows не даёт перезаписать запущенный exe, но даёт переименовать его.
        let old = exe.with_extension("old");
        let _ = fs::remove_file(&old);
        fs::rename(&exe, &old)
            .map_err(|_| "Не удалось переименовать текущий лаунчер.".to_string())?;
        if fs::rename(staged, &exe).is_err() {
            // Откат: возвращаем старый бинарник на место.
            let _ = fs::rename(&old, &exe);
            return Err("Не удалось установить обновление.".to_string());
        }
    }

    Command::new(&exe).spawn().map_err(|_| {
        "Обновление установлено, но перезапуск не удался — запустите лаунчер вручную.".to_string()
    })?;
    std::process::exit(0);
}

/// Удаляет следы прошлых обновлений (вызывается при старте лаунчера).
/// Ошибки игнорируются: .old может ещё держать завершающийся старый процесс.
pub fn cleanup_leftovers() {
    if let Ok(exe) = exe_path() {
        let _ = fs::remove_file(exe.with_extension("old"));
        let _ = fs::remove_file(exe.with_extension("update.partial"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compare_versions_orders_numerically() {
        assert_eq!(compare_versions("1.0.0", "1.0.0"), Ordering::Equal);
        assert_eq!(compare_versions("0.1.0", "0.2.0"), Ordering::Less);
        assert_eq!(compare_versions("0.10.0", "0.9.0"), Ordering::Greater);
        assert_eq!(compare_versions("1.2", "1.2.0"), Ordering::Equal);
        assert_eq!(compare_versions("abc", "0.0.1"), Ordering::Less);
    }

    #[test]
    fn staging_path_is_sibling_of_exe() {
        let staged = staging_path(Path::new("/opt/launcher/launcher-slint"));
        assert_eq!(
            staged,
            PathBuf::from("/opt/launcher/launcher-slint.update.partial")
        );
        let staged_win = staging_path(Path::new("C:/launcher/launcher.exe"));
        assert!(staged_win.to_string_lossy().ends_with("launcher.update.partial"));
    }
}
```

- [ ] **Step 2: Объявить модуль**

В `launcher-slint/src/main.rs` после строки 19 (`mod anticheat;`) добавить:

```rust
mod updater;
```

- [ ] **Step 3: Тесты проходят**

Run: `cargo test -p launcher-slint`
Expected: PASS (новые тесты updater + существующие). Предупреждения о dead_code допустимы до Task 7 (интеграция).

- [ ] **Step 4: Commit**

```bash
git add launcher-slint/src/updater.rs launcher-slint/src/main.rs
git commit -m "feat(launcher): модуль updater — проверка, скачивание, самозамена"
```

---

### Task 7: Лаунчер — UI-баннер и блокировка «Играть» (app.slint)

**Files:**
- Modify: `launcher-slint/ui/app.slint`

- [ ] **Step 1: Добавить свойства и колбэк**

В `launcher-slint/ui/app.slint` в корневой компонент (рядом со строкой 412 `in property <string> anticheat-alert;`) добавить:

```slint
    // Автообновление: статус скачивания, готовность к перезапуску,
    // обязательность (блокирует кнопку «Играть» до перезапуска).
    in property <bool> update-ready;
    in property <bool> update-mandatory;
    in property <string> update-version;
    in property <string> update-status;
```

Рядом с колбэками (строка ~417 `callback play-requested();`):

```slint
    callback update-restart-requested();
```

- [ ] **Step 2: Заблокировать «Играть» при обязательном обновлении**

Строка ~1086, в `PlayButton` заменить:

```slint
                        enabled: root.has-profile && !root.is-syncing;
```

на:

```slint
                        enabled: root.has-profile && !root.is-syncing && !root.update-mandatory;
```

и строкой выше label заменить:

```slint
                        label: root.is-syncing ? "ЗАГРУЗКА..." : "ИГРАТЬ";
```

на:

```slint
                        label: root.update-mandatory ? "ТРЕБУЕТСЯ ОБНОВЛЕНИЕ"
                            : (root.is-syncing ? "ЗАГРУЗКА..." : "ИГРАТЬ");
```

- [ ] **Step 3: Добавить баннер обновления**

Вставить ПЕРЕД блоком полноэкранного уведомления античита (строка ~1519, `// Полноэкранное уведомление античита`). Используем абсолютное позиционирование (гоча Slint: layout как не-единственный child не фиксирует ширину):

```slint
    // Плавающий баннер автообновления (нижний правый угол, поверх контента).
    if root.update-status != "" || root.update-ready: Rectangle {
        width: 380px;
        height: root.update-mandatory ? 86px : 68px;
        x: parent.width - self.width - 20px;
        y: parent.height - self.height - 20px;
        background: #14141cf2;
        border-radius: 14px;
        border-width: 1px;
        border-color: root.update-ready ? #6fdd8b55 : #ffffff22;
        drop-shadow-blur: 24px;
        drop-shadow-color: #00000090;

        HorizontalLayout {
            padding: 14px;
            spacing: 12px;

            VerticalLayout {
                alignment: center;
                spacing: 4px;
                Text {
                    text: root.update-ready
                        ? "Обновление \{root.update-version} готово"
                        : root.update-status;
                    color: #ffffff;
                    font-size: 14px;
                    font-weight: 700;
                    overflow: elide;
                }
                if root.update-mandatory: Text {
                    text: "Обязательное обновление — игра недоступна до перезапуска";
                    color: #ffb86b;
                    font-size: 11px;
                    overflow: elide;
                }
            }

            if root.update-ready: Rectangle {
                width: 132px;
                height: 40px;
                border-radius: 10px;
                background: restart-area.has-hover ? #7be89a : #6fdd8b;
                animate background { duration: 160ms; easing: ease-out-quad; }
                restart-area := TouchArea {
                    width: parent.width;
                    height: parent.height;
                    mouse-cursor: pointer;
                    clicked => {
                        root.update-restart-requested();
                    }
                }
                Text {
                    text: "Перезапустить";
                    color: #0c0c12;
                    font-size: 14px;
                    font-weight: 800;
                    horizontal-alignment: center;
                    vertical-alignment: center;
                    width: parent.width;
                    height: parent.height;
                }
            }
        }
    }
```

- [ ] **Step 4: Компиляция**

Run: `cargo build -p launcher-slint`
Expected: успешная сборка (slint-build компилирует .slint; ошибки синтаксиса всплывут здесь).

- [ ] **Step 5: Commit**

```bash
git add launcher-slint/ui/app.slint
git commit -m "feat(launcher): UI-баннер обновления + блокировка Играть при mandatory"
```

---

### Task 8: Лаунчер — интеграция updater в main.rs + 426 в anticheat

**Files:**
- Modify: `launcher-slint/src/main.rs`
- Modify: `launcher-slint/src/anticheat/mod.rs`

- [ ] **Step 1: Глобальное состояние обновления и запуск проверки**

В `launcher-slint/src/main.rs`:

1. В импорты (строка 6) добавить `AtomicBool`:

```rust
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
```

и в строку 7 добавить `OnceLock`:

```rust
use std::sync::{Arc, Mutex, OnceLock};
```

2. После определения `struct RuntimeState` (строка ~45) добавить:

```rust
/// Состояние автообновления. Глобальное (OnceLock): проверка живёт дольше
/// сессии логина и дёргается из SSE-слушателя, периодики и старта.
struct UpdateShared {
    /// Защита от параллельных скачиваний при множественных триггерах.
    in_progress: AtomicBool,
    /// Скачанное и проверенное обновление: (инфо, путь к staged-файлу).
    staged: Mutex<Option<(updater::UpdateInfo, PathBuf)>>,
}

static UPDATE_SHARED: OnceLock<Arc<UpdateShared>> = OnceLock::new();

fn update_shared() -> Arc<UpdateShared> {
    UPDATE_SHARED
        .get_or_init(|| {
            Arc::new(UpdateShared {
                in_progress: AtomicBool::new(false),
                staged: Mutex::new(None),
            })
        })
        .clone()
}

/// Фоновая проверка обновления: запрос к бэкенду, скачивание и стейджинг.
/// Все UI-обновления — через invoke_from_event_loop. Повторный вызов во время
/// активной проверки игнорируется.
fn spawn_update_check(app_weak: Weak<AppWindow>, config: AppConfig) {
    let shared = update_shared();
    if shared.in_progress.swap(true, Ordering::SeqCst) {
        return;
    }
    thread::spawn(move || {
        run_update_check(&app_weak, &config, &shared);
        shared.in_progress.store(false, Ordering::SeqCst);
    });
}

fn run_update_check(app_weak: &Weak<AppWindow>, config: &AppConfig, shared: &Arc<UpdateShared>) {
    let info = match updater::check_update(&config.api_url) {
        Ok(info) => info,
        // Сервер недоступен — тихо ждём следующего триггера (старт/SSE/30 мин).
        Err(_) => return,
    };
    if !info.update_available {
        return;
    }

    // Уже скачали именно эту версию — только освежаем UI.
    let already_staged = shared
        .staged
        .lock()
        .ok()
        .and_then(|staged| {
            staged
                .as_ref()
                .map(|(stored, _)| stored.latest_version == info.latest_version)
        })
        .unwrap_or(false);
    if already_staged {
        set_update_ui(app_weak, &info, true, String::new());
        return;
    }

    set_update_ui(
        app_weak,
        &info,
        false,
        format!("Скачивается обновление {}…", info.latest_version),
    );

    match updater::download_and_stage(&config.api_url, &info) {
        Ok(staged_path) => {
            if let Ok(mut staged) = shared.staged.lock() {
                *staged = Some((info.clone(), staged_path));
            }
            set_update_ui(app_weak, &info, true, String::new());
        }
        Err(message) => {
            // Ошибка остаётся в баннере; повтор — по следующему триггеру.
            set_update_ui(app_weak, &info, false, message);
        }
    }
}

/// Пробрасывает состояние обновления в UI-свойства (из любого потока).
fn set_update_ui(app_weak: &Weak<AppWindow>, info: &updater::UpdateInfo, ready: bool, status: String) {
    let app_weak = app_weak.clone();
    let version = info.latest_version.clone();
    let mandatory = info.mandatory;
    let _ = slint::invoke_from_event_loop(move || {
        if let Some(app) = app_weak.upgrade() {
            app.set_update_ready(ready);
            app.set_update_mandatory(mandatory);
            app.set_update_version(version.into());
            app.set_update_status(status.into());
        }
    });
}

/// Колбэк «Перезапустить»: подменяет бинарник и перезапускает процесс.
fn register_update_restart_handler(app: &AppWindow) {
    let app_weak = app.as_weak();
    app.on_update_restart_requested(move || {
        let shared = update_shared();
        let staged = shared
            .staged
            .lock()
            .ok()
            .and_then(|staged| staged.clone());
        let Some((info, staged_path)) = staged else {
            return;
        };
        if let Err(message) = updater::apply_and_restart(&staged_path) {
            // Подмена не удалась: сбрасываем staged (файл мог быть повреждён).
            if let Ok(mut staged) = shared.staged.lock() {
                *staged = None;
            }
            let info = info.clone();
            if let Some(app) = app_weak.upgrade() {
                app.set_update_ready(false);
                app.set_update_mandatory(info.mandatory);
                app.set_update_status(message.into());
            }
        }
    });
}
```

3. В `fn main()` после строки `let app = AppWindow::new()?;` (строка ~274) добавить:

```rust
    // Автообновление: подчищаем следы прошлой установки и проверяем новую
    // версию при старте (до логина — эндпоинт публичный).
    updater::cleanup_leftovers();
```

4. В `fn main()` после `register_play_handler(...)` (строка ~297) добавить:

```rust
    register_update_restart_handler(&app);
    spawn_update_check(app.as_weak(), config.clone());
    start_periodic_update_check(app.as_weak(), config.clone());
```

ВАЖНО: `config` дальше передаётся по значению в `restore_saved_session(&app, config, state, session_generation)` — вызовы выше должны идти до неё (клоны).

5. Рядом с `start_profile_event_listener` добавить периодику:

```rust
/// Страховочный фоновый опрос обновлений раз в 30 минут (SSE — основной канал).
fn start_periodic_update_check(app_weak: Weak<AppWindow>, config: AppConfig) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(30 * 60));
        spawn_update_check(app_weak.clone(), config.clone());
    });
}
```

- [ ] **Step 2: Различать SSE-события**

В `stream_profile_events` (строка ~832) заменить:

```rust
        if line.starts_with("data:") {
            refresh_profiles_now(config, state, app_weak);
        }
```

на:

```rust
        if let Some(payload) = line.strip_prefix("data:") {
            // Брокер общий: профили шлют "profiles", релизы лаунчера —
            // "launcher-release". Незнакомый payload считаем профилями
            // (обратная совместимость).
            if payload.trim() == "launcher-release" {
                spawn_update_check(app_weak.clone(), config.clone());
            } else {
                refresh_profiles_now(config, state, app_weak);
            }
        }
```

- [ ] **Step 3: 426 и заголовок версии в anticheat/mod.rs**

В `launcher-slint/src/anticheat/mod.rs`:

1. Добавить тип ошибки (после `struct GuardOk`, строка ~42):

```rust
/// Ошибки handshake/init, различающие форс-апдейт и сетевые сбои:
/// UpdateRequired блокирует запуск, Network — fail-open (M1).
enum InitError {
    UpdateRequired(String),
    Network(String),
}
```

2. В `pre_launch_guard` (строка ~52) заменить match:

```rust
    match init_handshake(config, token, &hwid_hash, &detections) {
        Ok(result) => {
            if !result.allowed {
                let reason = if result.reason.is_empty() {
                    "Запуск заблокирован системой защиты.".to_string()
                } else {
                    result.reason
                };
                return Err(reason);
            }
            Ok(GuardOk {
                launch_token: result.launch_token,
                nonce: result.nonce,
            })
        }
        // Форс-апдейт (426): запуск блокируется до обновления лаунчера.
        Err(InitError::UpdateRequired(message)) => Err(message),
        // fail-open в M1: при сетевой ошибке не блокируем игрока.
        Err(InitError::Network(_)) => Ok(GuardOk {
            launch_token: String::new(),
            nonce: String::new(),
        }),
    }
```

3. `init_handshake` (строка ~115): сменить сигнатуру на `Result<InitResult, InitError>`, добавить заголовок версии и обработку 426:

```rust
fn init_handshake(
    config: &AppConfig,
    token: &str,
    hwid_hash: &str,
    detections: &[scan::Detection],
) -> Result<InitResult, InitError> {
    let client = http_client().map_err(InitError::Network)?;
    let url = format!(
        "{}/api/anticheat/handshake/init",
        config.api_url.trim_end_matches('/')
    );
    let body = serde_json::json!({
        "hwidHash": hwid_hash,
        "detections": detections,
    });
    let response = client
        .post(url)
        .bearer_auth(token)
        // Серверный форс-апдейт: бэкенд отвечает 426, если версия ниже
        // минимальной обязательной.
        .header("X-Launcher-Version", env!("CARGO_PKG_VERSION"))
        .json(&body)
        .send()
        .map_err(|_| InitError::Network("init request failed".to_string()))?;

    let status = response.status();
    // 426 = требуется обновление лаунчера: блокируем запуск с сообщением сервера.
    if status.as_u16() == 426 {
        let message = response
            .json::<serde_json::Value>()
            .ok()
            .and_then(|value| {
                value
                    .get("message")
                    .and_then(|m| m.as_str())
                    .map(String::from)
            })
            .unwrap_or_else(|| "Требуется обновление лаунчера.".to_string());
        return Err(InitError::UpdateRequired(message));
    }
    // 403 = запуск заблокирован: тело содержит причину (allowed:false).
    if status.as_u16() == 403 || status.is_success() {
        return response
            .json::<InitResult>()
            .map_err(|_| InitError::Network("init parse failed".to_string()));
    }
    Err(InitError::Network(format!("init http {}", status.as_u16())))
}
```

- [ ] **Step 4: Сборка и тесты**

Run: `cargo test -p launcher-slint && cargo build -p launcher-slint`
Expected: PASS, сборка успешна. Если `staged.clone()` в `register_update_restart_handler` не компилируется — добавить `#[derive(Clone)]` уже есть у `UpdateInfo`; кортеж `(UpdateInfo, PathBuf)` клонируется автоматически.

- [ ] **Step 5: Commit**

```bash
git add launcher-slint/src/main.rs launcher-slint/src/anticheat/mod.rs
git commit -m "feat(launcher): интеграция автообновления — старт/SSE/периодика, 426-обработка"
```

---

### Task 9: Dashboard — типы + multipart-хелпер

**Files:**
- Modify: `dashboard/app/lib/types.ts`
- Modify: `dashboard/app/lib/api.ts`

- [ ] **Step 1: Типы**

В конец `dashboard/app/lib/types.ts` добавить:

```ts
export type LauncherReleaseFile = {
  id: string;
  releaseId: string;
  platform: string;
  fileName: string;
  hashSha256: string;
  size: number;
};

export type LauncherRelease = {
  id: string;
  version: string;
  changelog: string;
  mandatory: boolean;
  isActive: boolean;
  createdAt: string;
  files: LauncherReleaseFile[];
};
```

- [ ] **Step 2: apiUpload**

В конец `dashboard/app/lib/api.ts` добавить:

```ts
/** Multipart-загрузка с прогрессом. XHR: fetch не отдаёт upload-progress. */
export function apiUpload<T = unknown>(
  path: string,
  form: FormData,
  onProgress?: (fraction: number) => void
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${apiUrl}${path}`);
    xhr.setRequestHeader('Accept', 'application/json');
    const token = getToken();
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && onProgress) onProgress(event.loaded / event.total);
    };
    xhr.onerror = () => reject(new ApiError('Backend недоступен', 0));
    xhr.onload = () => {
      if (xhr.status === 401) {
        clearToken();
        window.location.href = '/login';
        reject(new ApiError('Сессия истекла', 401));
        return;
      }
      let data: { message?: string } = {};
      try {
        data = JSON.parse(xhr.responseText) as { message?: string };
      } catch {
        // пустой или не-JSON ответ — допустимо для 2xx
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(data as T);
      } else {
        reject(new ApiError(data.message ?? `Ошибка ${xhr.status}`, xhr.status));
      }
    };
    xhr.send(form);
  });
}
```

- [ ] **Step 3: Тайпчек**

Run: `cd dashboard && npm run build`
Expected: успешная сборка.

- [ ] **Step 4: Commit**

```bash
git add dashboard/app/lib/types.ts dashboard/app/lib/api.ts
git commit -m "feat(dashboard): тип LauncherRelease + apiUpload с прогрессом"
```

---

### Task 10: Dashboard — страница «Релизы»

**Files:**
- Create: `dashboard/components/releases/release-form.tsx`
- Create: `dashboard/components/releases/release-list.tsx`
- Create: `dashboard/app/releases/page.tsx`
- Modify: `dashboard/components/shell/sidebar.tsx:10-16`

- [ ] **Step 1: Форма создания релиза**

`dashboard/components/releases/release-form.tsx`:

```tsx
'use client';

// Форма нового релиза лаунчера: версия, changelog, флаг «обязательный»,
// бинарники по платформам. Загрузка multipart с прогрессом (apiUpload).

import { useState } from 'react';
import { Button } from '../ui/button';
import { Card } from '../ui/card';
import { Field } from '../ui/field';
import { Input, TextArea } from '../ui/input';
import { useToast } from '../ui/toast';
import { apiUpload, errorMessage } from '../../app/lib/api';

const platforms = [
  { key: 'linux-x64', label: 'Linux x64' },
  { key: 'windows-x64', label: 'Windows x64' }
] as const;

export function ReleaseForm({ onCreated }: { onCreated: () => void }) {
  const toast = useToast();
  const [version, setVersion] = useState('');
  const [changelog, setChangelog] = useState('');
  const [mandatory, setMandatory] = useState(false);
  const [files, setFiles] = useState<Record<string, File | null>>({
    'linux-x64': null,
    'windows-x64': null
  });
  const [progress, setProgress] = useState<number | null>(null);

  const selectedCount = Object.values(files).filter(Boolean).length;
  const canSubmit = /^\d+\.\d+\.\d+$/.test(version.trim()) && selectedCount > 0 && progress === null;

  async function submit() {
    const form = new FormData();
    form.set('version', version.trim());
    form.set('changelog', changelog.trim());
    form.set('mandatory', mandatory ? 'true' : 'false');
    for (const platform of platforms) {
      const file = files[platform.key];
      if (file) form.set(platform.key, file);
    }

    setProgress(0);
    try {
      await apiUpload('/api/admin/releases/', form, setProgress);
      toast('success', `Релиз ${version.trim()} опубликован`);
      setVersion('');
      setChangelog('');
      setMandatory(false);
      setFiles({ 'linux-x64': null, 'windows-x64': null });
      onCreated();
    } catch (error) {
      toast('error', errorMessage(error));
    } finally {
      setProgress(null);
    }
  }

  return (
    <Card className="flex flex-col gap-4">
      <h2 className="text-sm font-bold uppercase tracking-wide text-fg-muted">Новый релиз</h2>

      <Field label="Версия" hint="Формат X.Y.Z, должна совпадать с version в Cargo.toml сборки">
        <Input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="0.2.0" />
      </Field>

      <Field label="Changelog">
        <TextArea
          value={changelog}
          onChange={(e) => setChangelog(e.target.value)}
          placeholder="Что нового в этой версии"
        />
      </Field>

      {platforms.map((platform) => (
        <Field key={platform.key} label={`Бинарник ${platform.label}`}>
          <input
            type="file"
            onChange={(e) => setFiles((current) => ({ ...current, [platform.key]: e.target.files?.[0] ?? null }))}
            className="block w-full text-sm text-fg-secondary file:mr-3 file:rounded-lg file:border file:border-edge file:bg-surface file:px-3 file:py-2 file:text-sm file:font-semibold file:text-fg hover:file:bg-surface-strong"
          />
        </Field>
      ))}

      <label className="flex items-center gap-2 text-sm text-fg-secondary">
        <input type="checkbox" checked={mandatory} onChange={(e) => setMandatory(e.target.checked)} />
        Обязательный — старые лаунчеры не смогут запускать игру, пока не обновятся
      </label>

      {progress !== null && (
        <div className="h-2 w-full overflow-hidden rounded-full bg-surface-strong">
          <div className="h-full bg-fg transition-all" style={{ width: `${Math.round(progress * 100)}%` }} />
        </div>
      )}

      <Button variant="primary" disabled={!canSubmit} loading={progress !== null} onClick={() => void submit()}>
        Опубликовать релиз
      </Button>
    </Card>
  );
}
```

- [ ] **Step 2: Таблица релизов**

`dashboard/components/releases/release-list.tsx`:

```tsx
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
```

- [ ] **Step 3: Страница**

`dashboard/app/releases/page.tsx` (паттерн — `dashboard/app/profiles/page.tsx`):

```tsx
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
```

- [ ] **Step 4: Пункт меню**

В `dashboard/components/shell/sidebar.tsx`:

- строка 7: добавить `Rocket` в импорт из `lucide-react`;
- в `navItems` после пункта «Новости» добавить:

```ts
  { href: '/releases', label: 'Релизы', icon: Rocket },
```

(командная палитра читает navItems — новый пункт появится и там).

- [ ] **Step 5: Тайпчек**

Run: `cd dashboard && npm run build`
Expected: успешная сборка.

- [ ] **Step 6: Commit**

```bash
git add dashboard/components/releases/ dashboard/app/releases/ dashboard/components/shell/sidebar.tsx
git commit -m "feat(dashboard): страница Релизы — публикация и управление версиями лаунчера"
```

---

### Task 11: Финальная верификация + версия 0.2.0 + документация

**Files:**
- Modify: `launcher-slint/Cargo.toml:3`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Поднять версию лаунчера**

В `launcher-slint/Cargo.toml` заменить `version = "0.1.0"` на `version = "0.2.0"` — первый релиз с автообновлением. (Версии 0.1.0 без заголовка X-Launcher-Version считаются легаси.)

- [ ] **Step 2: Обновить CLAUDE.md**

В секцию «Архитектура бэкенда» (список доменных пакетов) добавить пункт после `news`:

```markdown
- `launcherrelease` — релизы лаунчера (автообновление). Бинарники в
  `backend/storage/releases/<version>/<platform>/`, заливка через дашборд
  (multipart, лимит 200 МБ/файл — следи за client_max_body_size в nginx на VPS).
  Публичные `/api/launcher/update|download`; событие `launcher-release` идёт
  через общий SSE-брокер профилей. Обязательные релизы: anticheat handshake/init
  отвечает 426 клиентам ниже минимальной mandatory-версии (X-Launcher-Version).
```

В секцию «Десктоп-лаунчер (launcher-slint)» добавить:

```markdown
Автообновление: `src/updater.rs` — проверка при старте/по SSE/раз в 30 мин,
скачивание в `<exe>.update.partial`, SHA-256, самозамена (Linux: rename поверх;
Windows: exe→.old + rename) и перезапуск по кнопке. Версия = `CARGO_PKG_VERSION`.
```

- [ ] **Step 3: Полная верификация**

```bash
docker run --rm -v "$PWD/backend":/src -w /src golang:1.26-bookworm sh -c "go vet ./... && go test ./..."
cargo test -p launcher-slint && cargo build -p launcher-slint
cd dashboard && npm run build
```
Expected: всё PASS / успешные сборки.

- [ ] **Step 4: Ручная smoke-проверка (опционально, через ./dev.sh)**

1. `./dev.sh`, залогиниться в дашборд, открыть «Релизы».
2. Опубликовать релиз 0.3.0 с любым файлом как linux-x64.
3. `GET http://127.0.0.1:8080/api/launcher/update?platform=linux-x64&version=0.2.0` → `updateAvailable: true`.
4. Запустить лаунчер (`npm run dev:launcher`) — должен появиться баннер скачивания/готовности.

- [ ] **Step 5: Commit**

```bash
git add launcher-slint/Cargo.toml CLAUDE.md
git commit -m "chore: версия лаунчера 0.2.0 + документация автообновления"
```

---

## Заметки для деплоя (не часть кода)

- На VPS nginx проксирует backend (`:8082`): для заливки релизов нужен
  `client_max_body_size 512m;` в конфиге nginx (прод-only файл, правится руками
  на VPS — `reset --hard` его не трогает).
- `backend/storage/releases/` создастся автоматически при первой заливке;
  в git не входит, деплоем не затирается (как `storage/profiles`).
- Windows-бинарник собирается отдельно (кросс-сборка или сборка на Windows) —
  вне скоупа этого плана; страница «Релизы» принимает его, когда он появится.
