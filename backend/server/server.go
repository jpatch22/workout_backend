// Package server wires the layers together: config -> tables -> services ->
// routers -> HTTP. This is the only file that knows about all of them.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackpatchell/workout-tracker/backend/config"
	"github.com/jackpatchell/workout-tracker/backend/middleware"
	"github.com/jackpatchell/workout-tracker/backend/routers"
	"github.com/jackpatchell/workout-tracker/backend/services"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// NewRouter connects to the database, optionally migrates, and builds the
// fully-wired http.Handler.
func NewRouter(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	db, err := tables.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	cleanup := db.Close

	if cfg.AutoMigrate {
		if err := db.Migrate(ctx); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("migrate: %w", err)
		}
	}

	// --- tables (database access) ---
	usersTable := tables.NewUsers(db)
	exercisesTable := tables.NewExercises(db)
	workoutsTable := tables.NewWorkouts(db)
	setsTable := tables.NewSets(db)
	progressTable := tables.NewProgress(db)

	// --- services (business logic) ---
	authSvc := services.NewAuthService(usersTable, cfg.JWTSecret, cfg.JWTTTL)
	exerciseSvc := services.NewExerciseService(exercisesTable)
	workoutSvc := services.NewWorkoutService(workoutsTable, setsTable)
	progressSvc := services.NewProgressService(progressTable, workoutsTable)

	// --- routers (HTTP mapping) ---
	r := mux.NewRouter()
	r.HandleFunc("/api/health", healthHandler).Methods(http.MethodGet)

	routers.NewAuthRouter(authSvc).Register(r)
	routers.NewExerciseRouter(exerciseSvc).Register(r, authSvc)
	routers.NewWorkoutRouter(workoutSvc).Register(r, authSvc)
	routers.NewSetRouter(workoutSvc).Register(r, authSvc)
	routers.NewProgressRouter(progressSvc).Register(r, authSvc)

	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowedHandler)

	// Recovery wraps everything; logging sits inside so panics are logged.
	var handler http.Handler = r
	handler = middleware.Logger(handler)
	handler = middleware.Recoverer(handler)
	handler = corsMiddleware(cfg)(handler)

	return handler, cleanup, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	routers.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "workout-tracker-api",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	routers.WriteError(w, http.StatusNotFound, "not_found", "no such endpoint")
}

func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	routers.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed for this endpoint")
}

// corsMiddleware allows the configured origins (the Vite dev server, and the
// GitHub Pages deployment) to call the API with credentials.
func corsMiddleware(cfg config.Config) mux.MiddlewareFunc {
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled, then drains.
func Run(ctx context.Context, handler http.Handler, port string) error {
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
