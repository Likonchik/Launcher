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
