package router

import (
	"auth-service/core/pkg/corehandler"
	"auth-service/core/pkg/corerouter"
	"auth-service/core/pkg/health"
	"auth-service/internal/handler"
	"auth-service/internal/middleware"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"
)

// SetupRoutes настраивает все пути приложения
func SetupRoutes(
	app *fiber.App,
	midManager *middleware.Manager,
	coreHandler *corehandler.DefaultHandler,
	defaultHandler *handler.DefaultHandler,
	healthCheckers []health.Checker,
	promEnabled bool,
	authRedirectURL string,
) {
	app.Use(midManager.Tracing())

	corerouter.RegisterSystemRoutes(app, healthCheckers, promEnabled, midManager.MetricsAuth())

	api := app.Group("/api/v1", midManager.Metrics())
	{
		api.Get("/core-status", coreHandler.BaseStatus)

		// Роуты регистрации
		api.Post("/register",
			midManager.RateLimit("register", 3, 10*time.Minute),
			defaultHandler.Register,
		)

		api.Post("/resend/email",
			midManager.RequireAuth(),
			midManager.RateLimit("resend_email_auth", 1, 1*time.Minute),
			defaultHandler.ResendEmail,
		)

		api.Post("/confirm/code",
			midManager.RequireAuth(),
			midManager.RateLimit("confirm_code", 5, 3*time.Minute),
			defaultHandler.ConfirmCode,
		)

		api.Post("/refresh",
			midManager.RateLimit("refresh", 20, 1*time.Minute),
			defaultHandler.Refresh,
		)

		api.Post("/hot/swap/email",
			midManager.RequireAuth(),
			midManager.RateLimit("swap_email", 3, 10*time.Minute),
			defaultHandler.HotSwapEmail,
		)

		// Включение TOTP
		api.Get("/get/totp",
			midManager.RequireStrictAuth(),
			midManager.RateLimit("get_totp", 3, 5*time.Minute),
			defaultHandler.NewTOTP,
		)

		api.Post("/confirm/totp",
			midManager.RequireStrictAuth(),
			midManager.RateLimit("confirm_totp", 5, 5*time.Minute),
			defaultHandler.ConfirmTOTP,
		)

		// Роуты авторизации
		auth := api.Group("/auth/login",
			midManager.RateLimit("auth_login", 10, 1*time.Minute),
			midManager.RequireMFAToken(),
		)

		api.Post("/login",
			midManager.RateLimit("login", 5, 1*time.Minute),
			defaultHandler.Login,
		)

		{
			mfaLimiter := midManager.RateLimit("mfa_attempts", 5, 1*time.Minute)

			auth.Post("/password", mfaLimiter, defaultHandler.LoginAuthMethodPassword)
			auth.Post("/password-email", mfaLimiter, defaultHandler.LoginAuthMethodPasswordEmail)
			auth.Post("/password-totp", mfaLimiter, defaultHandler.LoginAuthMethodPasswordTOTP)
			auth.Post("/totp", mfaLimiter, defaultHandler.LoginAuthMethodTOTP)

			auth.Post("/resend/email",
				midManager.RateLimit("resend_email_mfa", 1, 1*time.Minute),
				defaultHandler.ResendEmail,
			)
		}

		//ODIC endpoints

		app.Get("/oauth/authorize",
			midManager.RateLimit("redirect auth", 6, time.Minute),
			midManager.ValidateRedirectURI(),
			func(c fiber.Ctx) error {
				params := url.Values{}

				params.Set("response_type", "code")
				params.Set("client_id", c.Query("client_id"))
				params.Set("redirect_uri", c.Query("redirect_uri"))
				params.Set("state", c.Query("state"))

				redirectURL := authRedirectURL + "?" + params.Encode()

				return c.Redirect().
					Status(fiber.StatusFound).
					To(redirectURL)
			},
		)

		api.Post("/sso", //Созднание кода авторизации для S2S обмена на сессию
			midManager.RateLimit("mfa_attempts", 6, 1*time.Minute),
			midManager.RequireStrictAuth(),
			defaultHandler.PostSSOToken,
		)

		api.Post("/sso/register/partner",
			midManager.RateLimit("mfa_attempts", 6, 1*time.Minute),
			midManager.RequireStrictAuth(),
			defaultHandler.RegisterPartner,
		)

		s2s := api.Group("/sso/partner",
			midManager.RateLimit("s2s_api", 500, 1*time.Minute),
			midManager.RequireAPIKey(),
		)
		{
			s2s.Post("/exchange", defaultHandler.GetSSOToken)

			s2s.Post("/refresh", defaultHandler.RefreshS2S)
		}
	}
}
