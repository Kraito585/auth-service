package coretelemetry

import (
	"context"
	"net"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type RedisHook struct{}

func NewRedisHook() *RedisHook {
	return &RedisHook{}
}

func (h *RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		tracer := otel.Tracer("redis")
		ctx, span := tracer.Start(ctx, cmd.Name(), trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()

		span.SetAttributes(attribute.String("db.system", "redis"))
		span.SetAttributes(attribute.String("db.statement", cmd.String()))

		err := next(ctx, cmd)
		if err != nil {
			span.RecordError(err)
		}
		return err
	}
}

func (h *RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}
