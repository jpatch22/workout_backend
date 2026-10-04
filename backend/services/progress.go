package services

import (
	"context"
	"strings"
	"time"

	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// ProgressService computes every derived metric the UI displays.
//
// Nothing here is stored in the database: volume, estimated 1RM, trends, PR
// counts and streaks are all derived from the sets table on read. That means a
// change to the formulas takes effect immediately for all historical data and
// there is never a backfill to write.
type ProgressService struct {
	progress *tables.Progress
	workouts *tables.Workouts
}

// NewProgressService builds the progress service.
func NewProgressService(p *tables.Progress, w *tables.Workouts) *ProgressService {
	return &ProgressService{progress: p, workouts: w}
}

// Dashboard is everything the home screen needs, in one payload.
type Dashboard struct {
	Stats            tables.DashboardStats   `json:"stats"`
	RecentWorkouts   []tables.WorkoutSummary `json:"recentWorkouts"`
	TrackedExercises []tables.RecentExercise `json:"trackedExercises"`
}

// Dashboard assembles the landing screen payload.
func (s *ProgressService) Dashboard(ctx context.Context, userID int64) (*Dashboard, error) {
	stats, err := s.progress.Overview(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "dashboard stats")
	}

	recent, err := s.workouts.ListByUser(ctx, userID, 5)
	if err != nil {
		return nil, wrapTableErr(err, "dashboard workouts")
	}

	tracked, err := s.progress.ListTracked(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "dashboard exercises")
	}
	if len(tracked) > 10 {
		tracked = tracked[:10]
	}

	return &Dashboard{Stats: *stats, RecentWorkouts: recent, TrackedExercises: tracked}, nil
}

// Overview returns just the headline aggregates.
func (s *ProgressService) Overview(ctx context.Context, userID int64) (*tables.DashboardStats, error) {
	stats, err := s.progress.Overview(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "overview")
	}
	return stats, nil
}

// TrackedExercises lists every exercise the user has trained.
func (s *ProgressService) TrackedExercises(ctx context.Context, userID int64) ([]tables.RecentExercise, error) {
	list, err := s.progress.ListTracked(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "list tracked exercises")
	}
	return list, nil
}

// ExerciseProgress returns the full time series for one exercise, addressed by
// slug (stable, URL-friendly, and what the progress route carries).
//
// since optionally bounds the window; nil means all history.
func (s *ProgressService) ExerciseProgress(ctx context.Context, userID int64, slug string, since *time.Time) (*tables.ExerciseProgress, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, invalid("slug", "is required")
	}

	p, err := s.progress.ForExercise(ctx, userID, slug, since)
	if err != nil {
		return nil, wrapTableErr(err, "exercise progress")
	}
	return p, nil
}

// Estimated1RM is the Epley formula: weight * (1 + reps/30).
//
// It is defined once here and mirrored in the sets.PR SQL so both agree; the
// frontend uses the same formula for its optimistic per-set display.
func Estimated1RM(weight float64, reps int) float64 {
	if reps <= 0 {
		return weight
	}
	return weight * (1 + float64(reps)/30)
}

// SetVolume is the tonnage of one set: weight * reps.
func SetVolume(weight float64, reps int) float64 { return weight * float64(reps) }
