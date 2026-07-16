# Middleware

The middleware manager (`internal/middleware/manager.go`) centrally manages all middleware handlers.

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

The manager is assembled in `app.go` → `Build()` via `NewMiddlewareManager()`.

## Implementations

| Middleware | File | Purpose |
|-----------|------|---------|
| Logger | `middleware/pkg/logger.go` | Request logging (method, path, status, latency) |
| Metrics | `middleware/pkg/metrics.go` | HTTP metrics collection (request counters, latency histograms) |
| Tracing | `middleware/pkg/tracing.go` | Per-request span creation (OpenTelemetry) |
| RateLimit | `middleware/pkg/ratelimit.go` | Key-based rate limiting (Redis-backed) |
| RequireAuth | `middleware/pkg/auth.go` | JWT validation, extracts `user_id` and `email_verified` |
| RequireStrictAuth | `middleware/pkg/AuthMiddleware.go` | Like RequireAuth, but requires verified email |
| RequireAPIKey | `middleware/pkg/RequireAPIKey.go` | Validates client API key from `X-Client-Secret` header |
| RequireMFAToken | `middleware/pkg/auth.go` | Validates `auth_session_token` cookie for MFA endpoints |

## RequireAPIKey

Checks the `X-Client-Secret` header, looks up the key in Redis (`HGET clients:secrets <key>`), stores `client_id` in `c.Locals`. Returns `401` on missing/invalid key.

## RequireAuth / RequireStrictAuth

Extracts `Authorization: Bearer <jwt>`, parses the token via `jwtManager.ParseAndValidate(token, "sub")`, stores `user_id` and `email_verified` in `c.Locals`. `RequireStrictAuth` additionally blocks requests with `email_verified == false`.

## RequireMFAToken

Validates the `auth_session_token` cookie (set by the `/login` endpoint). Decrypts session data and stores it in `c.Locals`.

## RateLimit

Config-driven limits per endpoint (requests / duration). Uses Redis for counter storage; the key is formed from the path + IP (or user ID).