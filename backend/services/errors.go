// Package services holds the application's business logic.
//
// Rules for the whole codebase:
//   - routers call services
//   - services call tables
//   - only tables writes SQL
//
// Keeping validation and derived logic here means a new feature is one file in
// this package plus one router function, with no other layer needing edits.
package services

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// ValidationError is a user-correctable input problem. Routers map it to 400
// with the field messages intact so the UI can show them inline.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func invalid(field, msg string) error { return ValidationError{Field: field, Message: msg} }

// ErrUnauthorized is returned for missing, malformed, or invalid credentials.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when a valid user touches another user's resource.
var ErrForbidden = errors.New("forbidden")

// validatePassword enforces a minimum strength bar for new accounts.
func validatePassword(pw string) error {
	if len(pw) < 8 {
		return invalid("password", "must be at least 8 characters")
	}
	if len(pw) > 72 {
		// bcrypt silently truncates beyond 72 bytes; reject rather than mislead.
		return invalid("password", "must be at most 72 characters")
	}
	var hasLetter, hasOther bool
	for _, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasOther = true
		}
	}
	if !hasLetter {
		return invalid("password", "must contain a letter")
	}
	if !hasOther {
		return invalid("password", "must contain a digit or symbol")
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return invalid("email", "is required")
	}
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 {
		return invalid("email", "must be a valid address")
	}
	if !strings.Contains(email[at+1:], ".") {
		return invalid("email", "must be a valid address")
	}
	return nil
}

func hashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
