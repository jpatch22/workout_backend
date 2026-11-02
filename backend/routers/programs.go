package routers

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

type ProgramRouter struct{ programs *services.ProgramService }

func NewProgramRouter(programs *services.ProgramService) *ProgramRouter {
	return &ProgramRouter{programs: programs}
}

func (pr *ProgramRouter) Register(r *mux.Router, auth *services.AuthService) {
	sub := r.PathPrefix("/api/programs").Subrouter()
	sub.Use(middleware.Auth(auth))
	sub.HandleFunc("", pr.handleList).Methods(http.MethodGet)
	sub.HandleFunc("", pr.handleCreate).Methods(http.MethodPost)
	sub.HandleFunc("/{id}/sessions/{sessionId}/start", pr.handleStart).Methods(http.MethodPost)
	sub.HandleFunc("/{id}/sessions/{sessionId}/complete", pr.handleComplete).Methods(http.MethodPost)
	sub.HandleFunc("/{id}/sessions/{sessionId}/skip", pr.handleSkip).Methods(http.MethodPost)
	sub.HandleFunc("/{id}/restart", pr.handleRestart).Methods(http.MethodPost)
}

func (pr *ProgramRouter) handleList(w http.ResponseWriter, r *http.Request) {
	programs, err := pr.programs.List(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, programs)
}

type createProgramRequest struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Weeks       []tables.ProgramWeek `json:"weeks"`
}

func (pr *ProgramRouter) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createProgramRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	program, err := pr.programs.Create(r.Context(), middleware.UserID(r), req.Name, req.Description, req.Weeks)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, program)
}

func (pr *ProgramRouter) handleStart(w http.ResponseWriter, r *http.Request) {
	programID, sessionID, ok := programPathIDs(w, r)
	if !ok {
		return
	}
	workout, err := pr.programs.StartSession(r.Context(), middleware.UserID(r), programID, sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workout)
}

func (pr *ProgramRouter) handleComplete(w http.ResponseWriter, r *http.Request) {
	programID, sessionID, ok := programPathIDs(w, r)
	if !ok {
		return
	}
	program, err := pr.programs.CompleteSession(r.Context(), middleware.UserID(r), programID, sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, program)
}

func (pr *ProgramRouter) handleSkip(w http.ResponseWriter, r *http.Request) {
	programID, sessionID, ok := programPathIDs(w, r)
	if !ok {
		return
	}
	program, err := pr.programs.SkipSession(r.Context(), middleware.UserID(r), programID, sessionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, program)
}

func (pr *ProgramRouter) handleRestart(w http.ResponseWriter, r *http.Request) {
	programID, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return
	}
	program, err := pr.programs.Restart(r.Context(), middleware.UserID(r), programID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, program)
}

func programPathIDs(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	programID, err := decodeID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return 0, "", false
	}
	sessionID := strings.TrimSpace(mux.Vars(r)["sessionId"])
	if sessionID == "" || len(sessionID) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_id", "sessionId is required")
		return 0, "", false
	}
	return programID, sessionID, true
}
