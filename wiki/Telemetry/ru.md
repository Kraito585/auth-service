# Телеметрия

Микросервис использует три основных компонента наблюдаемости:

1. **Структурированное логирование** — `log/slog`
2. **Метрики** — Prometheus
3. **Трейсинг** — OpenTelemetry + Jaeger

## Логирование (`core/pkg/logger/logger.go`)

Использует стандартную библиотеку `log/slog` с текстовым или JSON форматом.

```go
logger := logger.NewLogger(cfg.Logger.Level, cfg.Logger.Format)
// где level = "debug"|"info"|"warn"|"error"
// format = "text"|"json"
```

Пример лога:
```
time=2024-01-15T10:30:00.000+09:00 level=INFO msg="request completed" method=POST path=/api/v1/login status=200 latency=45ms
```

Уровни логирования:
- `DEBUG` — детальная отладочная информация
- `INFO` — стандартные события (запросы, lifecycle)
- `WARN` — предупреждения (rate limit превышен, неверный токен)
- `ERROR` — ошибки (БД недоступна, Kafka ошибка)

## Метрики (`internal/telemetry/prometheus.go`)

Prometheus-метрики, регистрируемые через `prometheus/client_golang`:

| Метрика | Тип | Описание |
|---------|-----|----------|
| `http_requests_total` | Counter | Общее количество HTTP запросов |
| `http_request_duration_seconds` | Histogram | Задержка запросов |
| `http_requests_in_flight` | Gauge | Количество активных запросов |
| `kafka_messages_produced_total` | Counter | Отправлено сообщений в Kafka |
| `kafka_messages_consumed_total` | Counter | Прочитано сообщений из Kafka |
| `redis_operations_total` | Counter | Операций с Redis |

Доступны на эндпоинте `GET /metrics`. Опционально защищены Basic Auth (конфиг `metrics.auth_*`).

## Трейсинг (`internal/telemetry/jeager.go`)

Настроен OpenTelemetry exporter в Jaeger.

```go
func InitTracer(serviceName, jaegerEndpoint, env string) (*trace.TracerProvider, error)
```

Конфигурация:
```yaml
telemetry:
  tracer: "jaeger"
  jaeger:
    endpoint: "http://jaeger:14268/api/traces"
  environment: "production"
```

Все входящие HTTP запросы автоматически получают span через middleware `Tracing`. Имена span'ов формируются как `HTTP <METHOD> <PATH>`. Заголовки трейсинга пробрасываются в Redis и PostgreSQL хуки для end-to-end трассировки.

## Redis Tracing Hook (`core/pkg/coretelemetry/redis_hook.go`)

Перехватывает Redis-команды и добавляет span'ы:
- Span name: `REDIS <CMD>`
- Атрибуты: команда, ключи, длительность

## PostgreSQL Tracer (`core/pkg/coretelemetry/pgx_tracer.go`)

Кастомный pgx трасер для трейсинга SQL запросов:
- Span name: `SQL`
- Атрибуты: SQL текст (без параметров), длительность, количество затронутых строк

## Kafka Tracing Wrapper (`core/pkg/coretelemetry/kafka_wrapper.go`)

Span'ы для Kafka producer/consumer операций:
- Produce: span с атрибутами топика и размера сообщения
- Consume: span с атрибутами топика, партиции, offset'а