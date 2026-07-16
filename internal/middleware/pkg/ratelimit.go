package pkgmiddleware

import (
	coreredis "auth-service/core/pkg/redis"
	"auth-service/core/pkg/response"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

// RateLimitMiddleware создает хендлер, который ограничивает количество запросов
func RateLimitMiddleware(rdb *coreredis.Wrapper, name string, max int, window time.Duration, isProd bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()

		if !isProd {
			return c.Next()
		}

		// 1. Определяем, кого мы лимитируем (Идентификатор)
		// Сначала пытаемся достать ID пользователя (если роут защищен)
		identifier := c.IP() // По умолчанию лимитируем по IP (для публичных роутов)

		if userID := c.Locals("user_id"); userID != nil {
			identifier = userID.(string)
		} else if mfaID := c.Locals("mfa_user_id"); mfaID != nil {
			identifier = mfaID.(string)
		}

		// 2. Формируем уникальный ключ для Redis
		// Пример: ratelimit:email_resend:192.168.1.1
		key := fmt.Sprintf("ratelimit:%s:%s", name, identifier)

		// 3. Увеличиваем счетчик
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Если Redis упал, лучше пропустить запрос, чем "положить" весь API
			// Но стоит залогировать ошибку
			return c.Next()
		}

		// 4. Если это первый запрос, ставим время жизни (TTL) ключу
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		// 5. Проверяем, не превысил ли юзер лимит
		if count > int64(max) {
			// Возвращаем красивую ошибку через твой стандартизатор
			return response.Error(
				c,
				fiber.StatusTooManyRequests,
				"Слишком много запросов",
				"Лимит исчерпан. Пожалуйста, подождите.",
			)
		}

		// 6. Всё ок, пускаем дальше
		return c.Next()
	}
}
