package tables

import (
	"context"
	"fmt"
	"time"

	"github.com/jackpatchell/workout-tracker/backend/formulas"
)

// ProgressPoint is one (date, performance) sample for an exercise over time.
// All fields are computed in SQL from the sets table — nothing is denormalized.
type ProgressPoint struct {
	Date         time.Time `json:"date"`
	WorkoutID    int64     `json:"workoutId,omitempty"`
	BestWeight   float64   `json:"bestWeight"`
	BestReps     int       `json:"bestReps"`
	Estimated1RM float64   `json:"estimated1Rm"`
	TotalVolume  float64   `json:"totalVolume"`
	SetCount     int       `json:"setCount"`
}

// ExerciseProgress is the full time series plus headline stats for one exercise.
type ExerciseProgress struct {
	ExerciseID        int64           `json:"exerciseId"`
	Slug              string          `json:"slug"`
	Name              string          `json:"name"`
	MuscleGroup       string          `json:"muscleGroup,omitempty"`
	Points            []ProgressPoint `json:"points"`
	CurrentEst1RM     float64         `json:"currentEstimated1Rm"`
	PreviousEst1RM    float64         `json:"previousEstimated1Rm"`
	BestEst1RM        float64         `json:"bestEstimated1Rm"`
	BestEst1RMDate    *time.Time      `json:"bestEstimated1RmDate,omitempty"`
	PersonalRecords   int             `json:"personalRecords"`
	TotalSessions     int             `json:"totalSessions"`
	CurrentBestWeight float64         `json:"currentBestWeight"`
	TrendPercent      float64         `json:"trendPercent"`
}

// RecentExercise is one row of the dashboard's "recently trained" list.
type RecentExercise struct {
	ExerciseID   int64     `json:"exerciseId"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	MuscleGroup  string    `json:"muscleGroup,omitempty"`
	LastTrained  time.Time `json:"lastTrained"`
	SessionCount int       `json:"sessionCount"`
	BestEst1RM   float64   `json:"bestEstimated1Rm"`
}

// Progress is the data-access object for derived analytics.
type Progress struct{ db *DB }

// NewProgress builds the progress DAO.
func NewProgress(db *DB) *Progress { return &Progress{db: db} }

// ForExercise returns the per-session time series for one exercise.
//
// The "best set" of a session is the set with the highest estimated 1RM
// (Epley), so a heavy triple outranks a light set of twenty for progression
// purposes. Warmup sets are excluded throughout.
func (p *Progress) ForExercise(ctx context.Context, userID int64, slug string, since *time.Time, formula formulas.Formula) (*ExerciseProgress, error) {
	out := &ExerciseProgress{Slug: slug, Points: []ProgressPoint{}}

	err := p.db.Pool.QueryRow(ctx, `
		SELECT e.id, e.slug, e.name, COALESCE(e.muscle_group, '')
		FROM exercises e
		WHERE e.slug = $1 AND (e.user_id IS NULL OR e.user_id = $2)`,
		slug, userID,
	).Scan(&out.ExerciseID, &out.Slug, &out.Name, &out.MuscleGroup)
	if err != nil {
		return nil, wrapNoRows(err, fmt.Errorf("find exercise %q: %w", slug, err))
	}

	// The e1RM expression is built from the user's chosen formula rather than
	// hardcoded, so a chart and the PR badge that produced it are always computed
	// the same way.
	e1rm := formula.SQLFor("s")

	rows, err := p.db.Pool.Query(ctx, `
		SELECT date_trunc('day', w.performed_at)                       AS session_date,
		       w.id                                                   AS workout_id,
		       MAX(s.weight)                                           AS best_weight,
		       (ARRAY_AGG(s.reps ORDER BY `+e1rm+` DESC))[1]            AS best_reps,
		       MAX(`+e1rm+`)                                           AS est_1rm,
		       COALESCE(SUM(s.weight * s.reps), 0)                     AS volume,
		       COUNT(*)                                               AS set_count
		FROM sets s
		JOIN workout_exercises we ON we.id = s.workout_exercise_id
		JOIN workouts w          ON w.id  = we.workout_id
		WHERE we.exercise_id = $1 AND w.user_id = $2 AND NOT s.is_warmup
		  AND ($3::timestamptz IS NULL OR w.performed_at >= $3)
		GROUP BY session_date, w.id
		ORDER BY session_date ASC`, out.ExerciseID, userID, since)
	if err != nil {
		return nil, fmt.Errorf("progress series: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pt ProgressPoint
		if err := rows.Scan(&pt.Date, &pt.WorkoutID, &pt.BestWeight, &pt.BestReps,
			&pt.Estimated1RM, &pt.TotalVolume, &pt.SetCount); err != nil {
			return nil, fmt.Errorf("scan progress point: %w", err)
		}
		out.Points = append(out.Points, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progress: %w", err)
	}

	out.TotalSessions = len(out.Points)
	if out.TotalSessions > 0 {
		last := out.Points[out.TotalSessions-1]
		out.CurrentEst1RM = last.Estimated1RM
		out.CurrentBestWeight = last.BestWeight
		out.BestEst1RM = last.Estimated1RM
		out.BestEst1RMDate = &last.Date
		if out.TotalSessions > 1 {
			prev := out.Points[out.TotalSessions-2]
			out.PreviousEst1RM = prev.Estimated1RM
			if prev.Estimated1RM > 0 {
				out.TrendPercent = (last.Estimated1RM - prev.Estimated1RM) / prev.Estimated1RM * 100
			}
		}
		for _, pt := range out.Points {
			if pt.Estimated1RM > out.BestEst1RM {
				out.BestEst1RM = pt.Estimated1RM
				d := pt.Date
				out.BestEst1RMDate = &d
			}
		}
	}

	// Count sets explicitly flagged as PRs by the sets service.
	if err := p.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM sets s
		JOIN workout_exercises we ON we.id = s.workout_exercise_id
		JOIN workouts w          ON w.id  = we.workout_id
		WHERE we.exercise_id = $1 AND w.user_id = $2 AND s.is_pr`,
		out.ExerciseID, userID,
	).Scan(&out.PersonalRecords); err != nil {
		return nil, fmt.Errorf("count prs: %w", err)
	}

	return out, nil
}

