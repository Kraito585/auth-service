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
	oidcHandler *handler.OidcHandler,
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

			api.Post("/create/oidc/code", //Созднание кода авторизации для S2S обмена на сессию
				midManager.RateLimit("mfa_attempts", 6, 1*time.Minute),
				midManager.RequireStrictAuth(),
				oidcHandler.PostOIDCToken,
			)
		}

		//OIDC endpoints
		app.Get("/.well-known/openid-configuration", //TODO: Заменить данные на реальные значения
			midManager.RateLimit("openid_config", 500, time.Minute),
			func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{
					"issuer": "https://auth.example.com",

					"authorization_endpoint": "https://auth.example.com/oauth/authorize",
					"token_endpoint":         "https://auth.example.com/oauth/token",
					"userinfo_endpoint":      "https://auth.example.com/oauth/userinfo",

					"jwks_uri": "https://auth.example.com/.well-known/jwks.json",

					"response_types_supported": []string{
						"code",
					},

					"grant_types_supported": []string{
						"authorization_code",
						"refresh_token",
					},

					"subject_types_supported": []string{
						"public",
					},

					"id_token_signing_alg_values_supported": []string{
						"RS256",
					},

					"scopes_supported": []string{
						"openid",
						"profile",
						"email",
					},

					"code_challenge_methods_supported": []string{
						"S256",
					},
				})
			})

		app.Get("/oauth/authorize", //Переадресация на страницу авторизации с проверкой redirect_uri
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

		// TODO: Изменить handler узнать что ещё должно вернутся в теле ответа кроме токенов и их сроков жизни
		// большая часть описна в modle/oidc.go:TokenResponse
		app.Post("/oauth/token", //Обмен кода авторизации на сессию
			midManager.RateLimit("s2s_api", 500, 1*time.Minute),
			midManager.RequireAPIKey(),
			oidcHandler.GetOIDCSession,
		)

		app.Post("/oauth/refresh", //Обновление сессии и получение AccessToken и RefreshToken
			midManager.RateLimit("s2s_api", 500, 1*time.Minute),
			midManager.RequireAPIKey(),
			defaultHandler.RefreshS2S,
		)

		//TODO: Новый функциионал: GET /.well-known/jwks.json
		//TODO: Новый функциионал: GET /oauth/userinfo
		//TODO: Новый функциионал: POST /oauth/revoke
		//TODO: Новый функциионал: POST /oauth/introspect

		api.Post("/oidc/register/partner",
			midManager.RateLimit("mfa_attempts", 6, 1*time.Minute),
			midManager.RequireStrictAuth(),
			oidcHandler.RegisterPartner,
		)

	}
}
