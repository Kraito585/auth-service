package response

import (
	"github.com/gofiber/fiber/v3"
)

// APIResponse — единая структура для всех ответов сервера
type APIResponse struct {
	Success bool          `json:"success"`
	Data    interface{}   `json:"data,omitempty"`
	Error   *ErrorDetails `json:"error,omitempty"`
}

// ErrorDetails — структура для детального описания ошибки
type ErrorDetails struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// УБРАЛИ ЗВЕЗДОЧКУ: c fiber.Ctx
func OK(c fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// УБРАЛИ ЗВЕЗДОЧКУ: c fiber.Ctx
func Created(c fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// УБРАЛИ ЗВЕЗДОЧКУ: c fiber.Ctx
func NoContent(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// УБРАЛИ ЗВЕЗДОЧКУ: c fiber.Ctx
func Error(c fiber.Ctx, status int, message string, details interface{}) error {
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error: &ErrorDetails{
			Code:    status,
			Message: message,
			Details: details,
		},
	})
}

// УБРАЛИ ЗВЕЗДОЧКУ: c fiber.Ctx
func GlobalErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Внутренняя ошибка сервера"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	} else {
		message = err.Error()
	}

	return Error(c, code, message, nil)
}
