package pkgmiddleware

import (
	coreredis "auth-service/core/pkg/redis"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

func ValidateRedirectURI(rdb *coreredis.Wrapper) fiber.Handler {
	return func(c fiber.Ctx) error {
		responseType := c.Query("response_type")
		if responseType != "code" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Неверный response_type",
			})
		}

		clientID := c.Query("client_id")
		if clientID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Отсутствует client_id",
			})
		}

		redirectURI := c.Query("redirect_uri")
		if redirectURI == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Отсутствует redirect_uri",
			})
		}

		redisKey := fmt.Sprintf("clientID:%s", clientID)

		originalRedirectURI, err := rdb.Get(
			c.Context(),
			redisKey,
		).Result()

		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Неверный client_id",
			})
		}

		if originalRedirectURI != redirectURI {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Неверный redirect_uri",
			})
		}

		return c.Next()
	}
}
