package routers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
)

// ExerciseRouter serves the exercise catalog.
type ExerciseRouter struct{ exercises *services.ExerciseService }

// NewExerciseRouter builds the exercise router.
func NewExerciseRouter(e *services.ExerciseService) *ExerciseRouter {
	return &ExerciseRouter{exercises: e}
}

// Register mounts the authenticated exercise routes under /api/exercises.
func (er *ExerciseRouter) Register(r *mux.Router, auth *services.AuthService) {
	sub := r.PathPrefix("/api/exercises").Subrouter()
	sub.Use(middleware.Auth(auth))

	sub.HandleFunc("", er.handleList).Methods(http.MethodGet)
	sub.HandleFunc("", er.handleCreate).Methods(http.MethodPost)
	sub.HandleFunc("/{id}", er.handleGet).Methods(http.MethodGet)
	sub.HandleFunc("/{id}", er.handleDelete).Methods(http.MethodDelete)
}

func (er *ExerciseRouter) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := er.exercises.List(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type createExerciseRequest struct {
	Name        string `json:"name"`
	MuscleGroup string `json:"muscleGroup"`
	Equipment   string `json:"equipment"`
}

func (er *ExerciseRouter) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	ex, err := er.exercises.Create(r.Context(), middleware.UserID(r), req.Name, req.MuscleGroup, req.Equipment)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ex)
}

func (er *ExerciseRouter) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	ex, err := er.exercises.Get(r.Context(), middleware.UserID(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ex)
}

func (er *ExerciseRouter) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}

	if err := er.exercises.Delete(r.Context(), middleware.UserID(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
