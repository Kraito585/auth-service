package service

import (
	"auth-service/core/pkg/security"
	"auth-service/internal/repository"

	"go.opentelemetry.io/otel"
)

type OdicService struct {
	repo       *repository.DefaultRepository
	encryptor  TokenEncryptor
	jwtManager *security.JWTManager
	isProd     bool
}

func NewOdicService(repo *repository.DefaultRepository, enc TokenEncryptor, jwtManager *security.JWTManager, isProd bool) *OdicService {
	return &OdicService{
		repo:       repo,
		encryptor:  enc,
		jwtManager: jwtManager,
		isProd:     isProd,
	}
}

var OdicServiceTracer = otel.Tracer("odic-service")
