package repository

import (
	coreredis "auth-service/core/pkg/redis"
	"auth-service/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

type OidcRepository struct {
	db  *pgxpool.Pool
	r   *coreredis.Wrapper
	rud *coreredis.Wrapper
}

func NewOidcRepository(
	db *pgxpool.Pool,
	r *coreredis.Wrapper,
	rud *coreredis.Wrapper,
) *OidcRepository {
	return &OidcRepository{
		db:  db,
		r:   r,
		rud: rud,
	}
}

var oidcRepoTracer = otel.Tracer("oidc-repository")

// Создание ODIC токена для S2S обмена на сессию
func (r *OidcRepository) PostOIDCToken(ctx context.Context, clientID string, code string, userID string) error {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.PostOIDCToken")
	defer span.End()

	key := clientID + ":" + code
	if err := r.r.Set(ctx, key, userID, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("ошибка сохранения кода подтверждения: %w", err)
	}
	return nil
}

func (r *OidcRepository) PostOIDCNonce(ctx context.Context, nonce string, code string) error {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.PostOIDCNonce")
	defer span.End()

	if err := r.r.Set(ctx, code, nonce, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("ошибка сохранения nonce: %w", err)
	}
	return nil
}

func (r *OidcRepository) GetOIDCSession(ctx context.Context, code string, clientID string) (string, error) {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.GetOIDCSession")
	defer span.End()

	key := clientID + ":" + code
	UUID, err := r.r.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("код аунтетификации истек или не существует")
	} else if err != nil {
		return "", fmt.Errorf("ошибка чтения кода из кэша: %w", err)
	}

	return UUID, nil
}

func (r *OidcRepository) GetOIDCNonce(ctx context.Context, code string) (string, error) {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.GetOIDCNonce")
	defer span.End()

	nonce, err := r.r.GetDel(ctx, code).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("код аунтетификации истек или не существует")
	} else if err != nil {
		return "", fmt.Errorf("ошибка чтения nonce из кэша: %w", err)
	}

	return nonce, nil
}

// Дубикат функции
func (r *OidcRepository) NewSession(ctx context.Context, key string, data model.SessionData) error {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.NewSession")
	defer span.End()

	sessionBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("ошибка сериализации сессии для redis: %w", err)
	}

	err = r.r.Set(ctx, key, sessionBytes, 7*24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения сессии: %w", err)
	}

	return nil
}

func (r *OidcRepository) CreatePartner(
	ctx context.Context,
	UUID string,
	API string,
	hashedKey string,
	clientID string,
	redirectURI string,
	jsonData []byte,
) error {
	ctx, span := oidcRepoTracer.Start(ctx, "repository.CreatePartner")
	defer span.End()

	//Сгенерировал в начале чтобы не было проблем при откате транзакции
	key := "clientID:" + clientID

	query := `
		INSERT INTO partners (id, data, created_at)
		VALUES ($1, $2, $3)
	`

	now := time.Now()

	eventID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, query, UUID, jsonData, now)
	if err != nil {
		return fmt.Errorf("не удалось выполнить sql insert: %w", err)
	}

	kMailData, err := json.Marshal(map[string]interface{}{
		"UUID":      UUID,
		"EventID":   eventID,
		"UpdatedAt": now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("ошибка сериализации данных для outbox: %w", err)
	}

	outboxQuery := "INSERT INTO outbox_events (id, event_type, topic, payload) VALUES ($1, $2, $3, $4)"
	_, err = tx.Exec(ctx, outboxQuery, eventID, "mail.user.create.api.key", "mail-data", kMailData)
	if err != nil {
		return err
	}

	err = r.r.HSet(ctx, "clients:secrets", hashedKey, API).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения ключа в кэш: %w", err)
	}

	err = r.r.SAdd(ctx, key, redirectURI).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения CORS в кэш: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		hdelErr := r.r.HDel(ctx, "clients:secrets", hashedKey).Err()
		sremErr := r.r.SRem(ctx, key, redirectURI).Err()

		if hdelErr != nil || sremErr != nil {
			return fmt.Errorf("КРИТИЧНО: провал Commit БД (%v) И провал отката Redis (HDel: %v, SRem: %v)", err, hdelErr, sremErr)
		}

		return fmt.Errorf("ошибка коммита БД, изменения в Redis успешно отменены: %w", err)
	}

	return nil
}
