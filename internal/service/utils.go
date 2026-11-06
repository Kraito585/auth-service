package service

import (
	"auth-service/core/pkg/security"
	"auth-service/internal/model"
	"auth-service/internal/repository"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func CreateSession(
	ctx context.Context,
	repo *repository.UtilsRepository,
	jwtManager *security.JWTManager,
	emailLock bool,
	UUID string,
	ClientID string,
	Nonce string,
	ip string,
	userAgent string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) (string, string, time.Time, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return "", "", time.Time{}, err
	}

	payload := map[string]interface{}{
		"role":       "user",
		"session_id": sessionID,
		"platform":   "web",
	}

	if emailLock {
		payload["email"] = false
	}

	refreshToken, err := jwtManager.GenerateToken(UUID, refreshTTL, payload)
	if err != nil {
		return "", "", time.Time{}, err
	}

	accessToken, err := jwtManager.GenerateToken(UUID, accessTTL, payload)
	if err != nil {
		return "", "", time.Time{}, err
	}

	now := time.Now()

	newSessionData := model.SessionData{
		RefreshToken: refreshToken,
		IP:           ip,
		UserAgent:    userAgent,
		ClientID:     ClientID,
		Nonce:        Nonce,
		CreatedAt:    now.Unix(),
		ExpiresAt:    now.Add(7 * 24 * time.Hour).UnixMilli(),
	}

	key := fmt.Sprintf("session:%s:%s", UUID, sessionID.String())
	err = repo.NewSession(ctx, key, newSessionData)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return refreshToken, accessToken, now, nil
}
