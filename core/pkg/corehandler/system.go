package corehandler

import (
	"github.com/gofiber/fiber/v3"
)

// DefaultServiceCtx описывает контракт для бизнес-сервиса (чтобы хендлер не зависел от internal)
type DefaultServiceCtx interface {
	// Опиши здесь методы, если они нужны хендлеру, например:
	// GetStatus(ctx context.Context) (string, error)
}

type DefaultHandler struct {
	service DefaultServiceCtx
}

func NewDefaultHandler(service DefaultServiceCtx) *DefaultHandler {
	return &DefaultHandler{
		service: service,
	}
}

// Пример базового метода
func (h *DefaultHandler) BaseStatus(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"core_version": "v1.0.0",
		"status":       "operational",
	})
}
