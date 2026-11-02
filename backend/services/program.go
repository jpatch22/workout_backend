package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// ProgramService owns user-created training plans and their current run state.
type ProgramService struct {
	programs  *tables.Programs
	exercises *ExerciseService
	workouts  *WorkoutService
}

func NewProgramService(programs *tables.Programs, exercises *ExerciseService, workouts *WorkoutService) *ProgramService {
	return &ProgramService{programs: programs, exercises: exercises, workouts: workouts}
}

func (s *ProgramService) List(ctx context.Context, userID int64) ([]tables.Program, error) {
	programs, err := s.programs.ListByUser(ctx, userID)
	if err != nil {
		return nil, wrapTableErr(err, "list programs")
	}
	return programs, nil
}

func (s *ProgramService) Create(ctx context.Context, userID int64, name, description string, weeks []tables.ProgramWeek) (*tables.Program, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalid("name", "is required")
	}
	if len(name) > 100 {
		return nil, invalid("name", "must be at most 100 characters")
	}
	if len(description) > 240 {
		return nil, invalid("description", "must be at most 240 characters")
	}
	if err := s.validateWeeks(ctx, userID, weeks); err != nil {
		return nil, err
	}
	program, err := s.programs.Create(ctx, &tables.Program{
		UserID: userID, Name: name, Description: strings.TrimSpace(description), Weeks: weeks,
	})
	if err != nil {
		return nil, wrapTableErr(err, "create program")
	}
	return program, nil
}

func (s *ProgramService) validateWeeks(ctx context.Context, userID int64, weeks []tables.ProgramWeek) error {
	if len(weeks) == 0 || len(weeks) > 52 {
		return invalid("weeks", "must contain between 1 and 52 weeks")
	}
	seenWeeks := map[string]bool{}
	seenSessions := map[string]bool{}
	for wi := range weeks {
		week := &weeks[wi]
		week.ID = strings.TrimSpace(week.ID)
		week.Name = strings.TrimSpace(week.Name)
		if week.ID == "" || seenWeeks[week.ID] {
			return invalid("weeks", "week IDs must be present and unique")
		}
		seenWeeks[week.ID] = true
		if week.Name == "" || len(week.Name) > 80 {
			return invalid("weeks", "week names are required and must be at most 80 characters")
		}
		if len(week.Sessions) == 0 {
			return invalid("weeks", "each week must contain at least one session")
		}
		for si := range week.Sessions {
			session := &week.Sessions[si]
			session.ID = strings.TrimSpace(session.ID)
			session.Name = strings.TrimSpace(session.Name)
			if session.ID == "" || seenSessions[session.ID] {
				return invalid("weeks", "session IDs must be present and unique")
			}
			seenSessions[session.ID] = true
			if session.Name == "" || len(session.Name) > 80 {
				return invalid("weeks", "session names are required and must be at most 80 characters")
			}
			if session.Status == "" {
				session.Status = "upcoming"
			}
			switch session.Status {
			case "upcoming", "completed", "skipped":
			default:
				return invalid("weeks", "session status is invalid")
			}
			if session.WorkoutID != nil {
				if *session.WorkoutID <= 0 {
					return invalid("weeks", "workout IDs must be positive")
				}
				if _, err := s.workouts.GetWorkout(ctx, userID, *session.WorkoutID); err != nil {
					if errors.Is(err, ErrNotFound) {
						return invalid("weeks", "contains a workout you cannot use")
					}
					return err
				}
			}
			if session.Status == "completed" && session.WorkoutID == nil {
				return invalid("weeks", "completed sessions must be linked to a workout")
			}
			if session.Status == "skipped" && session.WorkoutID != nil {
				return invalid("weeks", "skipped sessions cannot be linked to a workout")
			}
			if len(session.Exercises) == 0 {
				return invalid("weeks", "each session must contain at least one exercise")
			}
			seenExercises := map[int64]bool{}
			for ei := range session.Exercises {
				exercise := &session.Exercises[ei]
				if exercise.ExerciseID <= 0 || seenExercises[exercise.ExerciseID] {
					return invalid("weeks", "exercise IDs must be positive and unique within a session")
				}
				seenExercises[exercise.ExerciseID] = true
				if _, err := s.exercises.Get(ctx, userID, exercise.ExerciseID); err != nil {
					if errors.Is(err, ErrNotFound) {
						return invalid("weeks", "contains an exercise you cannot use")
					}
					return err
				}
				if len(exercise.Notes) > 500 {
					return invalid("weeks", "exercise notes must be at most 500 characters")
				}
				if len(exercise.Sets) == 0 || len(exercise.Sets) > 30 {
					return invalid("weeks", "each exercise must contain between 1 and 30 target sets")
				}
				for _, set := range exercise.Sets {
					if set.Reps < 1 || set.Reps > 100 {
						return invalid("weeks", "target reps must be between 1 and 100")
					}
					if set.RPE != nil && (*set.RPE < 1 || *set.RPE > 10) {
						return invalid("weeks", "target RPE must be between 1 and 10")
					}
				}
				exercise.Notes = strings.TrimSpace(exercise.Notes)
			}
		}
	}
	return nil
}

