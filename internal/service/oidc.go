package service

import (
	"auth-service/core/pkg/security"
	"auth-service/internal/model"
	"auth-service/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

type OidcService struct {
	repo       *repository.OidcRepository
	utRepo     *repository.UtilsRepository
	encryptor  TokenEncryptor
	jwtManager *security.JWTManager
	isProd     bool
	accessTTL  int
	refreshTTL int
	fullHost   string
}

func NewOidcService(
	repo *repository.OidcRepository,
	utRepo *repository.UtilsRepository,
	enc TokenEncryptor,
	jwtManager *security.JWTManager,
	isProd bool,
	accessTTL,
	refreshTTL int,
	fullHost string,
) *OidcService {
	return &OidcService{
		repo:       repo,
		utRepo:     utRepo,
		encryptor:  enc,
		jwtManager: jwtManager,
		isProd:     isProd,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		fullHost:   fullHost,
	}
}

var OidcServiceTracer = otel.Tracer("oidc-service")

// Создание ODIC токена для S2S обмена на сессию
func (s *OidcService) PostOIDCToken(ctx context.Context, clientID, userID, nonce string) (string, error) {
	ctx, span := OidcServiceTracer.Start(ctx, "service.Register")
	defer span.End()
	code := uuid.New().String()

	err := s.repo.PostOIDCToken(ctx, clientID, code, userID)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить токен: %w", err)
	}

	err = s.repo.PostOIDCNonce(ctx, nonce, code)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить nonce: %w", err)
	}

	return code, nil
}

// Обмен OIDC токена на jwt сессиию
func (s *OidcService) GetOIDCSession(ctx context.Context, req model.GetOIDCSession) (model.TokenResponse, error) {
	UUID, err := s.repo.GetOIDCSession(ctx, req.Code, req.ClientID)
	if err != nil {
		return model.TokenResponse{}, fmt.Errorf("не удалось получить сессию по токену: %w", err)
	}

	nonce, err := s.repo.GetOIDCNonce(ctx, req.Code)
	if err != nil {
		return model.TokenResponse{}, fmt.Errorf("не удалось получить nonce по токену: %w", err)
	}

	refreshToken, accessToken, createdAt, err := CreateSession(
		ctx, s.utRepo,
		s.jwtManager,
		false, // Всегда false при авторизации через OIDC, т.к. OIDC запрщён без подтверждённого email
		UUID,
		req.ClientID,
		nonce,
		req.ClientIP,
		req.UserAgent,
		time.Duration(s.accessTTL)*time.Second,
		time.Duration(s.refreshTTL)*time.Second,
	)

	rowIDToken := model.IDTokenClaims{
		Iss:      s.fullHost,
		Sub:      UUID,
		Aud:      req.ClientID,
		Iat:      createdAt.Unix(),
		Exp:      createdAt.Add(time.Duration(s.accessTTL) * time.Second).Unix(),
		AuthTime: createdAt.Unix(),
		Nonce:    nonce,
	}

	IDToken, err := s.jwtManager.GenerateToken(UUID, time.Duration(s.accessTTL)*time.Second, map[string]interface{}{
		"iss":       rowIDToken.Iss,
		"sub":       rowIDToken.Sub,
		"aud":       rowIDToken.Aud,
		"iat":       rowIDToken.Iat,
		"exp":       rowIDToken.Exp,
		"auth_time": rowIDToken.AuthTime,
		"nonce":     rowIDToken.Nonce,
	})
	if err != nil {
		return model.TokenResponse{}, err
	}

	return model.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL),
		RefreshToken: refreshToken,
		IDToken:      IDToken,
		Scope:        "profile email",
	}, nil
}

// Регистрация API ключа
func (s *OidcService) RegisterPartner(ctx context.Context, UUID string, req model.RegisterPartner) (string, string, error) {
	origin, err := NormalizeOrigin(req.HomepageURL)
	if err != nil {
		return "", "", err
	}
	req.HomepageURL = origin

	API := uuid.New().String()

	rawAPIKey := "sk_live_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	hashedKey := HashAPIKey(rawAPIKey)

	payload := struct {
		HomepageURL  string `json:"home_page_url"`
		ReturnURI    string `json:"return_uri"`
		ClientID     string `json:"client_id"`
		OwnerUUID    string `json:"owner_uuid"`
		CompanyName  string `json:"company_name,omitempty"`
		FullName     string `json:"full_name"`
		CompanyEmail string `json:"company_email,omitempty"`
		API          string `json:"api"`
		APIKey       string `json:"api_key_hash"`
	}{
		HomepageURL:  req.HomepageURL,
		ReturnURI:    req.ReturnURI,
		ClientID:     req.ClientID,
		OwnerUUID:    UUID,
		CompanyName:  req.CompanyName,
		FullName:     req.FullName,
		CompanyEmail: req.CompanyEmail,
		API:          API,
		APIKey:       hashedKey,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("ошибка сериализации данных партнера: %w", err)
	}

	err = s.repo.CreatePartner(ctx, UUID, API, hashedKey, req.ClientID, req.ReturnURI, jsonData)
	if err != nil {
		return "", "", fmt.Errorf("ошибка создания партнера в БД: %w", err)
	}

	return API, rawAPIKey, nil
}
