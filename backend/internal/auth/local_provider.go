package auth

import (
	"context"
	"net/http"
	"strings"

	"launcher-backend/internal/models"
	"launcher-backend/internal/repo"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LocalProvider проверяет логин/пароль/2FA прямо в общей БД (bcrypt + TOTP).
// Логика повторяет прежний GML-эндпоинт Telegram-бота: поиск по нику/логину/почте,
// проверка блокировок, bcrypt, при включённой 2FA — TOTP-код.
type LocalProvider struct {
	db *gorm.DB
}

func NewLocalProvider(db *gorm.DB) LocalProvider {
	return LocalProvider{db: db}
}

func (p LocalProvider) SignIn(ctx context.Context, login, password, totpCode string) (ProviderSignInResponse, error) {
	login = strings.TrimSpace(login)
	badCreds := ProviderError{StatusCode: http.StatusUnauthorized, Message: "Неверный логин или пароль"}

	user, err := repo.FindUserLogin(ctx, p.db, login)
	if err != nil {
		return ProviderSignInResponse{}, err
	}
	if user == nil {
		_ = repo.InsertAuthLog(ctx, p.db, nil, login, "launcher", false, strptr("not_found"))
		return ProviderSignInResponse{}, badCreds
	}

	uid := user.ID
	if user.IsBanned || user.IsHwidBanned {
		_ = repo.InsertAuthLog(ctx, p.db, &uid, user.Login, "launcher", false, strptr("banned"))
		return ProviderSignInResponse{}, ProviderError{StatusCode: http.StatusForbidden, Message: "Аккаунт заблокирован"}
	}

	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		_ = repo.InsertAuthLog(ctx, p.db, &uid, user.Login, "launcher", false, strptr("bad_password"))
		return ProviderSignInResponse{}, badCreds
	}

	if user.TOTPEnabled && user.TOTPSecret != "" {
		code := strings.ReplaceAll(strings.TrimSpace(totpCode), " ", "")
		if code == "" {
			_ = repo.InsertAuthLog(ctx, p.db, &uid, user.Login, "launcher", false, strptr("totp_required"))
			return ProviderSignInResponse{}, ProviderError{
				StatusCode: http.StatusUnauthorized, Message: twoFactorMessage, RequiresTwoFactor: true,
			}
		}
		if !totp.Validate(code, user.TOTPSecret) {
			_ = repo.InsertAuthLog(ctx, p.db, &uid, user.Login, "launcher", false, strptr("invalid_totp"))
			return ProviderSignInResponse{}, ProviderError{
				StatusCode: http.StatusUnauthorized, Message: "Неверный код двухфакторной аутентификации", RequiresTwoFactor: true,
			}
		}
	}

	_ = repo.InsertAuthLog(ctx, p.db, &uid, user.Login, "launcher", true, strptr("OK"))
	return ProviderSignInResponse{
		Login:    user.Login,
		UserUUID: user.ProviderUUID,
		IsSlim:   user.IsSlim,
		Message:  "Успешная авторизация",
	}, nil
}

// MarkLogin фиксирует ip/hwid после успешного входа (для GML-эндпоинта лаунчера).
func (p LocalProvider) MarkLogin(ctx context.Context, providerUUID, ip, hardwareID string) {
	var u models.User
	if err := p.db.WithContext(ctx).Where("provider_uuid = ?", providerUUID).First(&u).Error; err != nil {
		return
	}
	_ = repo.UpdateUserAfterGMLLogin(ctx, p.db, u.ID, ip, hardwareID)
}

func strptr(s string) *string { return &s }
