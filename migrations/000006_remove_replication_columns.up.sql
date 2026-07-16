-- Удаляем event_id из таблицы users и все связанные ограничения (включая UNIQUE)
ALTER TABLE users 
DROP COLUMN IF EXISTS event_id CASCADE;

-- Удаляем event_id из таблицы partners и связанный индекс (idx_partners_event_id)
ALTER TABLE partners 
DROP COLUMN IF EXISTS event_id CASCADE;