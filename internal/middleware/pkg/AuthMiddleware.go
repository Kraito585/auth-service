package pkgmiddleware

import (
	"auth-service/core/pkg/security"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// RequireMFAToken пускает только пользователей с временным токеном второго этапа
func RequireMFAToken(jwtManager *security.JWTManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Отсутствует токен сессии авторизации",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Неверный формат заголовка Authorization",
			})
		}

		claims, err := jwtManager.ParseAndValidate(parts[1], "sub", "type")
		if err != nil {
			slog.Warn("MFA Auth Error", slog.Any("error", err))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "Токен сессии недействителен или истек",
				"details": err.Error(),
			})
		}

		if claims["type"].(string) != "mfa_session" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Неверный тип токена: ожидался mfa_session",
			})
		}

		c.Locals("user_id", claims["sub"].(string))

		return c.Next()
	}
}
