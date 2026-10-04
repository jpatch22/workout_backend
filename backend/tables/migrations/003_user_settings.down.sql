-- 003_user_settings.down.sql
DROP TRIGGER IF EXISTS user_settings_set_updated_at ON user_settings;
DROP TABLE IF EXISTS user_settings;