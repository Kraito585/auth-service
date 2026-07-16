package pkgmiddleware

import (
	"auth-service/core/pkg/security"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// AuthMiddleware проверяет JWT токен и кладет user_id в контекст
func AuthMiddleware(jwtManager *security.JWTManager, requireVerifiedEmail bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Отсутствует токен авторизации",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Неверный формат заголовка Authorization",
			})
		}

		claims, err := jwtManager.ParseAndValidate(parts[1], "sub")
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Невалидный токен"})
		}

		// 2. Проверяем статус почты
		emailVerified := true
		if emailClaim, exists := claims["email"].(bool); exists {
			emailVerified = emailClaim
		}

		// 3. БЛОКИРУЕМ, если роут требует почту, а её нет
		if requireVerifiedEmail && !emailVerified {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Для этого действия необходимо подтвердить email",
				"code":  "EMAIL_UNVERIFIED",
			})
		}

		// 4. Сохраняем данные и пускаем дальше
		c.Locals("user_id", claims["sub"].(string))
		c.Locals("email_verified", emailVerified)

		return c.Next()
	}
}
