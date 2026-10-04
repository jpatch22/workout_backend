package routers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/services"
)

// AuthRouter serves registration, login, and profile endpoints.
type AuthRouter struct{ auth *services.AuthService }

// NewAuthRouter builds the auth router.
func NewAuthRouter(auth *services.AuthService) *AuthRouter { return &AuthRouter{auth: auth} }

// Register mounts the public auth routes under /api/auth.
func (ar *AuthRouter) Register(r *mux.Router) {
	r.HandleFunc("/api/auth/register", ar.handleRegister).Methods(http.MethodPost)
	r.HandleFunc("/api/auth/login", ar.handleLogin).Methods(http.MethodPost)

	// Authenticated sub-tree for the current user's own profile.
	priv := r.PathPrefix("/api/auth").Subrouter()
	priv.Use(middleware.Auth(ar.auth))
	priv.HandleFunc("/me", ar.handleMe).Methods(http.MethodGet)
	priv.HandleFunc("/me", ar.handleUpdateMe).Methods(http.MethodPatch)
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (ar *AuthRouter) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	res, err := ar.auth.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (ar *AuthRouter) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	res, err := ar.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (ar *AuthRouter) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := ar.auth.CurrentUser(r.Context(), middleware.UserID(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type updateMeRequest struct {
	DisplayName string `json:"displayName"`
}

func (ar *AuthRouter) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var req updateMeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	u, err := ar.auth.UpdateProfile(r.Context(), middleware.UserID(r), req.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
