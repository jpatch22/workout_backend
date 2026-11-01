-- 004_set_notes.up.sql
-- Optional free-text note attached to an individual logged set (e.g. "paused",
-- "felt slow", "used straps"). Empty string means no note.
ALTER TABLE sets ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '';
