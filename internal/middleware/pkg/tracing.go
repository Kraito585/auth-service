package pkgmiddleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware принимает флаг активности из Менеджера
func TracingMiddleware(enabled bool) fiber.Handler {
	tracer := otel.Tracer("fiber-http")

	return func(c fiber.Ctx) error {
		if !enabled {
			return c.Next()
		}

		start := time.Now()

		savedCtx := c.Context()

		headers := c.GetReqHeaders()
		extractedCtx := otel.GetTextMapPropagator().Extract(savedCtx, propagation.HeaderCarrier(headers))

		spanName := "unknown_route"
		if route := c.Route(); route != nil {
			spanName = route.Path
		} else {
			spanName = c.Path()
		}

		ctx, span := tracer.Start(
			extractedCtx,
			spanName,
			trace.WithTimestamp(start),
			trace.WithSpanKind(trace.SpanKindServer),
		)

		c.SetContext(ctx)
		defer c.SetContext(savedCtx)

		span.SetAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.target", c.Path()),
		)

		traceID := span.SpanContext().TraceID().String()
		c.Set("X-Trace-Id", traceID)

		err := c.Next()

		span.SetAttributes(attribute.Int("http.status_code", c.Response().StatusCode()))
		if err != nil {
			span.RecordError(err)
		}

		span.End(trace.WithTimestamp(time.Now()))

		return err
	}
}
