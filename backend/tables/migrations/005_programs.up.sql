-- Programs are user-owned plans. Their ordered weeks, sessions, prescriptions,
-- and current run progress are stored as one JSON document per program.
CREATE TABLE IF NOT EXISTS programs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    weeks       JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS programs_user_created_idx
    ON programs (user_id, created_at DESC);

DROP TRIGGER IF EXISTS programs_set_updated_at ON programs;
CREATE TRIGGER programs_set_updated_at
    BEFORE UPDATE ON programs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
