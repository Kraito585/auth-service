package postgres

import (
	"context"
	"fmt"
	"auth-service/core/config"

	"auth-service/core/pkg/coretelemetry" //core:telemetry:end

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, cfg config.PostgresConfig, targetDB string) (*pgxpool.Pool, error) {
	// 1. Получаем DSN для конкретной базы данных
	dsn := cfg.DSN(targetDB)

	// 2. Парсим базовый конфиг пула
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга DSN: %w", err)
	}

	// 3. Применяем наши кастомные параметры из YAML
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns

	poolConfig.ConnConfig.Tracer = coretelemetry.NewPgxMetricsTracer()

	// 4. Создаем пул
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания пула: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("база данных %s недоступна: %w", targetDB, err)
	}

	return pool, nil
}
