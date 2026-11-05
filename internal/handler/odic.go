package handler

import (
	"auth-service/internal/service"

	"go.opentelemetry.io/otel"
)

type OdicHandler struct {
	srv     *service.OdicService
	is_prod bool
}

func NewOdicHandler(srv *service.OdicService, is_prod bool) *OdicHandler {
	return &OdicHandler{srv: srv}
}

var OdicHandlerTracer = otel.Tracer("http-handler")
