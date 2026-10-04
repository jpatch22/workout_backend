package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// User is the publicly-safe user shape returned to clients.
type User struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// AuthResult carries the token plus the user on register/login.
type AuthResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Claims is the JWT payload.
type Claims struct {
	UserID int64 `json:"uid"`
	jwt.RegisteredClaims
}

// AuthService owns registration, login, and token issuance.
type AuthService struct {
	users     *tables.Users
	settings  *tables.Settings
	jwtSecret []byte
	jwtTTL    time.Duration
}

// NewAuthService builds the auth service. The settings DAO is optional: when
// nil, registration simply skips creating a preferences row and the row is
// created lazily on first read instead.
func NewAuthService(users *tables.Users, settings *tables.Settings, secret []byte, ttl time.Duration) *AuthService {
	return &AuthService{users: users, settings: settings, jwtSecret: secret, jwtTTL: ttl}
}

// Register creates an account and returns a signed-in session.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	u, err := s.users.Create(ctx, email, hash, displayName)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return nil, ValidationError{Field: "email", Message: "is already registered"}
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Create the preferences row up front so the settings endpoint returns
	// defaults immediately. A failure here is not worth blocking registration
	// over: Get creates the row lazily, so the worst case is one extra query.
	if s.settings != nil {
		if err := s.settings.EnsureExists(ctx, u.ID); err != nil {
			return nil, fmt.Errorf("create settings for new user: %w", err)
		}
	}

	token, err := s.issueToken(u.ID)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: toUser(u)}, nil
}

// Login verifies credentials and returns a session. A wrong email and a wrong
// password return the identical error so the endpoint can't enumerate accounts.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, tables.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if !checkPassword(u.PasswordHash, password) {
		return nil, ErrUnauthorized
	}

	token, err := s.issueToken(u.ID)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: toUser(u)}, nil
}

// CurrentUser returns the profile for an authenticated user id.
func (s *AuthService) CurrentUser(ctx context.Context, userID int64) (*User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, tables.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("load user: %w", err)
	}
	out := toUser(u)
	return &out, nil
}

// UpdateProfile changes the display name.
func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, displayName string) (*User, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, invalid("displayName", "is required")
	}
	u, err := s.users.UpdateProfile(ctx, userID, displayName)
	if err != nil {
		if errors.Is(err, tables.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}
	out := toUser(u)
	return &out, nil
}

// ParseToken validates a bearer token and returns the user id it names.
func (s *AuthService) ParseToken(tokenString string) (int64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return 0, ErrUnauthorized
	}
	if !token.Valid || claims.UserID == 0 {
		return 0, ErrUnauthorized
	}
	return claims.UserID, nil
}

func (s *AuthService) issueToken(userID int64) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtTTL)),
			Issuer:    "workout-tracker",
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func toUser(u *tables.User) User {
	return User{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName}
}
