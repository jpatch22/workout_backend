package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackpatchell/workout-tracker/backend/formulas"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// ErrNotFound is the service-layer "no such resource (or not yours)" signal.
var ErrNotFound = errors.New("not found")

// WorkoutService owns session lifecycle and the logging of individual sets.
type WorkoutService struct {
	workouts *tables.Workouts
	sets     *tables.Sets
	settings *SettingsService
}

// NewWorkoutService builds the workout service.
func NewWorkoutService(w *tables.Workouts, s *tables.Sets, settings *SettingsService) *WorkoutService {
	return &WorkoutService{workouts: w, sets: s, settings: settings}
}

// formula resolves the user's chosen 1RM formula, falling back to the default if
// settings cannot be read. A preferences lookup failing should not stop someone
// logging a workout, so this degrades rather than erroring.
func (s *WorkoutService) formula(ctx context.Context, userID int64) formulas.Formula {
	if s.settings == nil {
		return formulas.Default
	}
	f, err := s.settings.Formula(ctx, userID)
	if err != nil {
		return formulas.Default
	}
	return f
}

// CreateWorkout starts a new session.
func (s *WorkoutService) CreateWorkout(ctx context.Context, userID int64, name string, performedAt *time.Time, notes string) (*tables.Workout, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultWorkoutName(performedAt)
	}
	if len(name) > 120 {
		return nil, invalid("name", "must be at most 120 characters")
	}
	if performedAt != nil {
		// Reject sessions dated far in the future — almost always a typo or a
		// client clock problem, and it would skew every progress chart. The
		// grace period matters because Postgres rounds TIMESTAMPTZ to the
		// microsecond while the client sends whole seconds: without it, a user
		// opening a session and saving it without touching the date would be
		// rejected for being a few hundred nanoseconds in the "future".
		if performedAt.After(time.Now().Add(5 * time.Minute)) {
			return nil, invalid("performedAt", "cannot be in the future")
		}
	}

	w, err := s.workouts.Create(ctx, &tables.Workout{
		UserID:      userID,
		Name:        name,
		PerformedAt: derefTime(performedAt),
		Notes:       strings.TrimSpace(notes),
	})
	if err != nil {
		return nil, wrapTableErr(err, "create workout")
	}
	return w, nil
}

// GetWorkout loads one session with exercises and sets inlined.
func (s *WorkoutService) GetWorkout(ctx context.Context, userID, id int64) (*tables.Workout, error) {
	w, err := s.workouts.GetOwned(ctx, userID, id)
	if err != nil {
		return nil, wrapTableErr(err, "get workout")
	}
	return w, nil
}

// UpdateWorkout edits a session's header.
func (s *WorkoutService) UpdateWorkout(ctx context.Context, userID, id int64, name string, performedAt *time.Time, notes string) (*tables.Workout, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultWorkoutName(performedAt)
	}
	if performedAt != nil && performedAt.After(time.Now().Add(5*time.Minute)) {
		return nil, invalid("performedAt", "cannot be in the future")
	}

	w, err := s.workouts.Update(ctx, userID, id, name, performedAt, strings.TrimSpace(notes))
	if err != nil {
		return nil, wrapTableErr(err, "update workout")
	}
	return w, nil
}

// DeleteWorkout removes a session and everything logged in it.
func (s *WorkoutService) DeleteWorkout(ctx context.Context, userID, id int64) error {
	if err := s.workouts.Delete(ctx, userID, id); err != nil {
		return wrapTableErr(err, "delete workout")
	}
	return nil
}

// ListWorkouts returns session summaries, newest first.
func (s *WorkoutService) ListWorkouts(ctx context.Context, userID int64, limit int) ([]tables.WorkoutSummary, error) {
	list, err := s.workouts.ListByUser(ctx, userID, limit, s.formula(ctx, userID))
	if err != nil {
		return nil, wrapTableErr(err, "list workouts")
	}
	return list, nil
}

