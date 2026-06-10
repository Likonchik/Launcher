package yggdrasil

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"launcher-backend/internal/models"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
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
