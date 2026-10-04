//go:build tools

// Command pgtest boots a real PostgreSQL server for integration tests.
//
// The project's docker-compose database is the intended way to run tests, but
// that needs a working Docker registry. This fallback starts the same engine
// from a downloaded binary so `go test` still exercises real SQL rather than
// skipping when Docker is unavailable.
//
// It is guarded by the `tools` build tag so it never compiles into the app.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func main() {
	cfg := embeddedpostgres.DefaultConfig().
		Port(5432).
		Username("postgres").
		Password("postgres").
		Database("workout_tracker").
		DataPath("/tmp/wt-pgdata").
		BinariesPath("/tmp/wt-pgbin").
		RuntimePath("/tmp/wt-pgrun").
		StartTimeout(180 * time.Second).
		Logger(os.Stdout)

	pg := embeddedpostgres.NewDatabase(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := pg.Start(); err != nil {
		log.Fatalf("start embedded postgres: %v", err)
	}
	defer func() {
		if err := pg.Stop(); err != nil {
			log.Printf("stop embedded postgres: %v", err)
		}
	}()

	fmt.Println("READY postgres://postgres:postgres@localhost:5432/workout_tracker?sslmode=disable")

	<-ctx.Done()
	fmt.Println("shutting down")
}
