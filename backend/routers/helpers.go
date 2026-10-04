// Package routers maps HTTP to services. Handlers here only decode input, call
// one service method, and encode the result — no business logic, no SQL.
package routers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/services"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// errorBody is the single error shape every endpoint returns, so the frontend
// has exactly one thing to parse.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// WriteJSON writes a JSON response. Exported so the server package can serve
// its health endpoint in exactly the same shape as every router.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// WriteError writes the standard error body.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, errorBody{Error: code, Message: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) { WriteJSON(w, status, v) }

func writeError(w http.ResponseWriter, status int, code, msg string) {
	WriteError(w, status, code, msg)
}

// writeServiceError is the single place that translates a service-layer error
// into an HTTP status. Adding a new error kind to the services package means
// changing the switch here and nowhere else.
func writeServiceError(w http.ResponseWriter, err error) {
	var ve services.ValidationError
	switch {
	case errors.As(err, &ve):
		body := errorBody{Error: "validation_error", Message: ve.Message, Field: ve.Field}
		writeJSON(w, http.StatusBadRequest, body)
	case errors.Is(err, services.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "a valid bearer token is required")
	case errors.Is(err, services.ErrForbidden), errors.Is(err, services.ErrNotFound),
		errors.Is(err, tables.ErrNotFound):
		// 404 rather than 403 for other users' rows: a 403 would confirm the
		// id exists, which is itself a leak.
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

// decodeJSON reads a JSON body with a size cap and strict field checking.
func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB is far above any real payload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

// decodeID reads a positive integer path variable.
func decodeID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(mux.Vars(r)[name], 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// decodePositiveInt parses an optional positive integer query parameter.
func decodePositiveInt(s string, dst *int) (bool, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return false, errors.New("not a positive integer")
	}
	*dst = v
	return true, nil
}

// parseOptionalTime reads an RFC3339 timestamp field, treating "" as absent.
func parseOptionalTime(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// parseRPE validates an optional RPE value.
func parseRPE(v *float64) (*float64, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 0 || *v > 10 {
		return nil, services.ValidationError{Field: "rpe", Message: "must be between 0 and 10"}
	}
	return v, nil
}
