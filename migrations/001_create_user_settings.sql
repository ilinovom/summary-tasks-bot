CREATE TABLE IF NOT EXISTS user_settings (
    user_id BIGINT PRIMARY KEY,
    active BOOLEAN,
    info_types JSONB,
    categories JSONB,
    frequency INTEGER,
    tariff TEXT,
    last_scheduled_sent BIGINT,
    last_get_news_now BIGINT,
    get_news_now_count INTEGER
);
