// Команда import-mariadb — разовый перенос аккаунтов из старой БД Telegram-бота
// (MariaDB/MySQL, целочисленные id) в общую БД монорепо (PostgreSQL/SQLite, uuid).
//
// Что переносится: users, auth_logs, telegram_login_otps, bot_audit_logs.
// Что НЕ переносится: game_profiles (игровая статистика удалена из проекта),
// sessions и telegram_bot_dialogue (эфемерны, пересоздадутся при работе).
//
// Словарь intID→uuid строится по users.uuid (offline-UUID Minecraft); если в строке
// uuid пуст — вычисляется из username. Все внешние ключи переводятся на uuid.
//
// Запуск (из каталога backend, где доступен .env с DATABASE_URL приёмника):
//
//	MARIADB_URL='mysql://user:pass@127.0.0.1:3306/launcher_accounts' \
//	DATABASE_URL='postgres://launcher:launcher_dev_password@127.0.0.1:5432/launcher?sslmode=disable' \
//	go run ./cmd/import-mariadb
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"launcher-backend/internal/config"
	"launcher-backend/internal/database"
	"launcher-backend/internal/mcuuid"
	"launcher-backend/internal/models"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("import failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	mariaURL := strings.TrimSpace(os.Getenv("MARIADB_URL"))
	if mariaURL == "" {
		return fmt.Errorf("MARIADB_URL не задан (исходная БД бота, формат mysql://user:pass@host:3306/db)")
	}

	src, err := openMySQL(mariaURL)
	if err != nil {
		return fmt.Errorf("подключение к MariaDB: %w", err)
	}
	defer src.Close()

	dst, err := database.Open(config.Load())
	if err != nil {
		return fmt.Errorf("подключение к приёмнику: %w", err)
	}
	if err := database.AutoMigrate(dst); err != nil {
		return fmt.Errorf("миграция приёмника: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	dict, usersImported, usersSkipped, err := importUsers(ctx, src, dst, log)
	if err != nil {
		return fmt.Errorf("перенос users: %w", err)
	}
	log.Info("users", "перенесено", usersImported, "пропущено_дубли", usersSkipped, "в_словаре", len(dict))

	authN, err := importAuthLogs(ctx, src, dst, dict)
	if err != nil {
		return fmt.Errorf("перенос auth_logs: %w", err)
	}
	log.Info("auth_logs", "перенесено", authN)

	otpN, err := importOTPs(ctx, src, dst, dict)
	if err != nil {
		return fmt.Errorf("перенос telegram_login_otps: %w", err)
	}
	log.Info("telegram_login_otps", "перенесено", otpN)

	auditN, err := importAudit(ctx, src, dst, dict)
	if err != nil {
		return fmt.Errorf("перенос bot_audit_logs: %w", err)
	}
	log.Info("bot_audit_logs", "перенесено", auditN)

	log.Info("импорт завершён", "game_profiles", "пропущены (удалены из проекта)")
	return nil
}

// importUsers переносит аккаунты и возвращает словарь intID→uuid.
func importUsers(ctx context.Context, src *sql.DB, dst *gorm.DB, log *slog.Logger) (map[int64]string, int, int, error) {
	rows, err := src.QueryContext(ctx, `
SELECT id, uuid, username, email, password, role,
       createdAt, updatedAt, lastLogin, ipAddress, hardwareId,
       isBanned, isHwidBanned, telegramId, telegramUsername, telegramLinkedAt,
       totpSecret, totpEnabled
FROM users ORDER BY id`)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	dict := make(map[int64]string)
	imported, skipped := 0, 0

	for rows.Next() {
		var (
			id                                  int64
			uuid, username, email, password, role string
			createdAt, updatedAt                time.Time
			lastLogin                           sql.NullTime
			ipAddress, hardwareID               sql.NullString
			isBanned, isHwidBanned, totpEnabled bool
			telegramID                          sql.NullInt64
			telegramUsername                    sql.NullString
			telegramLinkedAt                    sql.NullTime
			totpSecret                          sql.NullString
		)
		if err := rows.Scan(&id, &uuid, &username, &email, &password, &role,
			&createdAt, &updatedAt, &lastLogin, &ipAddress, &hardwareID,
			&isBanned, &isHwidBanned, &telegramID, &telegramUsername, &telegramLinkedAt,
			&totpSecret, &totpEnabled); err != nil {
			return nil, 0, 0, err
		}

		uid := normalizeUUID(uuid, username)
		dict[id] = uid

		u := models.User{
			ID:               uid,
			Login:            username,
			ProviderUUID:     uid,
			Email:            email,
			PasswordHash:     password,
			Role:             normalizeRole(role),
			IsBanned:         isBanned,
			IsHwidBanned:     isHwidBanned,
			HardwareID:       hardwareID.String,
			IPAddress:        ipAddress.String,
			TelegramUsername: telegramUsername.String,
			TOTPSecret:       totpSecret.String,
			TOTPEnabled:      totpEnabled,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}
		if telegramID.Valid {
			tg := telegramID.Int64
			u.TelegramID = &tg
		}
		if telegramLinkedAt.Valid {
			t := telegramLinkedAt.Time
			u.TelegramLinkedAt = &t
		}
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}

		// Идемпотентность: пропускаем уже существующий uuid.
		res := dst.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}}, DoNothing: true,
		}).Create(&u)
		if res.Error != nil {
			// Конфликт по другому уникальному индексу (login/email/telegram_id) — пропускаем строку.
			log.Warn("пропуск user", "id", id, "login", username, "err", res.Error)
			skipped++
			continue
		}
		if res.RowsAffected == 0 {
			skipped++
		} else {
			imported++
		}
	}
	return dict, imported, skipped, rows.Err()
}

