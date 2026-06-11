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
