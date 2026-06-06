package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// Admin user-creation sentinel errors. All wrap ErrAdminUserValidation so the
// HTTP handler can collapse them to a single 400 case. Names are prefixed with
// "AdminCreate" to avoid clashing with the auth-side ErrInvalidEmail.
var (
	ErrAdminUserValidation     = errors.New("service.admin.CreateUser: validation error")
	ErrAdminCreateInvalidEmail = fmt.Errorf("%w: invalid email", ErrAdminUserValidation)
	ErrAdminCreatePasswordWeak = fmt.Errorf("%w: password must be at least 8 characters", ErrAdminUserValidation)
	ErrAdminCreateInvalidRole  = fmt.Errorf("%w: role must be 'user' or 'admin'", ErrAdminUserValidation)
)

// AdminService handles admin-only operations on users.
type AdminService struct {
	users repository.UserRepo
	q     *generated.Queries
}

// NewAdminService wires the service.
func NewAdminService(users repository.UserRepo, q *generated.Queries) *AdminService {
	return &AdminService{users: users, q: q}
}

// ListUsers returns a paginated list of all users.
func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]domain.User, int64, error) {
	rows, err := s.q.ListUsers(ctx, generated.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("service.admin.ListUsers: %w", err)
	}
	total, err := s.q.CountUsers(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("service.admin.ListUsers count: %w", err)
	}
	users := make([]domain.User, len(rows))
	for i, r := range rows {
		u := domain.User{
			ID:        uuid.UUID(r.ID.Bytes).String(),
			Email:     r.Email,
			Role:      r.Role,
			IsBlocked: r.IsBlocked,
		}
		if r.CreatedAt.Valid {
			u.CreatedAt = r.CreatedAt.Time
		}
		if r.UpdatedAt.Valid {
			u.UpdatedAt = r.UpdatedAt.Time
		}
		users[i] = u
	}
	return users, total, nil
}

// SetRole updates a user's role (user → admin or admin → user).
func (s *AdminService) SetRole(ctx context.Context, userID, role string) error {
	if role != domain.RoleUser && role != domain.RoleAdmin {
		return errors.New("service.admin.SetRole: invalid role")
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Role == role {
		return nil
	}
	if err := s.users.UpdateRole(ctx, userID, role); err != nil {
		return fmt.Errorf("service.admin.SetRole: %w", err)
	}
	return nil
}

// SetBlocked blocks or unblocks a user account.
func (s *AdminService) SetBlocked(ctx context.Context, userID string, blocked bool) error {
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}
	if err := s.users.SetBlocked(ctx, userID, blocked); err != nil {
		return fmt.Errorf("service.admin.SetBlocked: %w", err)
	}
	return nil
}

// SetEmail changes a user's email (admin only). Normalises and validates the
// address; surfaces repository.ErrConflict for a duplicate so the handler maps 409.
func (s *AdminService) SetEmail(ctx context.Context, userID, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return ErrAdminCreateInvalidEmail
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}
	if err := s.users.UpdateEmail(ctx, userID, email); err != nil {
		return fmt.Errorf("service.admin.SetEmail: %w", err)
	}
	return nil
}

// SetPassword sets a new password for a user (admin only). Validates length and
// hashes with bcrypt at cost 12, matching CreateUser.
func (s *AdminService) SetPassword(ctx context.Context, userID, password string) error {
	if len(password) < 8 {
		return ErrAdminCreatePasswordWeak
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("service.admin.SetPassword: hash: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("service.admin.SetPassword: %w", err)
	}
	return nil
}

// CreateUser provisions a new account with the specified role on behalf of an admin.
// The caller is responsible for authorizing the request (the admin check happens at
// the middleware layer). Validates email + password length + role, hashes the
// password with bcrypt at cost 12, and surfaces repository.ErrConflict for duplicate
// emails so the handler can map that to 409.
func (s *AdminService) CreateUser(ctx context.Context, email, password, role string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return domain.User{}, ErrAdminCreateInvalidEmail
	}
	if len(password) < 8 {
		return domain.User{}, ErrAdminCreatePasswordWeak
	}
	if role != domain.RoleUser && role != domain.RoleAdmin {
		return domain.User{}, ErrAdminCreateInvalidRole
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return domain.User{}, fmt.Errorf("service.admin.CreateUser: hash: %w", err)
	}
	u, err := s.users.CreateWithRole(ctx, email, string(hash), role)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// GetStats returns platform-level counters.
func (s *AdminService) GetStats(ctx context.Context) (generated.GetStatsRow, error) {
	stats, err := s.q.GetStats(ctx)
	if err != nil {
		return generated.GetStatsRow{}, fmt.Errorf("service.admin.GetStats: %w", err)
	}
	return stats, nil
}
