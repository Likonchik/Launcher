package news

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	service *Service
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func NewHandler(service *Service) Handler {
	return Handler{service: service}
}

func (h Handler) RegisterRoutes(app *fiber.App, authMiddleware fiber.Handler) {
	group := app.Group("/api/news")
	group.Use(authMiddleware)
	group.Get("/", h.list)
}

func (h Handler) list(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", ""))
	items, err := h.service.List(c.Context(), limit)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{Message: "Не удалось получить новости"})
	}
	return c.JSON(items)
}
