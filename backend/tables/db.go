// Package tables holds the database models and all SQL access.
//
// Rule for the rest of the codebase: no SQL outside this package. Services and
// routers call the functions here and never see a query string. That keeps the
// database shape changeable in exactly one directory.
package tables

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed all:migrations
var migrationsFS embed.FS

// ErrNotFound is returned by every lookup that finds no matching row, so
// services can distinguish "missing" from "broken" with errors.Is.
var ErrNotFound = errors.New("not found")

// DB wraps the connection pool and is passed to every table constructor.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect opens the pool and verifies connectivity with a ping.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// Sensible pool defaults for a small app; tune per-environment later.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close releases the pool.
func (db *DB) Close() { db.Pool.Close() }

// Migrate applies any pending migrations from the embedded migrations/ directory.
//
// golang-migrate's postgres driver speaks database/sql, so we hand it a stdlib
// handle backed by pgx and let it borrow a connection from the existing pool
// rather than opening a second one.
func (db *DB) Migrate(ctx context.Context) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	sqlDB := stdlib.OpenDB(*db.Pool.Config().ConnConfig)
	defer sqlDB.Close()

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// inTx runs fn inside a transaction, rolling back on error or panic.
func (db *DB) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

// slugify turns a display name into a URL/DB-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// uniqueSlug appends -2, -3, ... until the slug is free in the given scope.
func uniqueSlug(ctx context.Context, q queryer, base string, userID *int64) (string, error) {
	candidate := base
	for i := 2; i < 200; i++ {
		var exists bool
		var err error
		if userID == nil {
			err = q.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM exercises WHERE slug = $1 AND user_id IS NULL)`,
				candidate).Scan(&exists)
		} else {
			err = q.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM exercises WHERE slug = $1 AND user_id = $2)`,
				candidate, *userID).Scan(&exists)
		}
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", errors.New("could not generate unique slug")
}

// queryer is satisfied by both *pgxpool.Pool and pgx.Tx, so table functions can
// run either standalone or inside a caller's transaction.
type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// wrapNoRows maps pgx's empty-result sentinel onto the package-level ErrNotFound
// while preserving the caller's wrapped message for context.
func wrapNoRows(err error, wrapped error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %w", ErrNotFound, wrapped)
	}
	return wrapped
}
