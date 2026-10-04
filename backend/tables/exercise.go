package tables

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Exercise is a catalog entry. UserID is nil for the shared global catalog and
// set for user-created exercises.
type Exercise struct {
	ID          int64     `json:"id"`
	UserID      *int64    `json:"userId,omitempty"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	MuscleGroup string    `json:"muscleGroup,omitempty"`
	Equipment   string    `json:"equipment,omitempty"`
	IsCustom    bool      `json:"isCustom"`
	CreatedAt   time.Time `json:"-"`
}

// exerciseCols coerces the nullable text columns to ” so they scan cleanly into
// the non-nullable Go fields above. The empty string is what the UI already
// treats as "not specified", so no information is lost.
const exerciseCols = `id, user_id, name, slug,
	COALESCE(muscle_group, ''), COALESCE(equipment, ''), is_custom, created_at`

// Exercises is the data-access object for the exercises table.
type Exercises struct{ db *DB }

// NewExercises builds the exercises DAO.
func NewExercises(db *DB) *Exercises { return &Exercises{db: db} }

// Create inserts a global (userID nil) or user-owned exercise, deriving a
// unique slug within the same scope.
func (e *Exercises) Create(ctx context.Context, ex *Exercise) (*Exercise, error) {
	name := strings.TrimSpace(ex.Name)
	if name == "" {
		return nil, errors.New("exercise name is required")
	}

	var out Exercise
	err := e.db.inTx(ctx, func(tx pgx.Tx) error {
		slug, err := uniqueSlug(ctx, tx, slugify(name), ex.UserID)
		if err != nil {
			return fmt.Errorf("slugify: %w", err)
		}
		isCustom := ex.UserID != nil

		return tx.QueryRow(ctx, `
			INSERT INTO exercises (user_id, name, slug, muscle_group, equipment, is_custom)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+exerciseCols,
			ex.UserID, name, slug, ex.MuscleGroup, ex.Equipment, isCustom,
		).Scan(&out.ID, &out.UserID, &out.Name, &out.Slug, &out.MuscleGroup,
			&out.Equipment, &out.IsCustom, &out.CreatedAt)
	})
	if err != nil {
		return nil, fmt.Errorf("insert exercise: %w", err)
	}
	return &out, nil
}

// ListByUser returns the exercises visible to a user: the global catalog plus
// their own custom exercises, alphabetically.
func (e *Exercises) ListByUser(ctx context.Context, userID int64) ([]Exercise, error) {
	rows, err := e.db.Pool.Query(ctx, `
		SELECT `+exerciseCols+`
		FROM exercises
		WHERE user_id IS NULL OR user_id = $1
		ORDER BY name ASC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list exercises: %w", err)
	}
	return scanExercises(rows)
}

// GetByID loads one exercise if it is visible to userID.
func (e *Exercises) GetByID(ctx context.Context, userID, id int64) (*Exercise, error) {
	var out Exercise
	err := e.db.Pool.QueryRow(ctx, `
		SELECT `+exerciseCols+`
		FROM exercises
		WHERE id = $1 AND (user_id IS NULL OR user_id = $2)`,
		id, userID,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.Slug, &out.MuscleGroup,
		&out.Equipment, &out.IsCustom, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get exercise: %w", err)
	}
	return &out, nil
}

// Delete removes a user-owned exercise. Global exercises cannot be deleted.
func (e *Exercises) Delete(ctx context.Context, userID, id int64) error {
	tag, err := e.db.Pool.Exec(ctx,
		`DELETE FROM exercises WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete exercise: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanExercises(rows pgx.Rows) ([]Exercise, error) {
	defer rows.Close()

	out := make([]Exercise, 0)
	for rows.Next() {
		var ex Exercise
		if err := rows.Scan(&ex.ID, &ex.UserID, &ex.Name, &ex.Slug, &ex.MuscleGroup,
			&ex.Equipment, &ex.IsCustom, &ex.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan exercise: %w", err)
		}
		out = append(out, ex)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exercises: %w", err)
	}
	return out, nil
}
