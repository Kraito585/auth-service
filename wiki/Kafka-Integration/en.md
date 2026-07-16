# Kafka Integration

The microservice uses Kafka for the **Outbox Relay** pattern — guaranteed event delivery.

## Outbox Relay (`core/pkg/kafka/outbox_relay.go`)

The Outbox pattern solves the dual-write problem — when you need to atomically write data to PostgreSQL and send an event to Kafka.

### How It Works

```
Request → PostgreSQL (data + outbox_event in a single transaction)
                  ↓
        OutboxRelay.poll() ← reads unsent events from outbox_events table
                  ↓
        Kafka.Produce() → sends event to topic
                  ↓
        Deletes record from outbox_events (or marks sent = true)
```

### Configuration

```yaml
kafka:
  brokers: ["kafka:9092"]
  outbox_topic: "auth.events"
  outbox_poll_interval: 1s
  outbox_batch_size: 100
```

### OutboxRelay Structure

```go
type OutboxRelay struct {
    reader   *postgres.Wrapper  // for reading outbox_events
    writer   *kafka.Writer      // for sending to Kafka
    interval time.Duration
    batchSize int
}
```

Methods:
- `NewOutboxRelay(reader, writer, cfg)` — constructor
- `Run(ctx)` — main loop: poll → produce → delete (in goroutine)
- `poll(ctx)` — reads up to `batchSize` events from outbox_events
- `produce(ctx, events)` — sends a batch of events to Kafka
- `ack(ctx, eventIDs)` — deletes successfully sent events

### outbox_events Table

```sql
CREATE TABLE outbox_events (
    id         UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

