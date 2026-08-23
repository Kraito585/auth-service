package handler

import (
	"auth-service/core/pkg/response"
	"auth-service/internal/model"
	"auth-service/internal/service"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
)

type DefaultHandler struct {
	srv     *service.DefaultService
	is_prod bool
}

func NewDefaultHandler(srv *service.DefaultService, is_prod bool) *DefaultHandler {
	return &DefaultHandler{srv: srv}
}

var handlerTracer = otel.Tracer("http-handler")

func (h *DefaultHandler) Register(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.Register")
	defer span.End()

	var req model.RegisterRequest

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	res, refreshToken, err := h.srv.Register(ctx, &req, clientIP, userAgent)
	if err != nil {
		return err
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": res.AccessToken,
	})
}

func (h *DefaultHandler) ResendEmail(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.ResendEmail")
	defer span.End()

	UUID := c.Locals("user_id").(string)

	err := h.srv.ResendEmail(ctx, UUID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка перевыпуска кода подтверждения", err.Error())
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Новый код подтверждения отправлен на почту",
	})
}

func (h *DefaultHandler) Refresh(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.Refresh")
	defer span.End()

	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Отсутствует токен обновления. Авторизуйтесь заново.",
		})
	}

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	newAccess, newRefresh, err := h.srv.Refresh(ctx, refreshToken, clientIP, userAgent)
	if err != nil {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour),
			HTTPOnly: true,
		})
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Сессия истекла или отозвана: " + err.Error(),
		})
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    newRefresh,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    newRefresh,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}
	return response.OK(c, fiber.Map{
		"access_token": newAccess,
	})
}

func (h *DefaultHandler) HotSwapEmail(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.HotSwapEmail")
	defer span.End()

	var req model.HotSwapEmail
	UUID := c.Locals("user_id").(string)

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	err := h.srv.HotSwapEmail(ctx, UUID, req.Email)
	if err != nil {
		return fmt.Errorf("ошибка смены email: %w", err)
	}

	return response.OK(c, "")
}

func (h *DefaultHandler) ConfirmCode(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.ConfirmCode")
	defer span.End()

	var req model.ConfirmCode
	UUID := c.Locals("user_id").(string)

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	refreshToken, publicToken, err := h.srv.ConfirmCode(ctx, req.Code, UUID, clientIP, userAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка подтверждения email", err.Error())
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": publicToken,
	})
}

func (h *DefaultHandler) NewTOTP(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.NewTOTP")
	defer span.End()

	UUID := c.Locals("user_id").(string)

	totpURL, err := h.srv.NewTOTP(ctx, UUID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка генерации totp", err.Error())
	}

	return response.OK(c, fiber.Map{
		"totp_url": totpURL,
	})
}

func (h *DefaultHandler) ConfirmTOTP(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.ConfirmTOTP")
	defer span.End()

	var req model.ConfirmCode
	UUID := c.Locals("user_id").(string)

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	codes, err := h.srv.ConfirmTOTP(ctx, UUID, req.Code)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка подтверждения кода", err.Error())
	}

	return response.OK(c, fiber.Map{
		"codes": codes,
	})
}

func (h *DefaultHandler) Login(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.Login")
	defer span.End()

	var req model.Login

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	token, authType, err := h.srv.Login(ctx, req.Login)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка входа в аккаунт", err.Error())
	}

	return response.OK(c, fiber.Map{
		"authType": authType,
		"code":     token,
	})
}

func (h *DefaultHandler) LoginAuthMethodPassword(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.LoginAuthMethodPassword")
	defer span.End()

	UUID := c.Locals("user_id").(string)
	var req model.LoginAuthMethodPassword

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}
	refreshToken, publicToken, err := h.srv.LoginAuthMethodPassword(ctx, UUID, req.Password, clientIP, userAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка входа в аккаунт", err.Error())
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": publicToken,
	})
}

func (h *DefaultHandler) LoginAuthMethodPasswordEmail(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.LoginAuthMethodPasswordEmail")
	defer span.End()

	UUID := c.Locals("user_id").(string)
	var req model.LoginAuthMethodPasswordEmail

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}
	refreshToken, publicToken, err := h.srv.LoginAuthMethodPasswordEmail(ctx, UUID, req.Password, req.Code, clientIP, userAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка входа в аккаунт", err.Error())
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": publicToken,
	})
}

func (h *DefaultHandler) LoginAuthMethodPasswordTOTP(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.LoginAuthMethodPasswordTOTP")
	defer span.End()

	UUID := c.Locals("user_id").(string)
	var req model.LoginAuthMethodPasswordTOTP

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}
	refreshToken, publicToken, err := h.srv.LoginAuthMethodPasswordTOTP(ctx, UUID, req.Password, req.Code, clientIP, userAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка входа в аккаунт", err.Error())
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": publicToken,
	})
}

func (h *DefaultHandler) LoginAuthMethodTOTP(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.LoginAuthMethodTOTP")
	defer span.End()

	UUID := c.Locals("user_id").(string)
	var req model.LoginAuthMethodTOTP

	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	refreshToken, publicToken, err := h.srv.LoginAuthMethodTOTP(ctx, UUID, req.Code, clientIP, userAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка входа в аккаунт", err.Error())
	}

	if !h.is_prod {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   false,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	} else {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Strict",
			Path:     "/api/v1/refresh",
		})
	}

	return response.OK(c, fiber.Map{
		"jwt": publicToken,
	})
}

func (h *DefaultHandler) PostSSOToken(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.PostSSOToken")
	defer span.End()

	UUID := c.Locals("user_id").(string)

	token, err := h.srv.PostSSOToken(ctx, UUID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка сохранения токена", err.Error())
	}

	return response.OK(c, fiber.Map{
		"code": token,
	})
}

func (h *DefaultHandler) GetSSOToken(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.GetSSOToken")
	defer span.End()

	var req model.SSOTokenS2S

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	refreshToken, publicToken, err := h.srv.GetSSOToken(ctx, req.Token, req.ClientIP, req.UserAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка сохранения токена", err.Error())
	}

	return response.OK(c, fiber.Map{
		"PublicToken":  publicToken,
		"RefreshToken": refreshToken,
	})
}

func (h *DefaultHandler) RefreshS2S(c fiber.Ctx) error {
	ctx, span := handlerTracer.Start(c.Context(), "handler.RefreshS2S")
	defer span.End()

	var req model.RefreshS2S

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	refreshToken, publicToken, err := h.srv.Refresh(ctx, req.RefreshKey, req.ClientIP, req.UserAgent)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка перевыпуска токена", err.Error())
	}

	return response.OK(c, fiber.Map{
		"PublicToken":  publicToken,
		"RefreshToken": refreshToken,
	})
}
