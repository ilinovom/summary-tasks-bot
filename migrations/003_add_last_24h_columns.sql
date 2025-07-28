ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS last_get_last_24h BIGINT,
    ADD COLUMN IF NOT EXISTS get_last_24h_count INTEGER;
