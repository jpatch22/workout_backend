package tables

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Set is one logged attempt: the atomic unit of all training data. Everything
// the app reports (volume, est. 1RM, PRs) is derived from these rows.
type Set struct {
	ID                int64     `json:"id"`
	WorkoutExerciseID int64     `json:"workoutExerciseId"`
	Weight            float64   `json:"weight"`
	Reps              int       `json:"reps"`
	RPE               *float64  `json:"rpe,omitempty"`
	IsWarmup          bool      `json:"isWarmup"`
	IsBodyweight      bool      `json:"isBodyweight"`
	IsPR              bool      `json:"isPr"`
	LoggedAt          time.Time `json:"loggedAt"`
}

// Sets is the data-access object for the sets table.
type Sets struct{ db *DB }

// NewSets builds the sets DAO.
func NewSets(db *DB) *Sets { return &Sets{db: db} }

const setCols = `id, workout_exercise_id, weight, reps, rpe, is_warmup, is_bodyweight, is_pr, logged_at`

// Create inserts a set and returns it with its generated ID.
func (s *Sets) Create(ctx context.Context, set *Set) (*Set, error) {
	var out Set
	err := s.db.Pool.QueryRow(ctx, `
		INSERT INTO sets (workout_exercise_id, weight, reps, rpe, is_warmup, is_bodyweight, is_pr)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+setCols,
		set.WorkoutExerciseID, set.Weight, set.Reps, set.RPE,
		set.IsWarmup, set.IsBodyweight, set.IsPR,
	).Scan(&out.ID, &out.WorkoutExerciseID, &out.Weight, &out.Reps, &out.RPE,
		&out.IsWarmup, &out.IsBodyweight, &out.IsPR, &out.LoggedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, fmt.Errorf("workout exercise does not exist: %w", err)
		}
		return nil, fmt.Errorf("insert set: %w", err)
	}
	return &out, nil
}

// Update replaces the mutable fields of a set.
func (s *Sets) Update(ctx context.Context, id int64, set *Set) (*Set, error) {
	var out Set
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE sets
		SET weight = $2, reps = $3, rpe = $4,
		    is_warmup = $5, is_bodyweight = $6, is_pr = $7
		WHERE id = $1
		RETURNING `+setCols,
		id, set.Weight, set.Reps, set.RPE, set.IsWarmup, set.IsBodyweight, set.IsPR,
	).Scan(&out.ID, &out.WorkoutExerciseID, &out.Weight, &out.Reps, &out.RPE,
		&out.IsWarmup, &out.IsBodyweight, &out.IsPR, &out.LoggedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update set: %w", err)
	}
	return &out, nil
}

// Delete removes a set.
func (s *Sets) Delete(ctx context.Context, id int64) error {
	tag, err := s.db.Pool.Exec(ctx, `DELETE FROM sets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByWorkoutExercise returns the sets for one exercise within a workout,
// oldest first so they read in the order they were performed.
func (s *Sets) ListByWorkoutExercise(ctx context.Context, workoutExerciseID int64) ([]Set, error) {
	rows, err := s.db.Pool.Query(ctx,
		`SELECT `+setCols+` FROM sets WHERE workout_exercise_id = $1 ORDER BY logged_at ASC, id ASC`,
		workoutExerciseID)
	if err != nil {
		return nil, fmt.Errorf("list sets: %w", err)
	}
	return scanSets(rows)
}

// GetOwned loads a set only if it belongs to a workout owned by userID.
// This is the ownership check that stops one user editing another's data by id.
func (s *Sets) GetOwned(ctx context.Context, userID, setID int64) (*Set, error) {
	var out Set
	err := s.db.Pool.QueryRow(ctx, `
		SELECT s.id, s.workout_exercise_id, s.weight, s.reps, s.rpe,
		       s.is_warmup, s.is_bodyweight, s.is_pr, s.logged_at
		FROM sets s
		JOIN workout_exercises we ON we.id = s.workout_exercise_id
		JOIN workouts w          ON w.id  = we.workout_id
		WHERE s.id = $1 AND w.user_id = $2`,
		setID, userID,
	).Scan(&out.ID, &out.WorkoutExerciseID, &out.Weight, &out.Reps, &out.RPE,
		&out.IsWarmup, &out.IsBodyweight, &out.IsPR, &out.LoggedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owned set: %w", err)
	}
	return &out, nil
}

// MarkPRIfBeater flags a set as a personal record when its estimated 1RM beats
// every previous non-warmup set for that exercise. Computed in SQL against the
// full history so it is correct even after deleting old workouts.
func (s *Sets) MarkPRIfBeater(ctx context.Context, userID, setID int64) (bool, error) {
	var isPR bool
	err := s.db.Pool.QueryRow(ctx, `
		WITH target AS (
			SELECT s.id, s.weight, s.reps, e.id AS exercise_id
			FROM sets s
			JOIN workout_exercises we ON we.id = s.workout_exercise_id
			JOIN workouts w          ON w.id  = we.workout_id
			JOIN exercises e         ON e.id  = we.exercise_id
			WHERE s.id = $1 AND w.user_id = $2
		),
		est AS (
			-- Epley estimated 1RM: weight * (1 + reps/30)
			SELECT t.id,
			       (t.weight * (1 + t.reps::float8 / 30)) AS est_1rm,
			       COALESCE((
			           SELECT MAX(s2.weight * (1 + s2.reps::float8 / 30))
			           FROM sets s2
			           JOIN workout_exercises we2 ON we2.id = s2.workout_exercise_id
			           JOIN workouts w2          ON w2.id  = we2.workout_id
			           WHERE we2.exercise_id = t.exercise_id
			             AND w2.user_id = $2
			             AND s2.id <> t.id
			             AND NOT s2.is_warmup
			       ), 0) AS prior_best
			FROM target t
		)
		UPDATE sets
		SET is_pr = (est.est_1rm > est.prior_best)
		FROM est
		WHERE sets.id = est.id
		RETURNING sets.is_pr`, setID, userID).Scan(&isPR)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("mark pr: %w", err)
	}
	return isPR, nil
}

func scanSets(rows pgx.Rows) ([]Set, error) {
	defer rows.Close()

	out := make([]Set, 0)
	for rows.Next() {
		var s Set
		if err := rows.Scan(&s.ID, &s.WorkoutExerciseID, &s.Weight, &s.Reps, &s.RPE,
			&s.IsWarmup, &s.IsBodyweight, &s.IsPR, &s.LoggedAt); err != nil {
			return nil, fmt.Errorf("scan set: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sets: %w", err)
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
