-- 003_user_settings.up.sql
-- Per-user preferences, most importantly which estimated-1RM formula to use.
--
-- preferences is JSONB rather than a column per setting so that adding a new
-- preference later does not require a migration, and so the API can return the
-- whole settings object in one read. The CHECK keeps the stored document to
-- the shape the app actually understands.

CREATE TABLE IF NOT EXISTS user_settings (
    user_id     BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    preferences JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_settings_preferences_object
        CHECK (jsonb_typeof(preferences) = 'object')
);

DROP TRIGGER IF EXISTS user_settings_set_updated_at ON user_settings;
CREATE TRIGGER user_settings_set_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed a row for every existing user so a read never has to distinguish
-- "no settings yet" from "settings are empty".
INSERT INTO user_settings (user_id, preferences)
SELECT id, '{}'::jsonb FROM users
ON CONFLICT (user_id) DO NOTHING;