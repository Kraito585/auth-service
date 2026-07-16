# Telemetry

The microservice uses three main observability components:

1. **Structured Logging** — `log/slog`
2. **Metrics** — Prometheus
3. **Tracing** — OpenTelemetry + Jaeger

## Logging (`core/pkg/logger/logger.go`)

Uses the standard `log/slog` library with text or JSON format.

```go
logger := logger.NewLogger(cfg.Logger.Level, cfg.Logger.Format)
// where level = "debug"|"info"|"warn"|"error"
// format = "text"|"json"
```

Example log:
```
time=2024-01-15T10:30:00.000+09:00 level=INFO msg="request completed" method=POST path=/api/v1/login status=200 latency=45ms
```

Log levels:
- `DEBUG` — detailed debug information
- `INFO` — standard events (requests, lifecycle)
- `WARN` — warnings (rate limit exceeded, invalid token)
- `ERROR` — errors (DB unavailable, Kafka error)

## Metrics (`internal/telemetry/prometheus.go`)

Prometheus metrics registered via `prometheus/client_golang`:

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP request count |
| `http_request_duration_seconds` | Histogram | Request latency |
| `http_requests_in_flight` | Gauge | In-flight request count |
| `kafka_messages_produced_total` | Counter | Messages published to Kafka |
| `kafka_messages_consumed_total` | Counter | Messages consumed from Kafka |
| `redis_operations_total` | Counter | Redis operations |

Available at `GET /metrics`. Optionally protected by Basic Auth (config `metrics.auth_*`).

## Tracing (`internal/telemetry/jeager.go`)

Configured with OpenTelemetry Jaeger exporter.

```go
func InitTracer(serviceName, jaegerEndpoint, env string) (*trace.TracerProvider, error)
```

Configuration:
```yaml
telemetry:
  tracer: "jaeger"
  jaeger:
    endpoint: "http://jaeger:14268/api/traces"
  environment: "production"
```

All incoming HTTP requests automatically receive a span via the `Tracing` middleware. Span names follow the format `HTTP <METHOD> <PATH>`. Tracing headers are propagated to Redis and PostgreSQL hooks for end-to-end tracing.

## Redis Tracing Hook (`core/pkg/coretelemetry/redis_hook.go`)

Intercepts Redis commands and adds spans:
- Span name: `REDIS <CMD>`
- Attributes: command, keys, duration

## PostgreSQL Tracer (`core/pkg/coretelemetry/pgx_tracer.go`)

Custom pgx tracer for SQL query tracing:
- Span name: `SQL`
- Attributes: SQL text (without parameters), duration, rows affected

## Kafka Tracing Wrapper (`core/pkg/coretelemetry/kafka_wrapper.go`)

Spans for Kafka producer/consumer operations:
- Produce: span with topic and message size attributes
- Consume: span with topic, partition, offset attributes