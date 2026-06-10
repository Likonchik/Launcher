package middleware

import "github.com/gofiber/fiber/v3"

func CORS(allowedOrigins []string) fiber.Handler {
	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
			c.Set("Access-Control-Allow-Origin", origin)
		}
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}
	return false
}
