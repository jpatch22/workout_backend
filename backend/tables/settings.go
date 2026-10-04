package tables

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Preferences is the JSONB document stored per user.
//
// It is a map rather than a fixed struct so a new preference can be added
// without a migration, but every read goes through services.Settings which fills
// in defaults, so callers always receive a fully-populated object.
type Preferences map[string]any

// Settings is the settings DAO. One row per user.
type Settings struct{ db *DB }

// NewSettings builds the settings DAO.
func NewSettings(db *DB) *Settings { return &Settings{db: db} }

// Get returns the raw preferences for a user, creating an empty row on first
// access so callers never have to handle a missing row.
func (s *Settings) Get(ctx context.Context, userID int64) (Preferences, error) {
	var raw []byte
	err := s.db.Pool.QueryRow(ctx,
		`SELECT preferences FROM user_settings WHERE user_id = $1`, userID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.create(ctx, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	prefs := Preferences{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &prefs); err != nil {
			return nil, fmt.Errorf("decode preferences for user %d: %w", userID, err)
		}
	}
	if prefs == nil {
		prefs = Preferences{}
	}
	return prefs, nil
}

// Update replaces the preferences document for a user.
func (s *Settings) Update(ctx context.Context, userID int64, prefs Preferences) error {
	raw, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}

	_, err = s.db.Pool.Exec(ctx,
		`UPDATE user_settings SET preferences = $2 WHERE user_id = $1`, userID, raw)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}

// EnsureExists creates the settings row if it is missing. Called at registration
// so the row is present from the start; also safe to call repeatedly.
func (s *Settings) EnsureExists(ctx context.Context, userID int64) error {
	_, err := s.db.Pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, preferences) VALUES ($1, '{}'::jsonb)
		 ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("create settings: %w", err)
	}
	return nil
}

// create inserts an empty settings row lazily, for users registered before this
// table existed.
func (s *Settings) create(ctx context.Context, userID int64) (Preferences, error) {
	if err := s.EnsureExists(ctx, userID); err != nil {
		return nil, err
	}
	return Preferences{}, nil
}
