# Интеграция с Kafka

Микросервис использует Kafka для паттерна **Outbox Relay** — гарантированной доставки событий.

## Outbox Relay (`core/pkg/kafka/outbox_relay.go`)

Паттерн Outbox решает проблему двойной записи (dual write) — когда нужно атомарно записать данные в PostgreSQL и отправить событие в Kafka.

### Принцип работы

```
Запрос → PostgreSQL (данные + outbox_event в одной транзакции)
                  ↓
        OutboxRelay.poll() ← читает неотправленные события из таблицы outbox_events
                  ↓
        Kafka.Produce() → отправляет событие в топик
                  ↓
        Удаляет запись из outbox_events (или помечает sent = true)
```

### Конфигурация

```yaml
kafka:
  brokers: ["kafka:9092"]
  outbox_topic: "auth.events"
  outbox_poll_interval: 1s
  outbox_batch_size: 100
```

### Структура OutboxRelay

```go
type OutboxRelay struct {
    reader   *postgres.Wrapper  // для чтения outbox_events
    writer   *kafka.Writer      // для отправки в Kafka
    interval time.Duration
    batchSize int
}
```

Методы:
- `NewOutboxRelay(reader, writer, cfg)` — конструктор
- `Run(ctx)` — основной цикл: poll → produce → delete (в горутине)
- `poll(ctx)` — читает не более `batchSize` событий из outbox_events
- `produce(ctx, events)` — отправляет пачку событий в Kafka
- `ack(ctx, eventIDs)` — удаляет успешно отправленные события

### Таблица outbox_events

```sql
CREATE TABLE outbox_events (
    id         UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

