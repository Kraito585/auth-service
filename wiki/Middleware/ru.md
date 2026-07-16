# Middleware

Менеджер middleware (`internal/middleware/manager.go`) централизованно управляет всеми промежуточными обработчиками.

## Manager

```go
type Manager struct {
    Logger     fiber.Handler
    Metrics    fiber.Handler
    Tracing    fiber.Handler
    RateLimit  fiber.Handler
    RequireAuth fiber.Handler
    RequireStrictAuth fiber.Handler
    RequireAPIKey fiber.Handler
    RequireMFAToken fiber.Handler
}
```

Менеджер собирается в `app.go` → `Build()` через `NewMiddlewareManager()`.

## Реализации

| Middleware | Файл | Назначение |
|-----------|------|------------|
| Logger | `middleware/pkg/logger.go` | Логирование каждого запроса (метод, путь, статус, latency) |
| Metrics | `middleware/pkg/metrics.go` | Сбор метрик HTTP (счётчики запросов, гистограммы задержек) |
| Tracing | `middleware/pkg/tracing.go` | Создание span'ов для каждого запроса (OpenTelemetry) |
| RateLimit | `middleware/pkg/ratelimit.go` | Ограничение частоты запросов по ключу (Redis-based) |
| RequireAuth | `middleware/pkg/auth.go` | Проверка JWT токена, извлечение `user_id` и `email_verified` |
| RequireStrictAuth | `middleware/pkg/AuthMiddleware.go` | Как RequireAuth, но с проверкой подтверждённого email |
| RequireAPIKey | `middleware/pkg/RequireAPIKey.go` | Проверка API-ключа клиента в заголовке `X-Client-Secret` |
| RequireMFAToken | `middleware/pkg/auth.go` | Проверка куки `auth_session_token` для MFA эндпоинтов |

## RequireAPIKey

Проверяет заголовок `X-Client-Secret`, ищет ключ в Redis (`HGET clients:secrets <key>`), сохраняет `client_id` в `c.Locals`. Возвращает `401` при отсутствии/неверном ключе.

## RequireAuth / RequireStrictAuth

Извлекает `Authorization: Bearer <jwt>`, парсит токен через `jwtManager.ParseAndValidate(token, "sub")`, сохраняет `user_id` и `email_verified` в `c.Locals`. `RequireStrictAuth` дополнительно блокирует запросы с `email_verified == false`.

## RequireMFAToken

Проверяет куку `auth_session_token` (устанавливается эндпоинтом `/login`). Расшифровывает данные сессии и сохраняет в `c.Locals`.

## RateLimit

На основе конфигурации с ограничениями по эндпоинтам (requests / duration). Использует Redis для хранения счётчиков, ключ формируется из пути + IP (или ID пользователя).