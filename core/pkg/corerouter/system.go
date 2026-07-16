package corerouter

import (
	"auth-service/core/pkg/health"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// RegisterSystemRoutes регистрирует общие для всех сервисов эндпоинты мониторинга
func RegisterSystemRoutes(
	app *fiber.App,
	checkers []health.Checker,
	promEnabled bool,
	metricsAuth fiber.Handler, // Передаем мидлвару как абстрактный хендлер
) {
	// Liveness probe
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Readiness probe
	app.Get("/readyz", func(c fiber.Ctx) error {
		var failures []string

		for _, checker := range checkers {
			if !checker.IsHealthy() {
				failures = append(failures, checker.Name()+" is unavailable")
			}
		}

		if len(failures) > 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  "error",
				"reasons": failures,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	if promEnabled {
		promHandler := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())

		// Используем переданную мидлвару авторизации
		app.Get("/metrics", metricsAuth, func(c fiber.Ctx) error {
			promHandler(c.RequestCtx())
			return nil
		})
	}
}
