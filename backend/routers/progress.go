package routers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
)

// ProgressRouter serves the derived analytics endpoints.
type ProgressRouter struct{ progress *services.ProgressService }

// NewProgressRouter builds the progress router.
func NewProgressRouter(p *services.ProgressService) *ProgressRouter {
	return &ProgressRouter{progress: p}
}

// Register mounts the authenticated progress routes under /api/progress.
func (pr *ProgressRouter) Register(r *mux.Router, auth *services.AuthService) {
	sub := r.PathPrefix("/api/progress").Subrouter()
	sub.Use(middleware.Auth(auth))

	sub.HandleFunc("", pr.handleOverview).Methods(http.MethodGet)
	sub.HandleFunc("/dashboard", pr.handleDashboard).Methods(http.MethodGet)
	sub.HandleFunc("/exercises", pr.handleTracked).Methods(http.MethodGet)
	sub.HandleFunc("/exercise/{slug}", pr.handleExercise).Methods(http.MethodGet)
}

func (pr *ProgressRouter) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := pr.progress.Overview(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (pr *ProgressRouter) handleDashboard(w http.ResponseWriter, r *http.Request) {
	d, err := pr.progress.Dashboard(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (pr *ProgressRouter) handleTracked(w http.ResponseWriter, r *http.Request) {
	list, err := pr.progress.TrackedExercises(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (pr *ProgressRouter) handleExercise(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	if slug == "" {
		writeError(w, http.StatusBadRequest, "invalid_slug", "slug is required")
		return
	}

	since, err := parseOptionalTime(r.URL.Query().Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_since", "since must be RFC3339")
		return
	}

	p, err := pr.progress.ExerciseProgress(r.Context(), middleware.UserID(r), slug, since)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
