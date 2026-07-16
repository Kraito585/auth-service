package middleware

import (
	coreredis "auth-service/core/pkg/redis"
	"auth-service/core/pkg/security"
	pkgmiddleware "auth-service/internal/middleware/pkg"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
)

type Manager struct {
	promEnabled   bool
	jaegerEnabled bool
	metricsSecure bool
	metricsUser   string
	metricsPass   string
	jwtManager    *security.JWTManager
	redisClient   *coreredis.Wrapper
	isprod        bool
}

// NewManager принимает зависимости из Строителя (app.go)
func NewManager(promEnabled, jaegerEnabled, metricsSecure bool, mUser, mPass string, jwtManager *security.JWTManager, redisClient *coreredis.Wrapper, isprod bool) *Manager {
	return &Manager{
		promEnabled:   promEnabled,
		jaegerEnabled: jaegerEnabled,
		metricsSecure: metricsSecure,
		metricsUser:   mUser,
		metricsPass:   mPass,
		jwtManager:    jwtManager,
		redisClient:   redisClient,
		isprod:        isprod,
	}
}

// Metrics отдает готовый middleware, прокидывая в него нужную зависимость
func (m *Manager) Metrics() fiber.Handler {
	return pkgmiddleware.MetricsMiddleware(m.promEnabled)
}

func (m *Manager) Tracing() fiber.Handler {
	return pkgmiddleware.TracingMiddleware(m.jaegerEnabled)
}

func (m *Manager) MetricsAuth() fiber.Handler {
	if !m.metricsSecure {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	return basicauth.New(basicauth.Config{
		Users: map[string]string{
			m.metricsUser: m.metricsPass,
		},
	})
}

func (m *Manager) RequireAuth() fiber.Handler {
	return pkgmiddleware.AuthMiddleware(m.jwtManager, false)
}
func (m *Manager) RequireStrictAuth() fiber.Handler {
	return pkgmiddleware.AuthMiddleware(m.jwtManager, true)
}

func (m *Manager) Logging() fiber.Handler {
	return pkgmiddleware.NewLogger()
}

func (m *Manager) RequireMFAToken() fiber.Handler {
	return pkgmiddleware.RequireMFAToken(m.jwtManager)
}

func (m *Manager) RateLimit(name string, max int, window time.Duration) fiber.Handler {
	// Прокидываем спрятанный в Manager клиент Redis дальше в логику
	return pkgmiddleware.RateLimitMiddleware(m.redisClient, name, max, window, m.isprod)
}

func (m *Manager) RequireAPIKey() fiber.Handler {
	return pkgmiddleware.RequireAPIKey(m.redisClient)
}
