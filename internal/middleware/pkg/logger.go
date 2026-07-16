package pkgmiddleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger создает middleware для структурированного логирования HTTP-запросов
func NewLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Передаем управление дальше, чтобы запрос обработался
		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		// В Fiber v3 используем c.Context() вместо c.UserContext()
		span := trace.SpanFromContext(c.Context())

		// Собираем базовые параметры лога
		attrs := []any{
			slog.Int("status", status),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("latency", duration.String()),
		}

		// Если есть trace_id, прикрепляем его к логу
		if span.SpanContext().IsValid() {
			attrs = append(attrs, slog.String("trace_id", span.SpanContext().TraceID().String()))
		}

		// Если была ошибка уровня HTTP, пишем Error, иначе Info
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			slog.Error("HTTP Request Failed", attrs...)
		} else {
			slog.Info("HTTP Request", attrs...)
		}

		return err
	}
}
