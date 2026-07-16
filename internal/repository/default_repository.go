package repository

import (
	coreredis "auth-service/core/pkg/redis"
	"auth-service/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

type DefaultRepository struct {
	db *pgxpool.Pool
	r  *coreredis.Wrapper
}

func NewDefaultRepository(
	db *pgxpool.Pool,
	r *coreredis.Wrapper,
) *DefaultRepository {
	return &DefaultRepository{
		db: db,
		r:  r,
	}
}

var defaultRepoTracer = otel.Tracer("default-repository")

func (r *DefaultRepository) WatchExistUserData(ctx context.Context, req *model.RegisterRequest) (bool, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.WatchExistUserData")
	defer span.End()

	query := "SELECT EXISTS(SELECT 1 FROM users WHERE login = $1 OR email = $2);"

	var exists bool

	err := r.db.QueryRow(ctx, query, req.Login, req.Email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}

	return exists, nil
}

func (r *DefaultRepository) Register(ctx context.Context, data model.RepoRegisterResponse) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.Register")
	defer span.End()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO users (id, login, email, hash_password, created_at, updated_at) 
            VALUES ($1, $2, $3, $4, $5, $5)`

	_, err = tx.Exec(ctx, query, data.UUID, data.Login, data.Email, data.HashPassword, time.UnixMilli(data.CreatedAt))
	if err != nil {
		return err
	}

	eventId, err := uuid.NewV7()

	kMailData, err := json.Marshal(map[string]interface{}{
		"UUID":      data.UUID,
		"Email":     data.Email,
		"Code":      data.Code,
		"EventID":   eventId.String(),
		"UpdatedAt": data.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("ошибка сериализации данных для outbox: %w", err)
	}

	outboxQuery := "INSERT INTO outbox_events (id, event_type, topic, payload) VALUES ($1, $2, $3, $4)"
	_, err = tx.Exec(ctx, outboxQuery, eventId, "mail.user.registered", "mail-data", kMailData)
	if err != nil {
		return err
	}

	key := "code:" + data.UUID
	if err := r.r.Set(ctx, key, data.Code, time.Hour).Err(); err != nil {
		return fmt.Errorf("пользователь создан, но ошибка кэша сессии: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *DefaultRepository) NewSession(ctx context.Context, key string, data model.SessionData) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.NewSession")
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

func (r *DefaultRepository) ExistSession(ctx context.Context, key string) (int64, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.ExistSession")
	defer span.End()

	// Exists возвращает количество совпадений. Для одного ключа это будет 1 или 0.
	count, err := r.r.Exists(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("ошибка при проверке существования сессии: %w", err)
	}

	return count, nil
}

func (r *DefaultRepository) HotSwapEmail(ctx context.Context, UUID string, Email string, code string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.HotSwapEmail")
	defer span.End()

	now := time.Now()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := "UPDATE users SET email = $1, updated_at = $2 WHERE id = $3 AND email_verified_at IS NULL"
	result, err := tx.Exec(ctx, query, Email, now, UUID)
	if err != nil {
		return fmt.Errorf("ошибка обновления адреса: %w", err)
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 1 {
		query = "INSERT INTO outbox_events (id, event_type, topic, payload) VALUES ($1, $2, $3, $4)"

		mEventId, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("ошибка генерации EventId: %w", err)
		}

		kMailData, err := json.Marshal(map[string]interface{}{
			"UUID":      UUID,
			"Email":     Email,
			"Code":      code,
			"EventID":   mEventId.String(),
			"UpdatedAt": now.UnixMilli(),
		})
		if err != nil {
			return fmt.Errorf("ошибка сериализации данных для outbox: %w", err)
		}

		_, err = tx.Exec(ctx, query, mEventId, "mail.user.hot.swap.email", "mail-data", kMailData)
		if err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		key := "code:" + UUID
		if err := r.r.Set(ctx, key, code, time.Hour).Err(); err != nil {
			return fmt.Errorf("email обновлен, но ошибка перевыпуска кода подтверждения: %w", err)
		}

		return nil
	}
	tx.Rollback(ctx)

	// 2. Если обновилось 0 строк, разбираемся в причине (этот код выполнится редко)
	var verifiedAt *time.Time
	checkQuery := "SELECT email_verified_at FROM users WHERE id = $1"

	err = r.db.QueryRow(ctx, checkQuery, UUID).Scan(&verifiedAt)
	if err != nil {
		// sql.ErrNoRows перехватится здесь
		return fmt.Errorf("не удалось найти аккаунт")
	}

	if verifiedAt != nil {
		return fmt.Errorf("почта уже верифицирована, изменение запрещено")
	}

	// Если мы дошли сюда, значит произошла какая-то аномалия
	return fmt.Errorf("не удалось обновить email по неизвестной причине")
}

func (r *DefaultRepository) ConfirmCode(ctx context.Context, code string, UUID string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.ConfirmCode")
	defer span.End()

	key := "code:" + UUID

	curentCode, err := r.r.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("код подтверждения истек или не существует")
	} else if err != nil {
		return fmt.Errorf("ошибка чтения кода из кэша: %w", err)
	}

	if curentCode != code {
		return fmt.Errorf("код не валиден")
	}

	now := time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := "UPDATE users SET email_verified_at = $1, updated_at = $1 WHERE id = $2"

	_, err = tx.Exec(ctx, query, now, UUID)
	if err != nil {
		return fmt.Errorf("ошибка сервера: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *DefaultRepository) DelUsedCode(ctx context.Context, UUID string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.DelUsedCode")
	defer span.End()

	key := "code:" + UUID
	if err := r.r.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("Внимание: не удалось удалить использованный код из Redis для %s: %v", UUID, err)
	}

	return nil
}

func (r *DefaultRepository) FetchTOTPData(ctx context.Context, UUID string) (string, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.FetchTOTPData")
	defer span.End()

	query := "SELECT login FROM users WHERE id = $1 AND totp_secret_encrypted IS NULL"
	var login string
	err := r.db.QueryRow(ctx, query, UUID).Scan(&login)
	if err != nil {
		return "", fmt.Errorf("не удалось найти аккаунт")
	}

	return login, nil
}

func (r *DefaultRepository) CacheTOTP(ctx context.Context, UUID string, encryptedSecret string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.CacheTOTP")
	defer span.End()

	key := "totp:secret:" + UUID
	if err := r.r.Set(ctx, key, encryptedSecret, 2*time.Hour).Err(); err != nil {
		return fmt.Errorf("ошибка сохраниения кода: %w", err)
	}
	return nil
}

func (r *DefaultRepository) GetTOTPSecretCache(ctx context.Context, UUID string) (string, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.GetTOTPSecretCache")
	defer span.End()

	key := "totp:secret:" + UUID
	secret, err := r.r.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("код подтверждения истек или не существует")
	}

	return secret, nil
}

func (r *DefaultRepository) SaveTOTPData(ctx context.Context, UUID string, encryptedSecret string, codes []byte) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.SaveTOTPData")
	defer span.End()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	query := "UPDATE users SET totp_secret_encrypted = $1, hash_totp_reset_codes = $2, totp_enabled_at = $3, updated_at = $3, auth_preference = $5 WHERE id = $4"
	_, err = tx.Exec(ctx, query, encryptedSecret, codes, now, UUID, "AuthMethodPasswordTOTP")
	if err != nil {
		return fmt.Errorf("ошибка сохранения данных: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	key := "totp:secret:" + UUID
	if err := r.r.Del(ctx, key).Err(); err != nil {
		slog.Warn("Не удалось удалить использованный код из Redis", slog.String("uuid", UUID), slog.Any("error", err))
	}

	return nil
}

func (r *DefaultRepository) Login(ctx context.Context, login string) (string, string, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.Login")
	defer span.End()

	query := "SELECT id, auth_preference FROM users WHERE login = $1"
	var UUID string
	var authType string

	err := r.db.QueryRow(ctx, query, login).Scan(&UUID, &authType)
	if err != nil {
		return "", "", fmt.Errorf("Пользователь не найден или внутрения ошибка баззы данных: %w", err)
	}

	return UUID, authType, nil
}

func (r *DefaultRepository) SendEmailCode(ctx context.Context, UUID string, code string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.SendEmailCode")
	defer span.End()

	key := "code:" + UUID
	if err := r.r.Set(ctx, key, code, 20*time.Minute).Err(); err != nil {
		return fmt.Errorf("ошибка сохранения кода подтверждения: %w", err)
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("ошибка генерации EventId: %w", err)
	}

	kMailData, err := json.Marshal(map[string]interface{}{
		"UUID":    UUID,
		"Code":    code,
		"EventID": eventID.String(),
	})
	if err != nil {
		return fmt.Errorf("ошибка сериализации данных для outbox: %w", err)
	}

	query := "INSERT INTO outbox_events (id, event_type, topic, payload) VALUES ($1, $2, $3, $4)"
	_, err = r.db.Exec(ctx, query, eventID, "mail.user.send.code", "mail-data", kMailData)
	if err != nil {
		return err
	}

	return nil
}

func (r *DefaultRepository) GetAuthData(ctx context.Context, UUID string, authMethod string) (model.LoginData, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.GetAuthData")
	defer span.End()

	var data model.LoginData
	var err error
	if authMethod == "AuthMethodPasswordEmail" {
		key := "code:" + UUID
		data.CurrentCode, err = r.r.Get(ctx, key).Result()
		if err == redis.Nil {
			slog.Warn("код отсутствует")
		} else if err != nil {
			return data, fmt.Errorf("ошибка чтения кода из кэша: %w", err)
		}
	}

	query := "SELECT hash_password, auth_preference, totp_secret_encrypted, email_verified_at FROM users WHERE id = $1"

	err = r.db.QueryRow(ctx, query, UUID).Scan(
		&data.PasswordHash,
		&data.AuthMethod,
		&data.TOTPSecretEncrypted,
		&data.EmailVerifiedAt,
	)
	if err != nil {
		return data, fmt.Errorf("пользователь не найден: %w", err)
	}

	return data, nil
}

func (r *DefaultRepository) PostSSOToken(ctx context.Context, UUID string, token string) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.PostSSOToken")
	defer span.End()

	key := "sso:token:" + token
	if err := r.r.Set(ctx, key, UUID, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("ошибка сохранения кода подтверждения: %w", err)
	}
	return nil
}

func (r *DefaultRepository) GetSSOToken(ctx context.Context, token string) (string, error) {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.GetSSOToken")
	defer span.End()

	key := "sso:token:" + token
	UUID, err := r.r.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("код аунтетификации истек или не существует")
	} else if err != nil {
		return "", fmt.Errorf("ошибка чтения кода из кэша: %w", err)
	}

	return UUID, nil
}

func (r *DefaultRepository) CreatePartner(ctx context.Context, UUID string, origin string, hashedKey string, jsonData []byte) error {
	ctx, span := defaultRepoTracer.Start(ctx, "repository.CreatePartner")
	defer span.End()

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

	err = r.r.HSet(ctx, "api:keys:secrets", hashedKey, UUID).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения ключа в кэш: %w", err)
	}

	err = r.r.SAdd(ctx, "cors:allowed_origins", origin).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения CORS в кэш: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		hdelErr := r.r.HDel(ctx, "api:keys:secrets", hashedKey).Err()
		sremErr := r.r.SRem(ctx, "cors:allowed_origins", origin).Err()

		// Формируем детальный отчет об ошибке, чтобы ничего не потерять
		if hdelErr != nil || sremErr != nil {
			return fmt.Errorf("КРИТИЧНО: провал Commit БД (%v) И провал отката Redis (HDel: %v, SRem: %v)", err, hdelErr, sremErr)
		}

		return fmt.Errorf("ошибка коммита БД, изменения в Redis успешно отменены: %w", err)
	}

	return nil
}
