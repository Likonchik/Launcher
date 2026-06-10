package anticheat

import (
	"context"
	"testing"

	"launcher-backend/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.Detection{}, &models.Hwid{}, &models.HwidBan{}, &models.AccountBan{}, &models.CheatSignature{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInitHandshakeIssuesToken(t *testing.T) {
	svc := NewService(newTestDB(t), "secret", false, nil, "")
	ctx := context.Background()

	res, err := svc.InitHandshake(ctx, "uuid-1", "Liko", "hwid-1", nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !res.Allowed || res.LaunchToken == "" || res.Nonce == "" {
		t.Fatalf("ожидался разрешённый запуск с токеном: %+v", res)
	}
	claims, err := svc.VerifyToken(res.LaunchToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UUID != "uuid-1" || claims.Nonce != res.Nonce {
		t.Fatalf("claims не совпали: %+v", claims)
	}
}

func TestInitHandshakeBlocksBannedAccount(t *testing.T) {
	svc := NewService(newTestDB(t), "secret", false, nil, "")
	ctx := context.Background()

	if err := svc.BanAccount(ctx, "uuid-ban", "Cheater", "x-ray", "admin"); err != nil {
		t.Fatalf("ban: %v", err)
	}
	res, err := svc.InitHandshake(ctx, "uuid-ban", "Cheater", "hwid-2", nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if res.Allowed || res.LaunchToken != "" {
		t.Fatalf("забаненный аккаунт не должен получать токен: %+v", res)
	}
}

func TestInitHandshakeBlocksBannedHwid(t *testing.T) {
	svc := NewService(newTestDB(t), "secret", false, nil, "")
	ctx := context.Background()

	if err := svc.BanHwid(ctx, "hwid-bad", "cheat device", "admin"); err != nil {
		t.Fatalf("ban: %v", err)
	}
	res, _ := svc.InitHandshake(ctx, "uuid-3", "Bob", "hwid-bad", nil)
	if res.Allowed {
		t.Fatal("забаненный HWID не должен получать токен")
	}
}

func TestInitRecordsHwidAndDetections(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, "secret", false, nil, "")
	ctx := context.Background()

	_, err := svc.InitHandshake(ctx, "uuid-4", "Eve", "hwid-4", []DetectionInput{
		{Type: "process", Signature: "cheatengine.exe", Severity: 6},
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var hwidCount, detCount int64
	db.Model(&models.Hwid{}).Where("hash = ?", "hwid-4").Count(&hwidCount)
	db.Model(&models.Detection{}).Where("user_uuid = ?", "uuid-4").Count(&detCount)
	if hwidCount != 1 {
		t.Fatalf("HWID не записан: %d", hwidCount)
	}
	if detCount != 1 {
		t.Fatalf("детект не записан: %d", detCount)
	}
}

type fakeVerifier struct {
	verified map[string]bool
}

func (f *fakeVerifier) MarkVerifiedByNonce(nonce string) bool {
	if f.verified[nonce] {
		return false // одноразовость
	}
	f.verified[nonce] = true
	return nonce != ""
}

func (f *fakeVerifier) InvalidateByNonce(nonce string) bool { return nonce != "" }

func TestConfirmMarksSessionVerified(t *testing.T) {
	verifier := &fakeVerifier{verified: map[string]bool{}}
	svc := NewService(newTestDB(t), "secret", false, verifier, "")
	ctx := context.Background()

	res, _ := svc.InitHandshake(ctx, "uuid-c", "Liko", "hwid-c", nil)
	if err := svc.Confirm(res.LaunchToken); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !verifier.verified[res.Nonce] {
		t.Fatal("confirm должен пометить сессию по nonce")
	}
	// Повторный confirm по тому же токену/nonce не проходит.
	if err := svc.Confirm(res.LaunchToken); err == nil {
		t.Fatal("повторный confirm должен быть отклонён")
	}
}

func TestConfirmRejectsBadToken(t *testing.T) {
	svc := NewService(newTestDB(t), "secret", false, &fakeVerifier{verified: map[string]bool{}}, "")
	if err := svc.Confirm("garbage.token"); err == nil {
		t.Fatal("невалидный токен не должен подтверждаться")
	}
}

func TestAutoBanOnHighSeverity(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, "secret", true, nil, "") // autoBan включён
	ctx := context.Background()

	res, _ := svc.InitHandshake(ctx, "uuid-5", "Mal", "hwid-5", nil)
	claims, _ := svc.VerifyToken(res.LaunchToken)
	if err := svc.RecordDetection(ctx, claims, DetectionInput{Type: "jar", Signature: "baritone", Severity: 9}); err != nil {
		t.Fatalf("detect: %v", err)
	}

	var accBans, hwidBans int64
	db.Model(&models.AccountBan{}).Where("user_uuid = ?", "uuid-5").Count(&accBans)
	db.Model(&models.HwidBan{}).Where("hwid_hash = ?", "hwid-5").Count(&hwidBans)
	if accBans != 1 || hwidBans != 1 {
		t.Fatalf("ожидался авто-бан аккаунта и HWID: acc=%d hwid=%d", accBans, hwidBans)
	}
}
