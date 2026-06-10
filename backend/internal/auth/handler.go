package auth

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	service Service
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Totp     string `json:"totp"`
}

type ErrorResponse struct {
	Message           string `json:"message"`
	RequiresTwoFactor bool   `json:"requiresTwoFactor,omitempty"`
}

func NewHandler(service Service) Handler {
	return Handler{service: service}
}

func (h Handler) RegisterRoutes(app *fiber.App) {
	group := app.Group("/api/auth")
	group.Post("/login", h.login)
	group.Post("/register", h.registerUnavailable)
	group.Get("/me", h.service.RequireAuth(), h.me)

	// GML custom authorization: лаунчер (и authlib) ходят сюда логином/паролем/2FA.
	// Делит общий код с /api/auth/login, но отвечает в формате GML (UserUuid/Login/IsSlim).
	app.Post("/api/gml/auth", h.gmlAuth)
}

type gmlAuthRequest struct {
	Login      string `json:"Login"`
	Password   string `json:"Password"`
	Totp       string `json:"Totp"`
	HardwareID string `json:"HardwareId"`
}

type gmlAuthResponse struct {
	Message  string `json:"Message"`
	UserUuid string `json:"UserUuid,omitempty"`
	Login    string `json:"Login,omitempty"`
	IsSlim   bool   `json:"IsSlim"`
}

func (h Handler) gmlAuth(c fiber.Ctx) error {
	var req gmlAuthRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(gmlAuthResponse{Message: "Некорректный JSON"})
	}
	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		return c.Status(http.StatusUnauthorized).JSON(gmlAuthResponse{Message: "Логин и пароль обязательны"})
	}
	result, err := h.service.Login(c.Context(), req.Login, req.Password, req.Totp)
	if err != nil {
		if providerErr, ok := AsProviderError(err); ok {
			status := providerErr.StatusCode
			if status == 0 {
				status = http.StatusUnauthorized
			}
			return c.Status(status).JSON(gmlAuthResponse{Message: providerErr.Message})
		}
		return c.Status(http.StatusBadGateway).JSON(gmlAuthResponse{Message: "Сервис авторизации недоступен"})
	}
	return c.JSON(gmlAuthResponse{
		Message:  "Успешная авторизация",
		UserUuid: result.User.ProviderUUID,
		Login:    result.User.Login,
		IsSlim:   result.User.IsSlim,
	})
}

func (h Handler) login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: "Некорректный JSON"})
	}

	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{Message: "Введите логин и пароль"})
	}

	result, err := h.service.Login(c.Context(), req.Login, req.Password, req.Totp)
	if err == nil {
		return c.JSON(result)
	}

	if providerErr, ok := AsProviderError(err); ok {
		status := providerErr.StatusCode
		if status == 0 {
			status = http.StatusUnauthorized
		}
		return c.Status(status).JSON(ErrorResponse{
			Message:           providerErr.Message,
			RequiresTwoFactor: providerErr.RequiresTwoFactor,
		})
	}

	return c.Status(http.StatusBadGateway).JSON(ErrorResponse{Message: "Сервис авторизации недоступен"})
}

func (h Handler) registerUnavailable(c fiber.Ctx) error {
	return c.Status(http.StatusNotImplemented).JSON(ErrorResponse{
		Message: "Регистрация отключена: используется внешняя GML custom authorization.",
	})
}

func (h Handler) me(c fiber.Ctx) error {
	user, ok := CurrentUser(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(ErrorResponse{Message: "Требуется авторизация"})
	}
	return c.JSON(user)
}
