package tables

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// WorkoutExercise is one exercise performed inside a workout, with its sets.
type WorkoutExercise struct {
	ID         int64    `json:"id"`
	WorkoutID  int64    `json:"workoutId"`
	ExerciseID int64    `json:"exerciseId"`
	Exercise   Exercise `json:"exercise"`
	Position   int      `json:"position"`
	Notes      string   `json:"notes,omitempty"`
	Sets       []Set    `json:"sets"`
}

// Workout is a training session with its exercises and their sets inlined,
// which is exactly the shape the Workout screen needs.
type Workout struct {
	ID          int64             `json:"id"`
	UserID      int64             `json:"userId"`
	Name        string            `json:"name"`
	PerformedAt time.Time         `json:"performedAt"`
	Notes       string            `json:"notes,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Exercises   []WorkoutExercise `json:"exercises"`
}

// WorkoutSummary is the lightweight list shape used by History and Dashboard.
type WorkoutSummary struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	PerformedAt     time.Time `json:"performedAt"`
	ExerciseCount   int       `json:"exerciseCount"`
	SetCount        int       `json:"setCount"`
	TotalVolume     float64   `json:"totalVolume"`
	TopEstimated1RM float64   `json:"topEstimated1Rm"`
}

// Workouts is the data-access object for workouts and workout_exercises.
type Workouts struct{ db *DB }

// NewWorkouts builds the workouts DAO.
func NewWorkouts(db *DB) *Workouts { return &Workouts{db: db} }

const workoutCols = `w.id, w.user_id, w.name, w.performed_at, w.notes, w.created_at, w.updated_at`

// Create inserts a workout.
func (w *Workouts) Create(ctx context.Context, workout *Workout) (*Workout, error) {
	var out Workout
	err := w.db.Pool.QueryRow(ctx, `
		INSERT INTO workouts (user_id, name, performed_at, notes)
		VALUES ($1, $2, COALESCE($3, now()), $4)
		RETURNING id, user_id, name, performed_at, notes, created_at, updated_at`,
		workout.UserID, workout.Name, workout.PerformedAt, workout.Notes,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.PerformedAt, &out.Notes,
		&out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workout: %w", err)
	}
	out.Exercises = []WorkoutExercise{}
	return &out, nil
}

// Update changes a workout's header fields.
func (w *Workouts) Update(ctx context.Context, userID, id int64, name string, performedAt *time.Time, notes string) (*Workout, error) {
	var out Workout
	err := w.db.Pool.QueryRow(ctx, `
		UPDATE workouts
		SET name = $3, performed_at = COALESCE($4, performed_at), notes = $5
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, name, performed_at, notes, created_at, updated_at`,
		id, userID, name, performedAt, notes,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.PerformedAt, &out.Notes,
		&out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update workout: %w", err)
	}
	return &out, nil
}

// Delete removes a workout; exercises and sets cascade.
func (w *Workouts) Delete(ctx context.Context, userID, id int64) error {
	tag, err := w.db.Pool.Exec(ctx, `DELETE FROM workouts WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOwned loads one workout owned by userID with exercises and sets inlined.
func (w *Workouts) GetOwned(ctx context.Context, userID, id int64) (*Workout, error) {
	var out Workout
	err := w.db.Pool.QueryRow(ctx, `
		SELECT `+workoutCols+`
		FROM workouts w
		WHERE w.id = $1 AND w.user_id = $2`, id, userID,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.PerformedAt, &out.Notes,
		&out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workout: %w", err)
	}

	exercises, err := w.loadExercises(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	// A workout with no exercises has no map entry, so the lookup yields a nil
	// slice — which encodes as JSON `null` rather than `[]`. Clients that call
	// .map/.flatMap on this would crash on an empty session, so always emit an
	// array.
	if exercises[out.ID] == nil {
		out.Exercises = []WorkoutExercise{}
	} else {
		out.Exercises = exercises[out.ID]
	}
	return &out, nil
}

// ListByUser returns workout summaries, newest first, optionally limited.
func (w *Workouts) ListByUser(ctx context.Context, userID int64, limit int) ([]WorkoutSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := w.db.Pool.Query(ctx, `
		SELECT w.id, w.name, w.performed_at,
		       COUNT(DISTINCT we.id)                      AS exercise_count,
		       COUNT(s.id)                                AS set_count,
		       COALESCE(SUM(s.weight * s.reps), 0)        AS total_volume,
		       COALESCE(MAX(s.weight * (1 + s.reps::float8 / 30)), 0) AS top_est_1rm
		FROM workouts w
		LEFT JOIN workout_exercises we ON we.workout_id = w.id
		LEFT JOIN sets s ON s.workout_exercise_id = we.id AND NOT s.is_warmup
		WHERE w.user_id = $1
		GROUP BY w.id, w.name, w.performed_at
		ORDER BY w.performed_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()

	out := make([]WorkoutSummary, 0)
	for rows.Next() {
		var s WorkoutSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.PerformedAt, &s.ExerciseCount,
			&s.SetCount, &s.TotalVolume, &s.TopEstimated1RM); err != nil {
			return nil, fmt.Errorf("scan workout summary: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workouts: %w", err)
	}
	return out, nil
}

// AddExercise attaches a catalog exercise to a workout. Re-adding an exercise
// already in the workout is a no-op that returns the existing row, so a
// double-tap in the UI can't create duplicates.
func (w *Workouts) AddExercise(ctx context.Context, userID, workoutID, exerciseID int64, notes string) (*WorkoutExercise, error) {
	var out WorkoutExercise

	err := w.db.inTx(ctx, func(tx pgx.Tx) error {
		// Confirm the workout belongs to this user before touching it.
		var owns bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM workouts WHERE id = $1 AND user_id = $2)`,
			workoutID, userID).Scan(&owns); err != nil {
			return err
		}
		if !owns {
			return ErrNotFound
		}

		// Confirm the exercise is visible to this user.
		var visible bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM exercises WHERE id = $1 AND (user_id IS NULL OR user_id = $2))`,
			exerciseID, userID).Scan(&visible); err != nil {
			return err
		}
		if !visible {
			return ErrNotFound
		}

		// Append at the end unless the exercise is already present.
		err := tx.QueryRow(ctx, `
			INSERT INTO workout_exercises (workout_id, exercise_id, position, notes)
			VALUES ($1, $2,
			        COALESCE((SELECT MAX(position) + 1 FROM workout_exercises WHERE workout_id = $1), 0),
			        $3)
			ON CONFLICT (workout_id, exercise_id) DO UPDATE SET notes = workout_exercises.notes
			RETURNING id, workout_id, exercise_id, position, notes`,
			workoutID, exerciseID, notes,
		).Scan(&out.ID, &out.WorkoutID, &out.ExerciseID, &out.Position, &out.Notes)
		if err != nil {
			return fmt.Errorf("add workout exercise: %w", err)
		}

		return tx.QueryRow(ctx,
			`SELECT `+exerciseCols+` FROM exercises WHERE id = $1`, exerciseID,
		).Scan(&out.Exercise.ID, &out.Exercise.UserID, &out.Exercise.Name, &out.Exercise.Slug,
			&out.Exercise.MuscleGroup, &out.Exercise.Equipment, &out.Exercise.IsCustom, &out.Exercise.CreatedAt)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("add exercise to workout: %w", err)
	}

	out.Sets = []Set{}
	return &out, nil
}

// RemoveExercise detaches an exercise from a workout; its sets cascade.
func (w *Workouts) RemoveExercise(ctx context.Context, userID, workoutID, workoutExerciseID int64) error {
	tag, err := w.db.Pool.Exec(ctx, `
		DELETE FROM workout_exercises we
		USING workouts w
		WHERE we.id = $1 AND we.workout_id = $2 AND w.id = we.workout_id AND w.user_id = $3`,
		workoutExerciseID, workoutID, userID)
	if err != nil {
		return fmt.Errorf("remove workout exercise: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetWorkoutExerciseOwned loads one workout_exercise if its workout is owned by userID.
func (w *Workouts) GetWorkoutExerciseOwned(ctx context.Context, userID, workoutExerciseID int64) (*WorkoutExercise, error) {
	var out WorkoutExercise
	err := w.db.Pool.QueryRow(ctx, `
		SELECT we.id, we.workout_id, we.exercise_id, we.position, we.notes,
		       e.id, e.user_id, e.name, e.slug,
		       COALESCE(e.muscle_group, ''), COALESCE(e.equipment, ''), e.is_custom, e.created_at
		FROM workout_exercises we
		JOIN workouts w  ON w.id = we.workout_id
		JOIN exercises e ON e.id = we.exercise_id
		WHERE we.id = $1 AND w.user_id = $2`, workoutExerciseID, userID,
	).Scan(&out.ID, &out.WorkoutID, &out.ExerciseID, &out.Position, &out.Notes,
		&out.Exercise.ID, &out.Exercise.UserID, &out.Exercise.Name, &out.Exercise.Slug,
		&out.Exercise.MuscleGroup, &out.Exercise.Equipment, &out.Exercise.IsCustom, &out.Exercise.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workout exercise: %w", err)
	}
	out.Sets = []Set{}
	return &out, nil
}

// loadExercises fetches workout_exercises (with catalog details and sets) for
// the given workouts, indexed by workout ID. One round trip for the list view.
func (w *Workouts) loadExercises(ctx context.Context, workoutIDs []int64) (map[int64][]WorkoutExercise, error) {
	indexed := make(map[int64][]WorkoutExercise, len(workoutIDs))
	if len(workoutIDs) == 0 {
		return indexed, nil
	}

	rows, err := w.db.Pool.Query(ctx, `
		SELECT we.workout_id, we.id, we.exercise_id, we.position, we.notes,
		       e.id, e.user_id, e.name, e.slug,
		       COALESCE(e.muscle_group, ''), COALESCE(e.equipment, ''), e.is_custom, e.created_at
		FROM workout_exercises we
		JOIN exercises e ON e.id = we.exercise_id
		WHERE we.workout_id = ANY($1)
		ORDER BY we.workout_id, we.position, we.id`, workoutIDs)
	if err != nil {
		return nil, fmt.Errorf("load workout exercises: %w", err)
	}

	byID := make(map[int64]*WorkoutExercise)
	order := make([]int64, 0)
	for rows.Next() {
		var workoutID int64
		var we WorkoutExercise
		if err := rows.Scan(&workoutID, &we.ID, &we.ExerciseID, &we.Position, &we.Notes,
			&we.Exercise.ID, &we.Exercise.UserID, &we.Exercise.Name, &we.Exercise.Slug,
			&we.Exercise.MuscleGroup, &we.Exercise.Equipment, &we.Exercise.IsCustom, &we.Exercise.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan workout exercise: %w", err)
		}
		we.Sets = []Set{}
		byID[we.ID] = &we
		order = append(order, we.ID)
		indexed[workoutID] = append(indexed[workoutID], we)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workout exercises: %w", err)
	}

	// Attach sets in one query rather than per exercise.
	if len(order) > 0 {
		setRows, err := w.db.Pool.Query(ctx, `
			SELECT workout_exercise_id, id, weight, reps, rpe, is_warmup, is_bodyweight, is_pr, logged_at
			FROM sets
			WHERE workout_exercise_id = ANY($1)
			ORDER BY workout_exercise_id, logged_at, id`, order)
		if err != nil {
			return nil, fmt.Errorf("load sets: %w", err)
		}
		defer setRows.Close()

		for setRows.Next() {
			var weID int64
			var s Set
			if err := setRows.Scan(&weID, &s.ID, &s.Weight, &s.Reps, &s.RPE,
				&s.IsWarmup, &s.IsBodyweight, &s.IsPR, &s.LoggedAt); err != nil {
				return nil, fmt.Errorf("scan set: %w", err)
			}
			if we, ok := byID[weID]; ok {
				we.Sets = append(we.Sets, s)
			}
		}
		if err := setRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate sets: %w", err)
		}

		// Copy the (now set-populated) structs back into the indexed slice.
		for _, slice := range indexed {
			for i := range slice {
				if we, ok := byID[slice[i].ID]; ok {
					slice[i] = *we
				}
			}
		}
	}

	return indexed, nil
}
