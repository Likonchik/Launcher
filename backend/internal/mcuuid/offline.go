package mcuuid

import (
	"crypto/md5"
	"fmt"

	"github.com/google/uuid"
)

// OfflinePlayerUUID — тот же UUID, что у игрока в offline-режиме Java Edition:
// UUID.nameUUIDFromBytes("OfflinePlayer:"+username в UTF-8).
// Ник должен совпадать с тем, под которым клиент заходит в игру (здесь — users.username / Login в GML).
func OfflinePlayerUUID(username string) (uuid.UUID, error) {
	if username == "" {
		return uuid.UUID{}, fmt.Errorf("пустой ник")
	}
	h := md5.Sum([]byte("OfflinePlayer:" + username))
	b := h[:]
	// версия 3, вариант RFC 4122 — как в java.util.UUID.nameUUIDFromBytes
	b[6] = (b[6] & 0x0f) | 0x30
	b[8] = (b[8] & 0x3f) | 0x80
	return uuid.UUID(b), nil
}

// OfflinePlayerUUIDString возвращает UUID в виде строки с дефисами (нижний регистр hex).
func OfflinePlayerUUIDString(username string) (string, error) {
	u, err := OfflinePlayerUUID(username)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
