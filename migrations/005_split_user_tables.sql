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

INSERT INTO message_schedule (user_id, frequency, last_scheduled_sent, next_category_index)
    SELECT user_id, frequency, last_scheduled_sent, next_category_index
    FROM user_settings
    WHERE frequency IS NOT NULL
       OR last_scheduled_sent IS NOT NULL
       OR next_category_index IS NOT NULL;

INSERT INTO user_commands (user_id, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count)
    SELECT user_id, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count
    FROM user_settings
    WHERE last_get_news_now IS NOT NULL
       OR get_news_now_count IS NOT NULL
       OR last_get_last_24h IS NOT NULL
       OR get_last_24h_count IS NOT NULL;

ALTER TABLE user_settings
    DROP COLUMN IF EXISTS frequency,
    DROP COLUMN IF EXISTS last_scheduled_sent,
    DROP COLUMN IF EXISTS next_category_index,
    DROP COLUMN IF EXISTS last_get_news_now,
    DROP COLUMN IF EXISTS get_news_now_count,
    DROP COLUMN IF EXISTS last_get_last_24h,
    DROP COLUMN IF EXISTS get_last_24h_count;