// AddExercise attaches an exercise to a session, validating that the user can
// actually see the exercise before writing.
func (s *WorkoutService) AddExercise(ctx context.Context, userID, workoutID, exerciseID int64, notes string) (*tables.WorkoutExercise, error) {
	if exerciseID <= 0 {
		return nil, invalid("exerciseId", "is required")
	}

	we, err := s.workouts.AddExercise(ctx, userID, workoutID, exerciseID, strings.TrimSpace(notes))
	if err != nil {
		if errors.Is(err, tables.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, wrapTableErr(err, "add exercise")
	}
	return we, nil
}

// RemoveExercise detaches an exercise from a session.
func (s *WorkoutService) RemoveExercise(ctx context.Context, userID, workoutID, workoutExerciseID int64) error {
	if err := s.workouts.RemoveExercise(ctx, userID, workoutID, workoutExerciseID); err != nil {
		return wrapTableErr(err, "remove exercise")
	}
	return nil
}

// LogSetInput is the validated, service-level shape for a new set.
type LogSetInput struct {
	Weight       float64
	Reps         int
	RPE          *float64
	IsWarmup     bool
	IsBodyweight bool
}

// LogSet validates and records one set, then re-evaluates whether it is a
// personal record. Returns the stored set with its PR flag resolved.
func (s *WorkoutService) LogSet(ctx context.Context, userID, workoutExerciseID int64, in LogSetInput) (*tables.Set, error) {
	if err := validateSet(in); err != nil {
		return nil, err
	}

	// Confirm ownership before writing so a guessed id can't be used to inject
	// sets into someone else's workout.
	if _, err := s.workouts.GetWorkoutExerciseOwned(ctx, userID, workoutExerciseID); err != nil {
		return nil, wrapTableErr(err, "log set")
	}

	set, err := s.sets.Create(ctx, &tables.Set{
		WorkoutExerciseID: workoutExerciseID,
		Weight:            in.Weight,
		Reps:              in.Reps,
		RPE:               in.RPE,
		IsWarmup:          in.IsWarmup,
		IsBodyweight:      in.IsBodyweight,
	})
	if err != nil {
		return nil, wrapTableErr(err, "log set")
	}

	// Warmups never count as PRs, so skip the comparison for them.
	if !in.IsWarmup {
		if _, err := s.sets.MarkPRIfBeater(ctx, userID, set.ID, s.formula(ctx, userID)); err != nil {
			return nil, wrapTableErr(err, "evaluate pr")
		}
		// Re-read so the client sees the resolved flag rather than the default.
		if refreshed, err := s.sets.GetOwned(ctx, userID, set.ID); err == nil {
			set = refreshed
		}
	}

	return set, nil
}

// UpdateSet edits an existing set and re-evaluates its PR flag.
func (s *WorkoutService) UpdateSet(ctx context.Context, userID, setID int64, in LogSetInput) (*tables.Set, error) {
	if err := validateSet(in); err != nil {
		return nil, err
	}

	existing, err := s.sets.GetOwned(ctx, userID, setID)
	if err != nil {
		return nil, wrapTableErr(err, "update set")
	}

	updated, err := s.sets.Update(ctx, setID, &tables.Set{
		Weight:       in.Weight,
		Reps:         in.Reps,
		RPE:          in.RPE,
		IsWarmup:     in.IsWarmup,
		IsBodyweight: in.IsBodyweight,
		IsPR:         existing.IsPR,
	})
	if err != nil {
		return nil, wrapTableErr(err, "update set")
	}

	if !in.IsWarmup {
		if _, err := s.sets.MarkPRIfBeater(ctx, userID, updated.ID, s.formula(ctx, userID)); err != nil {
			return nil, wrapTableErr(err, "evaluate pr")
		}
		if refreshed, err := s.sets.GetOwned(ctx, userID, updated.ID); err == nil {
			updated = refreshed
		}
	}

	return updated, nil
}

// DeleteSet removes a set.
func (s *WorkoutService) DeleteSet(ctx context.Context, userID, setID int64) error {
	if _, err := s.sets.GetOwned(ctx, userID, setID); err != nil {
		return wrapTableErr(err, "delete set")
	}
	if err := s.sets.Delete(ctx, setID); err != nil {
		return wrapTableErr(err, "delete set")
	}
	return nil
}

// validateSet enforces the physical constraints a set must satisfy.
func validateSet(in LogSetInput) error {
	if in.Weight < 0 {
		return invalid("weight", "cannot be negative")
	}
	// A bodyweight set legitimately carries weight 0, so only cap the ceiling.
	if in.Weight > 1500 {
		return invalid("weight", "must be at most 1500")
	}
	if in.Reps <= 0 {
		return invalid("reps", "must be at least 1")
	}
	if in.Reps > 1000 {
		return invalid("reps", "must be at most 1000")
	}
	if in.RPE != nil {
		if *in.RPE < 0 || *in.RPE > 10 {
			return invalid("rpe", "must be between 0 and 10")
		}
	}
	if in.Weight == 0 && !in.IsBodyweight && !in.IsWarmup {
		return invalid("weight", "must be greater than 0 unless the set is bodyweight or a warmup")
	}
	return nil
}

func defaultWorkoutName(at *time.Time) string {
	t := time.Now()
	if at != nil {
		t = *at
	}
	return t.Format("Jan 2, 2006") + " Workout"
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Now()
	}
	return *t
}
