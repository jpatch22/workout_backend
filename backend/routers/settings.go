package routers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
)

// SettingsRouter serves the user's preferences.
type SettingsRouter struct{ settings *services.SettingsService }

// NewSettingsRouter builds the settings router.
func NewSettingsRouter(s *services.SettingsService) *SettingsRouter {
	return &SettingsRouter{settings: s}
}

// Register mounts the authenticated settings routes under /api/settings.
func (sr *SettingsRouter) Register(r *mux.Router, auth *services.AuthService) {
	sub := r.PathPrefix("/api/settings").Subrouter()
	sub.Use(middleware.Auth(auth))

	sub.HandleFunc("", sr.handleGet).Methods(http.MethodGet)
	sub.HandleFunc("", sr.handleUpdate).Methods(http.MethodPatch, http.MethodPut)

	// The formula catalog is static, but it is served from the backend so the UI
	// never hardcodes formula names or descriptions.
	sub.HandleFunc("/formulas", sr.handleFormulas).Methods(http.MethodGet)
}

func (sr *SettingsRouter) handleGet(w http.ResponseWriter, r *http.Request) {
	settings, err := sr.settings.Get(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (sr *SettingsRouter) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var patch services.SettingsPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	settings, err := sr.settings.Update(r.Context(), middleware.UserID(r), patch)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (sr *SettingsRouter) handleFormulas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sr.settings.Catalog())
}
