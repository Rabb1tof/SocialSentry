// Package service implements business logic that sits between HTTP handlers and repositories.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	jwtpkg "github.com/rabb1tof/socialsentry/backend/pkg/jwt"
)

// Sentinel errors callers can compare against with errors.Is.
var (
	ErrUserExists     = errors.New("service.auth: user already exists")
	ErrInvalidCreds   = errors.New("service.auth: invalid credentials")
	ErrUserBlocked    = errors.New("service.auth: user is blocked")
	ErrInvalidRefresh = errors.New("service.auth: invalid or expired refresh token")
	ErrWeakPassword   = errors.New("service.auth: password must be at least 8 characters")
	ErrInvalidEmail   = errors.New("service.auth: invalid email")
)

// Tokens is what callers receive after Login or Refresh.
type Tokens struct {
	AccessToken      string
	RefreshTokenRaw  string
	RefreshExpiresAt time.Time
}

// AuthService implements user registration, login, refresh-token rotation, and logout.
type AuthService struct {
	users      repository.UserRepo
	tokens     repository.RefreshTokenRepo
	jwt        config.JWTConfig
	bcryptCost int
}

// NewAuthService wires the auth service with its dependencies.
func NewAuthService(users repository.UserRepo, tokens repository.RefreshTokenRepo, jwtCfg config.JWTConfig) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		jwt:        jwtCfg,
		bcryptCost: bcrypt.DefaultCost + 2, // cost 12
	}
}

// Register validates the input, bcrypt-hashes the password, and creates a user.
// Returns ErrUserExists when the email is already taken.
func (s *AuthService) Register(ctx context.Context, email, password string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !looksLikeEmail(email) {
		return domain.User{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return domain.User{}, ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("service.auth.Register: bcrypt: %w", err)
	}

	user, err := s.users.Create(ctx, email, string(hash))
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return domain.User{}, ErrUserExists
		}
		return domain.User{}, fmt.Errorf("service.auth.Register: %w", err)
	}
	return user, nil
}

// Login verifies credentials and returns the user plus a fresh access + refresh pair.
// The password field on the returned user is always blank.
func (s *AuthService) Login(ctx context.Context, email, password string) (domain.User, Tokens, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.User{}, Tokens{}, ErrInvalidCreds
		}
		return domain.User{}, Tokens{}, fmt.Errorf("service.auth.Login: %w", err)
	}
	if user.IsBlocked {
		return domain.User{}, Tokens{}, ErrUserBlocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return domain.User{}, Tokens{}, ErrInvalidCreds
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return domain.User{}, Tokens{}, err
	}
	user.Password = ""
	return user, tokens, nil
}

// Refresh rotates a valid refresh token: it deletes the used row and issues a new pair.
func (s *AuthService) Refresh(ctx context.Context, rawRefresh string) (domain.User, Tokens, error) {
	if rawRefresh == "" {
		return domain.User{}, Tokens{}, ErrInvalidRefresh
	}
	hash := sha256Hex(rawRefresh)

	rec, err := s.tokens.Get(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.User{}, Tokens{}, ErrInvalidRefresh
		}
		return domain.User{}, Tokens{}, fmt.Errorf("service.auth.Refresh: %w", err)
	}

	user, err := s.users.GetByID(ctx, rec.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.User{}, Tokens{}, ErrInvalidRefresh
		}
		return domain.User{}, Tokens{}, fmt.Errorf("service.auth.Refresh: %w", err)
	}
	if user.IsBlocked {
		return domain.User{}, Tokens{}, ErrUserBlocked
	}

	if err := s.tokens.Delete(ctx, hash); err != nil {
		return domain.User{}, Tokens{}, fmt.Errorf("service.auth.Refresh: %w", err)
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return domain.User{}, Tokens{}, err
	}
	return user, tokens, nil
}

// Logout removes the refresh-token row matching the given raw token.
// Missing rows are not an error (idempotent).
func (s *AuthService) Logout(ctx context.Context, rawRefresh string) error {
	if rawRefresh == "" {
		return nil
	}
	if err := s.tokens.Delete(ctx, sha256Hex(rawRefresh)); err != nil {
		return fmt.Errorf("service.auth.Logout: %w", err)
	}
	return nil
}

func (s *AuthService) issueTokens(ctx context.Context, user domain.User) (Tokens, error) {
	access, err := jwtpkg.Generate(user.ID, user.Role, s.jwt.AccessTTL, s.jwt.Secret)
	if err != nil {
		return Tokens{}, fmt.Errorf("service.auth.issueTokens: access: %w", err)
	}

	raw, err := generateRefresh()
	if err != nil {
		return Tokens{}, fmt.Errorf("service.auth.issueTokens: refresh: %w", err)
	}
	expiresAt := time.Now().Add(s.jwt.RefreshTTL)
	if err := s.tokens.Create(ctx, user.ID, sha256Hex(raw), expiresAt); err != nil {
		return Tokens{}, fmt.Errorf("service.auth.issueTokens: store refresh: %w", err)
	}
	return Tokens{
		AccessToken:      access,
		RefreshTokenRaw:  raw,
		RefreshExpiresAt: expiresAt,
	}, nil
}

func generateRefresh() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// looksLikeEmail is a deliberately lightweight check — full RFC 5322 is overkill.
func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.IndexByte(s[at+1:], '.') < 0 {
		return false
	}
	return true
}