func importAuthLogs(ctx context.Context, src *sql.DB, dst *gorm.DB, dict map[int64]string) (int, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT userId, username, ip, source, success, message, createdAt FROM auth_logs ORDER BY id`)
	if err != nil {
		// Таблицы может не быть — не критично.
		return 0, nil
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var (
			userID            sql.NullInt64
			username, source  string
			ip, message       sql.NullString
			success           bool
			createdAt         time.Time
		)
		if err := rows.Scan(&userID, &username, &ip, &source, &success, &message, &createdAt); err != nil {
			return n, err
		}
		log := models.AuthLog{
			Username: username, IP: ip.String, Source: source,
			Success: success, Message: message.String, CreatedAt: createdAt,
		}
		if userID.Valid {
			if uid, ok := dict[userID.Int64]; ok {
				log.UserID = &uid
			}
		}
		if err := dst.WithContext(ctx).Create(&log).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func importOTPs(ctx context.Context, src *sql.DB, dst *gorm.DB, dict map[int64]string) (int, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT id, userId, codeHash, expiresAt, consumedAt, telegramChatId, purpose FROM telegram_login_otps ORDER BY id`)
	if err != nil {
		return 0, nil
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var (
			id, codeHash, purpose string
			userID                int64
			expiresAt             time.Time
			consumedAt            sql.NullTime
			chatID                int64
		)
		if err := rows.Scan(&id, &userID, &codeHash, &expiresAt, &consumedAt, &chatID, &purpose); err != nil {
			return n, err
		}
		uid, ok := dict[userID]
		if !ok {
			continue // владелец не перенесён
		}
		otp := models.TelegramOTP{
			ID: id, UserID: uid, CodeHash: codeHash, ExpiresAt: expiresAt,
			TelegramChatID: chatID, Purpose: purpose,
		}
		if consumedAt.Valid {
			t := consumedAt.Time
			otp.ConsumedAt = &t
		}
		if err := dst.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&otp).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func importAudit(ctx context.Context, src *sql.DB, dst *gorm.DB, dict map[int64]string) (int, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT id, adminTelegramId, adminUserId, targetUserId, action, details, createdAt FROM bot_audit_logs ORDER BY createdAt`)
	if err != nil {
		return 0, nil
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var (
			id, action            string
			adminTelegramID       sql.NullInt64
			adminUserID, targetID sql.NullInt64
			details               sql.NullString
			createdAt             time.Time
		)
		if err := rows.Scan(&id, &adminTelegramID, &adminUserID, &targetID, &action, &details, &createdAt); err != nil {
			return n, err
		}
		a := models.BotAuditLog{ID: id, Action: action, Details: details.String, CreatedAt: createdAt}
		if adminTelegramID.Valid {
			v := adminTelegramID.Int64
			a.AdminTelegramID = &v
		}
		if adminUserID.Valid {
			if uid, ok := dict[adminUserID.Int64]; ok {
				a.AdminUserID = &uid
			}
		}
		if targetID.Valid {
			if uid, ok := dict[targetID.Int64]; ok {
				a.TargetUserID = &uid
			}
		}
		if err := dst.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

// normalizeUUID берёт существующий uuid (приведённый к нижнему регистру) либо строит offline-UUID из ника.
func normalizeUUID(raw, username string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return strings.ToLower(raw)
	}
	if u, err := mcuuid.OfflinePlayerUUIDString(username); err == nil {
		return u
	}
	return raw
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return models.RoleAdmin
	case "moderator":
		return models.RoleModerator
	default:
		return models.RoleUser
	}
}

// openMySQL принимает mysql://user:pass@host:port/db или нативный DSN.
func openMySQL(raw string) (*sql.DB, error) {
	dsn, err := toMySQLDSN(raw)
	if err != nil {
		return nil, err
	}
	con, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	con.SetMaxOpenConns(5)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := con.PingContext(ctx); err != nil {
		_ = con.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return con, nil
}

func toMySQLDSN(raw string) (string, error) {
	if !strings.HasPrefix(raw, "mysql://") {
		if !strings.Contains(raw, "parseTime=") {
			sep := "?"
			if strings.Contains(raw, "?") {
				sep = "&"
			}
			raw += sep + "parseTime=true&loc=UTC"
		}
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	pass, _ := u.User.Password()
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	q := u.Query()
	if q.Get("parseTime") == "" {
		q.Set("parseTime", "true")
	}
	if q.Get("loc") == "" {
		q.Set("loc", "UTC")
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		u.User.Username(), pass, u.Hostname(), port, strings.TrimPrefix(u.Path, "/"), q.Encode()), nil
}
