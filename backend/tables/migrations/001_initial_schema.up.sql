-- 001_initial_schema.up.sql
-- Normalized schema for workout tracking.
--
--   users
--     └── workouts
--           └── workout_exercises
--                 └── sets
--
--   exercises (catalog, optionally user-owned)
--     └── workout_exercises
--
-- Derived values (volume, est. 1RM, PRs) are COMPUTED at query time,
-- not stored redundantly.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    display_name  TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- exercises — the catalog. is_custom lets a user create their own; when set,
-- user_id scopes the exercise to that user (NULL = shared/global catalog).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS exercises (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      REFERENCES users (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL,
    muscle_group TEXT,
    equipment   TEXT,
    is_custom   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT exercises_custom_needs_owner
        CHECK (is_custom = FALSE OR user_id IS NOT NULL)
);

CREATE UNIQUE INDEX IF NOT EXISTS exercises_global_slug_key
    ON exercises (slug) WHERE user_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS exercises_user_slug_key
    ON exercises (user_id, slug) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS exercises_user_id_idx ON exercises (user_id);

-- ---------------------------------------------------------------------------
-- workouts — one training session.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workouts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL DEFAULT '',
    performed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS workouts_user_performed_idx
    ON workouts (user_id, performed_at DESC);

-- ---------------------------------------------------------------------------
-- workout_exercises — exercises performed within a workout, in order.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workout_exercises (
    id          BIGSERIAL PRIMARY KEY,
    workout_id  BIGINT NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
    exercise_id BIGINT NOT NULL REFERENCES exercises (id) ON DELETE RESTRICT,
    position    INTEGER NOT NULL DEFAULT 0,
    notes       TEXT   NOT NULL DEFAULT '',

    UNIQUE (workout_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS workout_exercises_workout_idx
    ON workout_exercises (workout_id, position);

-- ---------------------------------------------------------------------------
-- sets — the atomic logged data. Everything interesting is derived from this.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sets (
    id                BIGSERIAL PRIMARY KEY,
    workout_exercise_id BIGINT NOT NULL REFERENCES workout_exercises (id) ON DELETE CASCADE,
    weight            NUMERIC(7,2) NOT NULL,
    reps              INTEGER      NOT NULL,
    rpe               NUMERIC(3,1),
    is_warmup         BOOLEAN      NOT NULL DEFAULT FALSE,
    is_bodyweight     BOOLEAN      NOT NULL DEFAULT FALSE,
    is_pr             BOOLEAN      NOT NULL DEFAULT FALSE,
    logged_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT sets_weight_non_negative  CHECK (weight  >= 0),
    CONSTRAINT sets_reps_positive       CHECK (reps    > 0),
    CONSTRAINT sets_rpe_range           CHECK (rpe IS NULL OR (rpe >= 0 AND rpe <= 10))
);

CREATE INDEX IF NOT EXISTS sets_workout_exercise_idx
    ON sets (workout_exercise_id, logged_at);

-- ---------------------------------------------------------------------------
-- updated_at maintenance
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_set_updated_at ON users;
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS workouts_set_updated_at ON workouts;
CREATE TRIGGER workouts_set_updated_at
    BEFORE UPDATE ON workouts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();