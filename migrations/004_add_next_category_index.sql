ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS next_category_index INTEGER;