// ListTracked returns every exercise the user has actually trained, with their
// best estimated 1RM. This is what the Progress page indexes over.
func (p *Progress) ListTracked(ctx context.Context, userID int64, formula formulas.Formula) ([]RecentExercise, error) {
	e1rm := formula.SQLFor("s")

	rows, err := p.db.Pool.Query(ctx, `
		SELECT e.id, e.name, e.slug, COALESCE(e.muscle_group, ''),
		       MAX(w.performed_at)                                        AS last_trained,
		       COUNT(DISTINCT w.id)                                       AS session_count,
		       MAX(`+e1rm+`)                                              AS best_est_1rm
		FROM sets s
		JOIN workout_exercises we ON we.id = s.workout_exercise_id
		JOIN workouts w          ON w.id  = we.workout_id
		JOIN exercises e         ON e.id  = we.exercise_id
		WHERE w.user_id = $1 AND NOT s.is_warmup
		GROUP BY e.id, e.name, e.slug, e.muscle_group
		ORDER BY last_trained DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tracked exercises: %w", err)
	}
	defer rows.Close()

	out := make([]RecentExercise, 0)
	for rows.Next() {
		var re RecentExercise
		if err := rows.Scan(&re.ExerciseID, &re.Name, &re.Slug, &re.MuscleGroup,
			&re.LastTrained, &re.SessionCount, &re.BestEst1RM); err != nil {
			return nil, fmt.Errorf("scan tracked exercise: %w", err)
		}
		out = append(out, re)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracked exercises: %w", err)
	}
	return out, nil
}

// DashboardStats is the aggregate shown on the landing screen.
type DashboardStats struct {
	TotalWorkouts    int     `json:"totalWorkouts"`
	TotalVolume      float64 `json:"totalVolume"`
	TotalSets        int     `json:"totalSets"`
	WorkoutsThisWeek int     `json:"workoutsThisWeek"`
	VolumeThisWeek   float64 `json:"volumeThisWeek"`
	PRsThisMonth     int     `json:"prsThisMonth"`
	StreakDays       int     `json:"streakDays"`
}

// Overview computes the dashboard aggregates in one query.
func (p *Progress) Overview(ctx context.Context, userID int64) (*DashboardStats, error) {
	out := &DashboardStats{}

	err := p.db.Pool.QueryRow(ctx, `
		SELECT
		  COUNT(DISTINCT w.id)                                     AS total_workouts,
		  COALESCE(SUM(s.weight * s.reps) FILTER (WHERE NOT s.is_warmup), 0) AS total_volume,
		  COUNT(s.id) FILTER (WHERE NOT s.is_warmup)               AS total_sets,
		  COUNT(DISTINCT w.id) FILTER (
		    WHERE w.performed_at >= date_trunc('week', now()))    AS workouts_this_week,
		  COALESCE(SUM(s.weight * s.reps) FILTER (
		    WHERE NOT s.is_warmup
		      AND w.performed_at >= date_trunc('week', now())), 0) AS volume_this_week,
		  COUNT(s.id) FILTER (
		    WHERE s.is_pr
		      AND w.performed_at >= date_trunc('month', now()))   AS prs_this_month
		FROM sets s
		JOIN workout_exercises we ON we.id = s.workout_exercise_id
		JOIN workouts w          ON w.id  = we.workout_id
		WHERE w.user_id = $1`, userID,
	).Scan(&out.TotalWorkouts, &out.TotalVolume, &out.TotalSets,
		&out.WorkoutsThisWeek, &out.VolumeThisWeek, &out.PRsThisMonth)
	if err != nil {
		return nil, fmt.Errorf("overview aggregates: %w", err)
	}

	// A workout with no logged sets contributes no rows to the join above, so
	// count empty sessions separately — otherwise "workouts" undercounts and
	// the streak logic disagrees with the history list.
	if err := p.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workouts WHERE user_id = $1`, userID,
	).Scan(&out.TotalWorkouts); err != nil {
		return nil, fmt.Errorf("count workouts: %w", err)
	}
	if err := p.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workouts
		 WHERE user_id = $1 AND performed_at >= date_trunc('week', now())`, userID,
	).Scan(&out.WorkoutsThisWeek); err != nil {
		return nil, fmt.Errorf("count weekly workouts: %w", err)
	}

	// Current and longest streak of consecutive training days.
	rows, err := p.db.Pool.Query(ctx, `
		SELECT DISTINCT date_trunc('day', performed_at)::date AS day
		FROM workouts WHERE user_id = $1 ORDER BY day DESC LIMIT 400`, userID)
	if err != nil {
		return nil, fmt.Errorf("streak days: %w", err)
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan streak day: %w", err)
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate streak days: %w", err)
	}

	// A streak survives if the last session was today or yesterday.
	//
	// days[0] is a calendar date read from Postgres (midnight in the DB's zone),
	// so compare against a date truncated the same way rather than against a
	// raw time.Now() — otherwise the 48h window is off by the UTC offset and a
	// user who trained this morning can read as having no streak.
	if len(days) > 0 {
		today := truncateToDate(time.Now())
		lastDay := truncateToDate(days[0])
		if today.Sub(lastDay) <= 48*time.Hour {
			out.StreakDays = 1
			for i := 1; i < len(days); i++ {
				if truncateToDate(days[i-1]).Sub(truncateToDate(days[i])) == 24*time.Hour {
					out.StreakDays++
				} else {
					break
				}
			}
		}
	}

	return out, nil
}

// truncateToDate drops the time component while staying on the same calendar
// day, so day arithmetic isn't skewed by the clock's time of day.
func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
