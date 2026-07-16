package health

import (
	"context"

	pkgkafka "auth-service/core/pkg/kafka"

	coreredis "auth-service/core/pkg/redis"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Checker interface {
	Name() string
	IsHealthy() bool
}

type MigrationChecker struct {
	ready *atomic.Bool
}

func NewMigrationChecker(ready *atomic.Bool) *MigrationChecker {
	return &MigrationChecker{ready: ready}
}

func (m *MigrationChecker) Name() string {
	return "Database Migrations"
}

func (m *MigrationChecker) IsHealthy() bool {
	return m.ready.Load()
}

type RedisChecker struct {
	client *coreredis.Wrapper
}

func NewRedisChecker(client *coreredis.Wrapper) *RedisChecker {
	return &RedisChecker{client: client}
}

func (r *RedisChecker) Name() string {
	return "Redis Cache"
}

func (r *RedisChecker) IsHealthy() bool {
	// 1. Проверяем, инициализирован ли вообще клиент
	if r.client == nil {
		return false
	}

	// 2. Задаем жесткий таймаут, чтобы проверка не зависла, если сеть пропадет
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 3. Делаем реальный сетевой запрос к Redis (используем метод Ping из нашего Wrapper'а)
	err := r.client.Ping(ctx).Err()

	// Если ошибки нет (err == nil), значит Redis жив и отвечает
	return err == nil
}

type PostgresChecker struct {
	pool *pgxpool.Pool
}

func NewPostgresChecker(pool *pgxpool.Pool) *PostgresChecker {
	return &PostgresChecker{pool: pool}
}

func (p *PostgresChecker) Name() string { return "PostgreSQL Database" }

func (p *PostgresChecker) IsHealthy() bool {
	if p.pool == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := p.pool.Ping(ctx)
	return err == nil
}

type OutboxRelayChecker struct {
	relay *pkgkafka.OutboxRelay
}

func NewOutboxRelayChecker(relay *pkgkafka.OutboxRelay) *OutboxRelayChecker {
	return &OutboxRelayChecker{relay: relay}
}

func (c *OutboxRelayChecker) Name() string {
	return "Kafka Outbox Relay"
}

func (c *OutboxRelayChecker) IsHealthy() bool {
	if c.relay == nil {
		return false
	}
	// Вызываем твой готовый метод!
	return c.relay.IsHealthy()
}
