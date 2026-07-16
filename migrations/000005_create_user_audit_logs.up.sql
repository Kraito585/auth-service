CREATE TABLE IF NOT EXISTS user_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для мгновенной загрузки истории конкретного пользователя 
-- (например, для отображения в его личном кабинете или админке)
CREATE INDEX idx_user_audit_logs_user_id ON user_audit_logs(user_id);

-- Индекс для аналитики и техподдержки 
-- (например, найти всех, у кого был конфликт логина за последнюю неделю)
CREATE INDEX idx_user_audit_logs_action ON user_audit_logs(action);

-- GIN-индекс на случай, если придется искать по содержимому JSON 
-- (например, найти старый email в поле details->>'old_email')
CREATE INDEX idx_user_audit_logs_details ON user_audit_logs USING GIN (details);