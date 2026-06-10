// Package adminapi — HTTP-эндпоинты управления пользователями для Next.js dashboard.
// Заменяет прежнюю Go-шаблонную админ-панель Telegram-бота.
package adminapi

import (
	"net/http"
	"strconv"

	"launcher-backend/internal/auth"
	"launcher-backend/internal/repo"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

type errorResponse struct {
	Message string `json:"message"`
}

func NewHandler(db *gorm.DB) Handler { return Handler{db: db} }

func (h Handler) RegisterRoutes(app *fiber.App, authMiddleware fiber.Handler) {
	g := app.Group("/api/admin")
	g.Use(authMiddleware, auth.RequireAdmin)
	g.Get("/stats", h.stats)
	g.Get("/users", h.listUsers)
	g.Get("/users/:id", h.userDetail)
	g.Patch("/users/:id/role", h.updateRole)
	g.Post("/users/:id/ban", h.ban)
	g.Post("/users/:id/unban", h.unban)
	g.Post("/users/:id/hwid-ban", h.hwidBan)
	g.Post("/users/:id/hwid-unban", h.hwidUnban)
	g.Delete("/users/:id", h.deleteUser)
	g.Get("/auth-logs", h.authLogs)
	g.Get("/audit-logs", h.auditLogs)
}

func atoiQuery(c fiber.Ctx, key string, def int) int {
	n, err := strconv.Atoi(c.Query(key))
	if err != nil || n < 1 {
		return def
	}
	return n
}

func (h Handler) stats(c fiber.Ctx) error {
	s, err := repo.FetchAdminStats(c.Context(), h.db)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Не удалось получить статистику"})
	}
	return c.JSON(s)
}

func (h Handler) listUsers(c fiber.Ctx) error {
	page := atoiQuery(c, "page", 1)
	items, total, err := repo.ListUsersAdmin(c.Context(), h.db, c.Query("q", ""), page)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Не удалось получить список"})
	}
	return c.JSON(fiber.Map{"items": items, "total": total, "page": page, "pageSize": repo.AdminPageSize})
}

func (h Handler) userDetail(c fiber.Ctx) error {
	detail, err := repo.GetUserDetail(c.Context(), h.db, c.Params("id"))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Ошибка"})
	}
	if detail == nil {
		return c.Status(http.StatusNotFound).JSON(errorResponse{Message: "Пользователь не найден"})
	}
	return c.JSON(detail)
}

func (h Handler) updateRole(c fiber.Ctx) error {
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind().Body(&req); err != nil || !repo.ValidRole(req.Role) {
		return c.Status(http.StatusBadRequest).JSON(errorResponse{Message: "Недопустимая роль"})
	}
	id := c.Params("id")
	ok, err := repo.UpdateUserRole(c.Context(), h.db, id, req.Role)
	if err != nil || !ok {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Не удалось обновить роль"})
	}
	h.audit(c, id, "admin_role", req.Role)
	return c.SendStatus(http.StatusNoContent)
}

func (h Handler) ban(c fiber.Ctx) error      { return h.setBan(c, true, false) }
func (h Handler) unban(c fiber.Ctx) error     { return h.setBan(c, false, false) }
func (h Handler) hwidBan(c fiber.Ctx) error   { return h.setBan(c, true, true) }
func (h Handler) hwidUnban(c fiber.Ctx) error { return h.setBan(c, false, true) }

func (h Handler) setBan(c fiber.Ctx, banned, hwid bool) error {
	id := c.Params("id")
	var (
		ok  bool
		err error
	)
	if hwid {
		ok, err = repo.SetHwidBan(c.Context(), h.db, id, banned)
	} else {
		ok, err = repo.SetBan(c.Context(), h.db, id, banned)
	}
	if err != nil || !ok {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Не удалось изменить статус блокировки"})
	}
	action := "admin_ban"
	if !banned {
		action = "admin_unban"
	}
	if hwid {
		action = "hwid_" + action
	}
	h.audit(c, id, action, "")
	return c.SendStatus(http.StatusNoContent)
}

func (h Handler) deleteUser(c fiber.Ctx) error {
	id := c.Params("id")
	if err := repo.DeleteUser(c.Context(), h.db, id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Не удалось удалить"})
	}
	h.audit(c, id, "admin_delete", "")
	return c.SendStatus(http.StatusNoContent)
}

func (h Handler) authLogs(c fiber.Ctx) error {
	items, err := repo.ListAuthLogs(c.Context(), h.db, atoiQuery(c, "page", 1))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Ошибка"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h Handler) auditLogs(c fiber.Ctx) error {
	items, err := repo.ListAuditLogs(c.Context(), h.db, atoiQuery(c, "page", 1))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(errorResponse{Message: "Ошибка"})
	}
	return c.JSON(fiber.Map{"items": items})
}

// audit записывает действие администратора (id из JWT) в журнал.
func (h Handler) audit(c fiber.Ctx, targetID, action, details string) {
	admin, ok := auth.CurrentUser(c)
	if !ok {
		return
	}
	adminID := admin.ID
	target := targetID
	var detailsPtr *string
	if details != "" {
		detailsPtr = &details
	}
	_ = repo.InsertAudit(c.Context(), h.db, nil, &adminID, &target, action, detailsPtr)
}
