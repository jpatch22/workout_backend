package services

import (
	"context"
	"errors"
	"strings"

	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// ExerciseService manages the exercise catalog and user-created movements.
type ExerciseService struct {
	exercises *tables.Exercises
}

// NewExerciseService builds the exercise service.
func NewExerciseService(ex *tables.Exercises) *ExerciseService {
	return &ExerciseService{exercises: ex}
}

// List returns every exercise visible to the user (global catalog + own).
func (s *ExerciseService) List(ctx context.Context, userID int64) ([]tables.Exercise, error) {
	list, err := s.exercises.ListByUser(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "list exercises")
	}
	return list, nil
}

// Get loads one exercise the user can see.
func (s *ExerciseService) Get(ctx context.Context, userID, id int64) (*tables.Exercise, error) {
	ex, err := s.exercises.GetByID(ctx, userID, id)
	if err != nil {
		return nil, wrapTableErr(err, "get exercise")
	}
	return ex, nil
}

// Create adds a user-owned exercise to the catalog.
func (s *ExerciseService) Create(ctx context.Context, userID int64, name, muscleGroup, equipment string) (*tables.Exercise, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalid("name", "is required")
	}
	if len(name) > 120 {
		return nil, invalid("name", "must be at most 120 characters")
	}

	ex, err := s.exercises.Create(ctx, &tables.Exercise{
		UserID:      &userID,
		Name:        name,
		MuscleGroup: strings.TrimSpace(muscleGroup),
		Equipment:   strings.TrimSpace(equipment),
	})
	if err != nil {
		return nil, wrapTableErr(err, "create exercise")
	}
	return ex, nil
}

// Delete removes a user-owned exercise. Global catalog entries are immutable.
func (s *ExerciseService) Delete(ctx context.Context, userID, id int64) error {
	if err := s.exercises.Delete(ctx, userID, id); err != nil {
		return wrapTableErr(err, "delete exercise")
	}
	return nil
}

// wrapTableErr normalizes "not found" from the tables layer and adds context to
// everything else, so services have a single error vocabulary to reason about.
func wrapTableErr(err error, context string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, tables.ErrNotFound) {
		return ErrNotFound
	}
	return errors.New(context + ": " + err.Error())
}
