package pkgkafka

import (
	"auth-service/core/config"
	"auth-service/core/pkg/coretelemetry"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type OutboxEvent struct {
	ID        uuid.UUID
	EventType string
	Topic     string
	Payload   []byte
	CreatedAt time.Time
}

type OutboxRelay struct {
	db        *pgxpool.Pool
	nc        *nats.Conn
	js        jetstream.JetStream
	tick      *time.Ticker
	isHealthy atomic.Bool
	isProd    bool
}

func NewOutboxRelay(db *pgxpool.Pool, kCfg config.KafkaConfig, isProd bool) *OutboxRelay {
	slog.Info("prode status: %w", isProd)
	opts := []nats.Option{
		nats.Name("auth-service-outbox-relay"),
		nats.Timeout(10 * time.Second),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Бесконечные реконнекты при обрыве
	}

	// Если в конфиге включен TLS
	if tlsCfg := kCfg.TLSConfig(); tlsCfg != nil {
		opts = append(opts, nats.Secure(tlsCfg))
	}

	// Склеиваем массив брокеров в строку формата "nats://host1:4222,nats://host2:4222"
	brokersURL := strings.Join(kCfg.Brokers, ",")

	var nc *nats.Conn
	var js jetstream.JetStream
	var err error

	slog.Info("Ждем готовности NATS JetStream...")
	for i := 0; i < 15; i++ {
		nc, err = nats.Connect(brokersURL, opts...)
		if err == nil {
			js, err = jetstream.New(nc)
			if err == nil {
				break
			}
			nc.Close()
		}
		slog.Info("NATS еще не готов", slog.Int("attempt", i), slog.Any("error", err))
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		slog.Error("Не удалось подключиться к NATS после всех попыток", slog.Any("error", err))
	} else if js != nil {
		// Уникальный список топиков без дубликатов
		subjects := []string{"synk_auth", "mail_data", "synk-auth", "mail-data"}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
			Name:      "EVENTS",
			Subjects:  subjects,
			Storage:   jetstream.FileStorage,
			Retention: jetstream.LimitsPolicy,
		})
		cancel()

		if err != nil {
			slog.Error("Ошибка создания/обновления стрима в NATS", slog.Any("error", err))
		} else {
			slog.Info("NATS готов, стрим EVENTS обновлен.", slog.Any("subjects", subjects))
		}
	}

	return &OutboxRelay{
		db:     db,
		nc:     nc,
		js:     js,
		tick:   time.NewTicker(time.Duration(kCfg.OutboxRelay.Tick) * time.Millisecond),
		isProd: isProd,
	}
}

func (r *OutboxRelay) IsHealthy() bool {
	return r.isHealthy.Load() && r.nc != nil && r.nc.IsConnected()
}

func (r *OutboxRelay) Start(ctx context.Context) {
	slog.Info("Outbox Relay started (NATS JetStream)")
	r.isHealthy.Store(true)
	defer r.isHealthy.Store(false)
	defer func() {
		if r.nc != nil {
			// Drain аккуратно досылает буфер перед закрытием соединения
			_ = r.nc.Drain()
		}
	}()
	defer r.tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.tick.C:
			r.processBatch(ctx)
		}
	}
}

var outboxRelayTracer = otel.Tracer("outbox-relay")

func (r *OutboxRelay) processBatch(ctx context.Context) {
	ctx, span := outboxRelayTracer.Start(ctx, "outbox_relay.processBatch")
	defer span.End()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	fetchEventsSQL := `
	UPDATE outbox_events
		SET status = 'processing'
		WHERE id IN (
			SELECT id
			FROM outbox_events
			WHERE status = 'pending' 
				AND (scheduled_at <= NOW())
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
	RETURNING id, event_type, topic, payload, created_at;`

	rows, err := tx.Query(ctx, fetchEventsSQL, 50)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Topic, &e.Payload, &e.CreatedAt); err != nil {
			return
		}
		events = append(events, e)
	}

	if len(events) == 0 {
		return
	}

	natsMsgs := make([]*nats.Msg, len(events))
	ids := make([]uuid.UUID, len(events))

	propagator := otel.GetTextMapPropagator()

	for i, e := range events {
		timestampStr := strconv.FormatInt(e.CreatedAt.UnixNano(), 10)

		headers := make(nats.Header)
		headers.Set("x-event-id", e.ID.String())
		headers.Set("x-event-type", e.EventType)
		headers.Set("x-created-at", timestampStr)

		// Встроенная дедупликация NATS JetStream (полный аналог идемпотентности):
		// Если транзакция в БД откатится после публикации и сообщение уйдет повторно,
		// NATS отбросит дубликат по этому заголовку.
		headers.Set(nats.MsgIdHdr, e.ID.String())

		// nats.Header имеет тип map[string][]string, поэтому напрямую совместим с HeaderCarrier
		propagator.Inject(ctx, propagation.HeaderCarrier(headers))

		natsMsgs[i] = &nats.Msg{
			Subject: e.Topic,
			Data:    e.Payload,
			Header:  headers,
		}
		ids[i] = e.ID
	}

	err = coretelemetry.ObserveKafka("write", func() error {
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Публикуем пачку асинхронно (pipeline) и ждем подтверждения сохранения на диск
		futures := make([]jetstream.PubAckFuture, 0, len(natsMsgs))
		for _, msg := range natsMsgs {
			pubAckFuture, err := r.js.PublishMsgAsync(msg)
			if err != nil {
				return err
			}
			futures = append(futures, pubAckFuture)
		}

		for _, f := range futures {
			select {
			case <-f.Ok():
			case err := <-f.Err():
				return err
			case <-writeCtx.Done():
				return writeCtx.Err()
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("NATS write error",
			slog.Any("error", err),
			slog.String("first_event_topic", events[0].Topic),
		)
		return
	}

	_, err = tx.Exec(ctx, "DELETE FROM outbox_events WHERE id = ANY($1)", ids)
	if err != nil {
		slog.Error("Finalize error", slog.Any("error", err))
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("Commit error", slog.Any("error", err))
		return
	}

	// Выполнится только если Commit прошел успешно (err == nil)
	coretelemetry.OutboxRelayedTotal.Add(float64(len(events)))
}
