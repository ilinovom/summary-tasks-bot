ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS username TEXT;
