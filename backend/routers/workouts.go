package routers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
)

// WorkoutRouter serves sessions and the sets logged inside them.
type WorkoutRouter struct{ workouts *services.WorkoutService }

// NewWorkoutRouter builds the workout router.
func NewWorkoutRouter(w *services.WorkoutService) *WorkoutRouter { return &WorkoutRouter{workouts: w} }

// Register mounts the authenticated workout routes under /api/workouts.
func (wr *WorkoutRouter) Register(r *mux.Router, auth *services.AuthService) {
	sub := r.PathPrefix("/api/workouts").Subrouter()
	sub.Use(middleware.Auth(auth))

	sub.HandleFunc("", wr.handleList).Methods(http.MethodGet)
	sub.HandleFunc("", wr.handleCreate).Methods(http.MethodPost)
	sub.HandleFunc("/{id}", wr.handleGet).Methods(http.MethodGet)
	sub.HandleFunc("/{id}", wr.handleUpdate).Methods(http.MethodPut, http.MethodPatch)
	sub.HandleFunc("/{id}", wr.handleDelete).Methods(http.MethodDelete)

	sub.HandleFunc("/{id}/exercises", wr.handleAddExercise).Methods(http.MethodPost)
	sub.HandleFunc("/{id}/exercises/{weId}", wr.handleRemoveExercise).Methods(http.MethodDelete)
}

func (wr *WorkoutRouter) handleList(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if _, err := decodePositiveInt(v, &limit); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return
		}
	}

	list, err := wr.workouts.ListWorkouts(r.Context(), middleware.UserID(r), limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type workoutRequest struct {
	Name        string `json:"name"`
	PerformedAt string `json:"performedAt"`
	Notes       string `json:"notes"`
}

func (wr *WorkoutRouter) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req workoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	performedAt, err := parseOptionalTime(req.PerformedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "performedAt must be RFC3339")
		return
	}

	wk, err := wr.workouts.CreateWorkout(r.Context(), middleware.UserID(r), req.Name, performedAt, req.Notes)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wk)
}

func (wr *WorkoutRouter) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	wk, err := wr.workouts.GetWorkout(r.Context(), middleware.UserID(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wk)
}

func (wr *WorkoutRouter) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	var req workoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	performedAt, err := parseOptionalTime(req.PerformedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "performedAt must be RFC3339")
		return
	}

	wk, err := wr.workouts.UpdateWorkout(r.Context(), middleware.UserID(r), id, req.Name, performedAt, req.Notes)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wk)
}

func (wr *WorkoutRouter) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	if err := wr.workouts.DeleteWorkout(r.Context(), middleware.UserID(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addExerciseRequest struct {
	ExerciseID int64  `json:"exerciseId"`
	Notes      string `json:"notes"`
}

func (wr *WorkoutRouter) handleAddExercise(w http.ResponseWriter, r *http.Request) {
	workoutID, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	var req addExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	we, err := wr.workouts.AddExercise(r.Context(), middleware.UserID(r), workoutID, req.ExerciseID, req.Notes)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, we)
}

func (wr *WorkoutRouter) handleRemoveExercise(w http.ResponseWriter, r *http.Request) {
	workoutID, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}
	weID, err := decodeID(r, "weId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "workoutExerciseId must be a positive integer")
		return
	}

	if err := wr.workouts.RemoveExercise(r.Context(), middleware.UserID(r), workoutID, weID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetRouter serves individual set logging. Sets live at the top level
// (/api/sets/:id) because they are addressed by their own id once created.
type SetRouter struct{ workouts *services.WorkoutService }

// NewSetRouter builds the set router.
func NewSetRouter(w *services.WorkoutService) *SetRouter { return &SetRouter{workouts: w} }

// Register mounts the authenticated set routes.
//
// The prefixes are deliberately narrow (`/api/sets`, `/api/workout-exercises`)
// rather than a single broad `/api` subrouter: a catch-all prefix here would
// match every /api path, including ones that belong to other routers, and
// swallow their 404/405 responses before the root handlers ever see them.
func (sr *SetRouter) Register(r *mux.Router, auth *services.AuthService) {
	sets := r.PathPrefix("/api/sets").Subrouter()
	sets.Use(middleware.Auth(auth))
	sets.HandleFunc("/{id}", sr.handleUpdate).Methods(http.MethodPut, http.MethodPatch)
	sets.HandleFunc("/{id}", sr.handleDelete).Methods(http.MethodDelete)

	workoutExercises := r.PathPrefix("/api/workout-exercises").Subrouter()
	workoutExercises.Use(middleware.Auth(auth))
	workoutExercises.HandleFunc("/{weId}/sets", sr.handleCreate).Methods(http.MethodPost)
}

type setRequest struct {
	Weight       float64  `json:"weight"`
	Reps         int      `json:"reps"`
	RPE          *float64 `json:"rpe"`
	IsWarmup     bool     `json:"isWarmup"`
	IsBodyweight bool     `json:"isBodyweight"`
}

func (req setRequest) toInput() (services.LogSetInput, error) {
	rpe, err := parseRPE(req.RPE)
	if err != nil {
		return services.LogSetInput{}, err
	}
	return services.LogSetInput{
		Weight:       req.Weight,
		Reps:         req.Reps,
		RPE:          rpe,
		IsWarmup:     req.IsWarmup,
		IsBodyweight: req.IsBodyweight,
	}, nil
}

func (sr *SetRouter) handleCreate(w http.ResponseWriter, r *http.Request) {
	weID, err := decodeID(r, "weId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "workoutExerciseId must be a positive integer")
		return
	}

	var req setRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeServiceError(w, err)
		return
	}

	set, err := sr.workouts.LogSet(r.Context(), middleware.UserID(r), weID, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, set)
}

func (sr *SetRouter) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	var req setRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeServiceError(w, err)
		return
	}

	set, err := sr.workouts.UpdateSet(r.Context(), middleware.UserID(r), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (sr *SetRouter) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	if err := sr.workouts.DeleteSet(r.Context(), middleware.UserID(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
