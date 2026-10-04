// Command server runs the workout-tracker REST API.
//
// Configuration comes entirely from the environment (see backend/config), so
// the same binary runs locally, in CI, and on Render unchanged.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackpatchell/workout-tracker/backend/config"
	"github.com/jackpatchell/workout-tracker/backend/server"
	"github.com/joho/godotenv"
)

func main() {
	// .env is a local convenience only; missing file is not an error in CI.
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file loaded (%v); using environment", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Signal-aware context so a deploy drains in-flight requests.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler, cleanup, err := server.NewRouter(ctx, cfg)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer cleanup()

	if err := server.Run(ctx, handler, cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
