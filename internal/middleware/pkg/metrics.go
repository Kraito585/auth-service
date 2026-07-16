package pkgmiddleware

import (
	"strconv"
	"time"

	"auth-service/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

// NewMetricsMiddleware принимает флаг активности извне
func NewMetricsMiddleware(promEnabled bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Если метрики отключены, запрос пролетает без задержек
		if !promEnabled {
			return c.Next()
		}

		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		method := c.Method()

		routePath := "unknown"

		if route := c.Route(); route != nil {
			routePath = route.Path // <-- Убрали скобки ()
		}
		status := strconv.Itoa(c.Response().StatusCode())

		// Пишем в гистограмму
		telemetry.HTTPRequestDuration.WithLabelValues(method, routePath, status).Observe(duration)

		return err
	}
}

func MetricsMiddleware(promEnabled bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Если Прометей выключен, пропускаем логику
		if !promEnabled {
			return c.Next()
		}

		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Response().StatusCode())

		// Безопасное получение шаблона маршрута
		routePath := "unknown"
		if route := c.Route(); route != nil {
			routePath = route.Path // Вернет шаблон, например "/users/:id"
		}

		telemetry.HTTPRequestDuration.WithLabelValues(
			c.Method(),
			routePath,
			statusCode,
		).Observe(duration)

		return err
	}
}
