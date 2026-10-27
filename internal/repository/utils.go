package repository

import (
	coreredis "auth-service/core/pkg/redis"
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PermData struct {
	UUID  string
	Perm  string
	Value any
}

func SetUpdatePerm(ctx context.Context, db DBTX, rud *coreredis.Wrapper, data PermData) error {
	payload := map[string]any{
		data.Perm: data.Value,
	}

	query := `
		UPDATE users
		SET perms = COALESCE(permissions, '{}'::jsonb) || $2::jsonb
		WHERE id = $1;
	`
	_, err := db.Exec(ctx, query, data.UUID, payload)
	if err != nil {
		return fmt.Errorf("ошибка сохранения данных в базу данных 'SetUpdatePerm': %w", err)
	}

	key := data.Perm + ":" + data.UUID
	slog.Info(key)
	if err := rud.Set(ctx, key, data.Value, 0).Err(); err != nil {
		return fmt.Errorf("пользователь создан, но ошибка кэша сессии: %w", err)
	}

	return nil
}
