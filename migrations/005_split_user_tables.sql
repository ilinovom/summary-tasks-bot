ALTER TABLE user_settings
    DROP COLUMN IF EXISTS frequency,
    DROP COLUMN IF EXISTS last_scheduled_sent,
    DROP COLUMN IF EXISTS next_category_index,
    DROP COLUMN IF EXISTS last_get_news_now,
    DROP COLUMN IF EXISTS get_news_now_count,
    DROP COLUMN IF EXISTS last_get_last_24h,
    DROP COLUMN IF EXISTS get_last_24h_count;

CREATE TABLE IF NOT EXISTS message_schedule (
    user_id BIGINT PRIMARY KEY REFERENCES user_settings (user_id),
    frequency INTEGER,
    last_scheduled_sent BIGINT,
    next_category_index INTEGER
);

CREATE TABLE IF NOT EXISTS user_commands (
    user_id BIGINT PRIMARY KEY REFERENCES user_settings (user_id),
    last_get_news_now BIGINT,
    get_news_now_count INTEGER,
    last_get_last_24h BIGINT,
    get_last_24h_count INTEGER
);
