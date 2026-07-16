CREATE TABLE IF NOT EXISTS partners (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE
);

-- Индекс для быстрого поиска по конкретному событию
CREATE INDEX idx_partners_event_id ON partners(event_id);

-- GIN-индекс для быстрого поиска внутри JSONB структуры
CREATE INDEX idx_partners_data ON partners USING GIN (data);