package pkgkafka

import (
	"auth-service/core/config"
	"context"

	"auth-service/core/pkg/coretelemetry"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
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
	Writer    *kafka.Writer
	tick      *time.Ticker
	isHealthy atomic.Bool
	isProd    bool
}

func NewOutboxRelay(db *pgxpool.Pool, kCfg config.KafkaConfig, isProd bool) *OutboxRelay {

	transport := &kafka.Transport{
		SASL:        kCfg.SASLMechanism(),
		IdleTimeout: 30 * time.Second,
		TLS:         kCfg.TLSConfig(),
	}

	if !isProd {
		client := &kafka.Client{
			Addr:      kafka.TCP(kCfg.Brokers...),
			Transport: transport,
		}

		slog.Info("Ждем готовности Kafka...")
		for i := 0; i < 15; i++ {
			_, err := client.CreateTopics(context.Background(), &kafka.CreateTopicsRequest{
				Topics: []kafka.TopicConfig{
					{Topic: "synk_auth", NumPartitions: 1, ReplicationFactor: 1},
					{Topic: "mail_data", NumPartitions: 1, ReplicationFactor: 1},
				},
			})
			if err == nil {
				slog.Info("Kafka готова, топики проверены.")
				break
			}
			slog.Info("Kafka еще не готова", slog.Int("attempt", i), slog.Any("error", err))
			time.Sleep(2 * time.Second)
		}
	}

	return &OutboxRelay{
		db:   db,
		tick: time.NewTicker(time.Duration(kCfg.OutboxRelay.Tick) * time.Millisecond),
		Writer: &kafka.Writer{
			Addr:                   kafka.TCP(kCfg.Brokers...),
			Balancer:               &kafka.Hash{},
			MaxAttempts:            5,
			RequiredAcks:           kafka.RequireOne,
			Transport:              transport,
			WriteTimeout:           10 * time.Second,
			AllowAutoTopicCreation: true,
			BatchTimeout:           10 * time.Millisecond,
		},
	}
}

func (r *OutboxRelay) IsHealthy() bool {
	return r.isHealthy.Load()
}

func (r *OutboxRelay) Start(ctx context.Context) {
	slog.Info("Outbox Relay started")
	r.isHealthy.Store(true)
	defer r.isHealthy.Store(false)
	defer r.Writer.Close()
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

	kafkaMsgs := make([]kafka.Message, len(events))
	ids := make([]uuid.UUID, len(events))

	propagator := otel.GetTextMapPropagator()

	for i, e := range events {
		timestampStr := strconv.FormatInt(e.CreatedAt.UnixNano(), 10)

		headers := []kafka.Header{
			{Key: "x-event-id", Value: []byte(e.ID.String())},
			{Key: "x-event-type", Value: []byte(e.EventType)},
			{Key: "x-created-at", Value: []byte(timestampStr)},
		}

		// Инжектим trace context (traceparent + tracestate) в headers Kafka
		traceHeaders := make(propagation.MapCarrier)
		propagator.Inject(ctx, traceHeaders)
		for k, v := range traceHeaders {
			headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
		}

		kafkaMsgs[i] = kafka.Message{
			Topic:   e.Topic,
			Key:     []byte(e.ID.String()),
			Value:   e.Payload,
			Headers: headers,
		}
		ids[i] = e.ID
	}

	err = coretelemetry.ObserveKafka("write", func() error {

		err = r.Writer.WriteMessages(ctx, kafkaMsgs...)

		return err
	})

	if err != nil {
		slog.Error("Kafka write error", slog.Any("error", err))
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
