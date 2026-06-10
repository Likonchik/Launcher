package models

import "time"

// YggdrasilSession — персист игровой сессии: переживает рестарт backend,
// чтобы игроков не выкидывало с серверов при деплое.
type YggdrasilSession struct {
	AccessToken string `gorm:"primaryKey;size:64"`
	ClientToken string `gorm:"size:64"`
	UUID        string `gorm:"size:64"`
	Name        string `gorm:"size:64"`
	Nonce       string `gorm:"size:64;index"`
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
