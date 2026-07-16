package pkgmiddleware

import (
	coreredis "auth-service/core/pkg/redis"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

func RequireAPIKey(rdb *coreredis.Wrapper) fiber.Handler {
	return func(c fiber.Ctx) error {
		// 1. Достаем ключ (например, из специального заголовка)
		apiKey := c.Get("X-Client-Secret")
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Отсутствует API ключ"})
		}

		// 2. Ищем в Redis
		// HGET clients:secrets <apiKey>
		clientID, err := rdb.HGet(c.Context(), "clients:secrets", apiKey).Result()
		if err == redis.Nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Неверный API ключ"})
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ошибка проверки ключа"})
		}

		// 3. Сохраняем ID партнера в контекст (чтобы знать, кому мы выдаем сессию)
		c.Locals("client_id", clientID)

		return c.Next()
	}
}
