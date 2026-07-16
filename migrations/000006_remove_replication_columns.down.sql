-- 1. Восстанавливаем event_id в users
ALTER TABLE users ADD COLUMN event_id UUID;

-- Генерируем UUID для старых записей, чтобы не нарушить NOT NULL
UPDATE users SET event_id = gen_random_uuid() WHERE event_id IS NULL;

-- Возвращаем ограничения
ALTER TABLE users ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_event_id_key UNIQUE (event_id);


-- 2. Восстанавливаем event_id в partners
ALTER TABLE partners ADD COLUMN event_id UUID;

-- Генерируем UUID для старых записей
UPDATE partners SET event_id = gen_random_uuid() WHERE event_id IS NULL;

-- Возвращаем ограничения и индекс
ALTER TABLE partners ALTER COLUMN event_id SET NOT NULL;
CREATE INDEX idx_partners_event_id ON partners(event_id);