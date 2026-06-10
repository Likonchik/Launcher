package models

import "time"

// Detection — зафиксированное античитом обнаружение (от лаунчера или агента).
// Привязано к игроку (UserUUID — нормализованный Yggdrasil-UUID) и по возможности
// к железу (HwidHash) и игровой сессии (SessionID — accessToken-хэш).
type Detection struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserUUID  string    `gorm:"size:64;index;not null" json:"userUuid"`
	Login     string    `gorm:"size:64;index" json:"login"`
	HwidHash  string    `gorm:"size:64;index" json:"hwidHash"`
	SessionID string    `gorm:"size:64;index" json:"sessionId"`
	Source    string    `gorm:"size:16;not null;default:launcher" json:"source"` // launcher|java|native
	Type      string    `gorm:"size:32;not null" json:"type"`                    // process|class|jar|file|attach|debugger|tamper
	Signature string    `gorm:"size:255" json:"signature"`                       // что именно сработало
	Severity  int       `gorm:"not null;default:1" json:"severity"`              // 1..10
	Raw       string    `gorm:"type:text" json:"raw"`                            // JSON с деталями
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// Hwid — известный аппаратный отпечаток и история его появления.
type Hwid struct {
	Hash          string    `gorm:"size:64;primaryKey" json:"hash"`
	FirstUserUUID string    `gorm:"size:64;index" json:"firstUserUuid"`
	FirstLogin    string    `gorm:"size:64" json:"firstLogin"`
	SeenCount     int64     `gorm:"not null;default:0" json:"seenCount"`
	FirstSeen     time.Time `json:"firstSeen"`
	LastSeen      time.Time `json:"lastSeen"`
}

// HwidBan — аппаратный бан. Активен, если ExpiresAt == nil (перманентно) или в будущем.
type HwidBan struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	HwidHash  string     `gorm:"size:64;uniqueIndex;not null" json:"hwidHash"`
	Reason    string     `gorm:"size:255" json:"reason"`
	BannedBy  string     `gorm:"size:64" json:"bannedBy"` // login админа
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// AccountBan — бан аккаунта по нормализованному UUID.
type AccountBan struct {
	ID        string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserUUID  string     `gorm:"size:64;uniqueIndex;not null" json:"userUuid"`
	Login     string     `gorm:"size:64;index" json:"login"`
	Reason    string     `gorm:"size:255" json:"reason"`
	BannedBy  string     `gorm:"size:64" json:"bannedBy"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// CheatSignature — запись блэклиста, по которой лаунчер и агенты ищут читы.
// Kind задаёт, что сопоставляется; матч ведётся по Pattern (имя/подстрока) или HashHex.
type CheatSignature struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	Kind      string    `gorm:"size:16;not null" json:"kind"` // process|class|jar|file
	Pattern   string    `gorm:"size:255" json:"pattern"`      // имя/подстрока (lowercase)
	HashHex   string    `gorm:"size:64;index" json:"hashHex"` // SHA-256, если матч по хэшу
	Severity  int       `gorm:"not null;default:5" json:"severity"`
	Note      string    `gorm:"size:255" json:"note"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
