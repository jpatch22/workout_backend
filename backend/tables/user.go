package tables

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// User is an account row. PasswordHash never leaves this package except through
// AuthenticatePassword, which compares rather than returns it.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Users is the data-access object for the users table.
type Users struct{ db *DB }

// NewUsers builds the users DAO.
func NewUsers(db *DB) *Users { return &Users{db: db} }

const userCols = `id, email, password_hash, display_name, created_at, updated_at`

// Create inserts a user and returns it, including generated ID and timestamps.
func (u *Users) Create(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var out User
	err := u.db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING `+userCols,
		email, passwordHash, displayName,
	).Scan(&out.ID, &out.Email, &out.PasswordHash, &out.DisplayName, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("email already registered: %w", err)
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &out, nil
}

// GetByEmail loads a user for login. Returns ErrNotFound if absent.
func (u *Users) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(u.db.Pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(email)),
	))
}

// GetByID loads a user by primary key.
func (u *Users) GetByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(u.db.Pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

// UpdateProfile changes the mutable, non-credential fields.
func (u *Users) UpdateProfile(ctx context.Context, id int64, displayName string) (*User, error) {
	out, err := scanUser(u.db.Pool.QueryRow(ctx,
		`UPDATE users SET display_name = $2 WHERE id = $1 RETURNING `+userCols,
		id, displayName))
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return out, nil
}

func scanUser(row pgx.Row) (*User, error) {
	var out User
	err := row.Scan(&out.ID, &out.Email, &out.PasswordHash, &out.DisplayName, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &out, nil
}