func (s *ProgramService) StartSession(ctx context.Context, userID, programID int64, sessionID string) (*tables.Workout, error) {
	program, err := s.get(ctx, userID, programID)
	if err != nil {
		return nil, err
	}
	session, err := nextProgramSession(program, sessionID)
	if err != nil {
		return nil, err
	}
	if session.WorkoutID != nil {
		return s.workouts.GetWorkout(ctx, userID, *session.WorkoutID)
	}
	workoutName := program.Name + " · " + session.Name
	for len(workoutName) > 120 {
		_, size := utf8.DecodeLastRuneInString(workoutName)
		workoutName = workoutName[:len(workoutName)-size]
	}
	workout, err := s.workouts.CreateWorkout(ctx, userID, workoutName, nil, "")
	if err != nil {
		return nil, err
	}
	for _, exercise := range session.Exercises {
		prescription := make([]string, len(exercise.Sets))
		for i, set := range exercise.Sets {
			prescription[i] = "Set " + strconv.Itoa(i+1) + ": " + strconv.Itoa(set.Reps) + " reps"
			if set.RPE != nil {
				prescription[i] += " @ RPE " + strconv.FormatFloat(*set.RPE, 'f', -1, 64)
			}
		}
		notes := "Program target — " + strings.Join(prescription, " · ")
		if exercise.Notes != "" {
			notes += "\n" + exercise.Notes
		}
		if _, err := s.workouts.AddExercise(ctx, userID, workout.ID, exercise.ExerciseID, notes); err != nil {
			return nil, err
		}
	}
	workoutID := workout.ID
	session.WorkoutID = &workoutID
	if err := s.programs.UpdateWeeks(ctx, userID, programID, program.Weeks); err != nil {
		return nil, wrapTableErr(err, "start program session")
	}
	return s.workouts.GetWorkout(ctx, userID, workout.ID)
}

func (s *ProgramService) SkipSession(ctx context.Context, userID, programID int64, sessionID string) (*tables.Program, error) {
	program, err := s.get(ctx, userID, programID)
	if err != nil {
		return nil, err
	}
	session, err := nextProgramSession(program, sessionID)
	if err != nil {
		return nil, err
	}
	if session.WorkoutID != nil {
		return nil, invalid("session", "an in-progress session must be completed or resumed, not skipped")
	}
	session.Status = "skipped"
	return s.save(ctx, userID, program)
}

func (s *ProgramService) CompleteSession(ctx context.Context, userID, programID int64, sessionID string) (*tables.Program, error) {
	program, err := s.get(ctx, userID, programID)
	if err != nil {
		return nil, err
	}
	session, err := nextProgramSession(program, sessionID)
	if err != nil {
		return nil, err
	}
	if session.WorkoutID == nil {
		return nil, invalid("session", "start the workout before marking it complete")
	}
	session.Status = "completed"
	return s.save(ctx, userID, program)
}

func (s *ProgramService) Restart(ctx context.Context, userID, programID int64) (*tables.Program, error) {
	program, err := s.get(ctx, userID, programID)
	if err != nil {
		return nil, err
	}
	for wi := range program.Weeks {
		for si := range program.Weeks[wi].Sessions {
			program.Weeks[wi].Sessions[si].Status = "upcoming"
			program.Weeks[wi].Sessions[si].WorkoutID = nil
		}
	}
	return s.save(ctx, userID, program)
}

func (s *ProgramService) get(ctx context.Context, userID, programID int64) (*tables.Program, error) {
	program, err := s.programs.GetOwned(ctx, userID, programID)
	if err != nil {
		return nil, wrapTableErr(err, "get program")
	}
	return program, nil
}

func (s *ProgramService) save(ctx context.Context, userID int64, program *tables.Program) (*tables.Program, error) {
	if err := s.programs.UpdateWeeks(ctx, userID, program.ID, program.Weeks); err != nil {
		return nil, wrapTableErr(err, "save program progress")
	}
	return program, nil
}

func nextProgramSession(program *tables.Program, sessionID string) (*tables.ProgramSession, error) {
	var next *tables.ProgramSession
	var requested *tables.ProgramSession
	for wi := range program.Weeks {
		for si := range program.Weeks[wi].Sessions {
			session := &program.Weeks[wi].Sessions[si]
			if session.ID == sessionID {
				requested = session
			}
			if next == nil && session.Status == "upcoming" {
				next = session
			}
		}
	}
	if requested == nil {
		return nil, ErrNotFound
	}
	if requested.Status != "upcoming" {
		return nil, invalid("session", "this session is already completed or skipped")
	}
	if next == nil || next.ID != requested.ID {
		return nil, invalid("session", "sessions must be completed or skipped in program order")
	}
	return requested, nil
}
