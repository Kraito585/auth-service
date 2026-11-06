package handler

import (
	"auth-service/core/pkg/response"
	"auth-service/internal/model"
	"auth-service/internal/service"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
)

// _____/\\\\\\\\\\\______/\\\\\\\\\_________/\\\\\\\\\\\___
//  ___/\\\/////////\\\__/\\\///////\\\_____/\\\/////////\\\_
//   __\//\\\______\///__\///______\//\\\___\//\\\______\///__
//    ___\////\\\___________________/\\\/_____\////\\\_________
//     ______\////\\\_____________/\\\//__________\////\\\______
//      _________\////\\\_______/\\\//________________\////\\\___
//       __/\\\______\//\\\____/\\\/____________/\\\______\//\\\__
//        _\///\\\\\\\\\\\/____/\\\\\\\\\\\\\\\_\///\\\\\\\\\\\/___
//         ___\///////////_____\///////////////____\///////////_____

type OidcHandler struct {
	srv     *service.OidcService
	is_prod bool
}

func NewOidcHandler(srv *service.OidcService, is_prod bool) *OidcHandler {
	return &OidcHandler{srv: srv}
}

var OidcHandlerTracer = otel.Tracer("http-handler")

// Создание ODIC токена для S2S обмена на сессию
func (h *OidcHandler) PostOIDCToken(c fiber.Ctx) error {
	ctx, span := OidcHandlerTracer.Start(c.Context(), "handler.PostOIDCToken")
	defer span.End()

	var req model.RequestNewOIDCCode

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	UUID := c.Locals("user_id").(string)

	code, err := h.srv.PostOIDCToken(ctx, req.ClientID, UUID, req.Nonce)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка сохранения токена", err.Error())
	}

	return response.OK(c, fiber.Map{
		"code": code,
	})
}

// Обмен OIDC токена на jwt сессиию
func (h *OidcHandler) GetOIDCSession(c fiber.Ctx) error {
	ctx, span := OidcHandlerTracer.Start(c.Context(), "handler.GetOIDCSession")
	defer span.End()

	var req model.GetOIDCSession

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	res, err := h.srv.GetOIDCSession(ctx, req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка сохранения токена", err.Error())
	}

	return c.JSON(res)
}

func (h *DefaultHandler) RefreshS2S(c fiber.Ctx) error {
	ctx, span := OidcHandlerTracer.Start(c.Context(), "handler.RefreshS2S")
	defer span.End()

	cliientID := c.Locals("client_id").(string)

	var req model.RefreshS2S

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	res, err := h.srv.Refresh(ctx, req.RefreshKey, req.ClientIP, req.UserAgent, cliientID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "ошибка перевыпуска токена", err.Error())
	}

	return c.JSON(res)
}

// Регистрация API ключа
func (h *OidcHandler) RegisterPartner(c fiber.Ctx) error {
	ctx, span := OidcHandlerTracer.Start(c.Context(), "handler.RegisterPartner")
	defer span.End()
	var req model.RegisterPartner

	if err := c.Bind().JSON(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Неверный формат запроса", err.Error())
	}

	UUID := c.Locals("user_id").(string)

	api, apiKey, err := h.srv.RegisterPartner(ctx, UUID, req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Ошибка регистрации api ключа", err.Error())
	}

	return response.OK(c, fiber.Map{
		"api":    api,
		"apiKey": apiKey,
	})
}
