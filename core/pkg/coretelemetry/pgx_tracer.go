package coretelemetry

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type ctxKey string

const pgxStartKey ctxKey = "pgx_start"

type PgxMetricsTracer struct{}

func NewPgxMetricsTracer() *PgxMetricsTracer {
	return &PgxMetricsTracer{}
}

func (t *PgxMetricsTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, pgxStartKey, time.Now())
}

func (t *PgxMetricsTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	startTime, ok := ctx.Value(pgxStartKey).(time.Time)
	if !ok {
		return
	}

	DependencyDuration.WithLabelValues("postgresql", "query").Observe(time.Since(startTime).Seconds())
}
