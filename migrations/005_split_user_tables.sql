BEGIN;

-- 1) Таблицы (если ещё не созданы)
CREATE TABLE IF NOT EXISTS message_schedule (
                                                user_id BIGINT PRIMARY KEY REFERENCES user_settings (user_id) ON DELETE CASCADE,
    frequency INTEGER,
    last_scheduled_sent BIGINT,
    next_category_index INTEGER
    );

CREATE TABLE IF NOT EXISTS user_commands (
                                             user_id BIGINT PRIMARY KEY REFERENCES user_settings (user_id) ON DELETE CASCADE,
    last_get_news_now BIGINT,
    get_news_now_count INTEGER,
    last_get_last_24h BIGINT,
    get_last_24h_count INTEGER
    );

-- 2) Гарантированно создаём строки для всех user_id
INSERT INTO message_schedule (user_id)
SELECT us.user_id
FROM user_settings us
    ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_commands (user_id)
SELECT us.user_id
FROM user_settings us
    ON CONFLICT (user_id) DO NOTHING;

-- 3) Дозаполняем поля из старых колонок, где они не NULL
UPDATE message_schedule ms
SET frequency           = us.frequency,
    last_scheduled_sent = us.last_scheduled_sent,
    next_category_index = us.next_category_index
    FROM user_settings us
WHERE ms.user_id = us.user_id
  AND (us.frequency IS NOT NULL
   OR us.last_scheduled_sent IS NOT NULL
   OR us.next_category_index IS NOT NULL);

UPDATE user_commands uc
SET last_get_news_now   = us.last_get_news_now,
    get_news_now_count  = us.get_news_now_count,
    last_get_last_24h   = us.last_get_last_24h,
    get_last_24h_count  = us.get_last_24h_count
    FROM user_settings us
WHERE uc.user_id = us.user_id
  AND (us.last_get_news_now IS NOT NULL
   OR us.get_news_now_count IS NOT NULL
   OR us.last_get_last_24h IS NOT NULL
   OR us.get_last_24h_count IS NOT NULL);

-- 4) Удаляем старые поля из user_settings
ALTER TABLE user_settings
DROP COLUMN IF EXISTS frequency,
    DROP COLUMN IF EXISTS last_scheduled_sent,
    DROP COLUMN IF EXISTS next_category_index,
    DROP COLUMN IF EXISTS last_get_news_now,
    DROP COLUMN IF EXISTS get_news_now_count,
    DROP COLUMN IF EXISTS last_get_last_24h,
    DROP COLUMN IF EXISTS get_last_24h_count;

COMMIT;