CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    topic VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    retries INTEGER DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    scheduled_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox_events (scheduled_at)
WHERE status IN ('pending', 'failed');

CREATE OR REPLACE VIEW outbox_write_view AS
SELECT 
    id, 
    event_type, 
    topic, 
    payload, 
    scheduled_at
FROM outbox_events;