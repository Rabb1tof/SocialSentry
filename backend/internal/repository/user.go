package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// UserRepo is the storage contract for users.
type UserRepo interface {
	// Create persists a new user via the public registration flow. The very first
	// user ever inserted is promoted to admin so the system has a bootstrap operator;
	// subsequent users get the default 'user' role.
	Create(ctx context.Context, email, passwordHash string) (domain.User, error)
	// CreateWithRole persists a new user with an explicit role. Used by the admin
	// create-user endpoint; the caller is responsible for authorizing the operation.
	CreateWithRole(ctx context.Context, email, passwordHash, role string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	UpdateRole(ctx context.Context, id, role string) error
	SetBlocked(ctx context.Context, id string, blocked bool) error
	// UpdateEmail changes a user's email. Returns ErrConflict if the new email
	// is already taken by another account.
	UpdateEmail(ctx context.Context, id, email string) error
	// UpdatePassword sets a new (already-hashed) password for a user.
	UpdatePassword(ctx context.Context, id, passwordHash string) error
}

type pgUserRepo struct {
	q *generated.Queries
}

// NewUserRepo returns a pgx-backed UserRepo.
func NewUserRepo(q *generated.Queries) UserRepo {
	return &pgUserRepo{q: q}
}

func (r *pgUserRepo) Create(ctx context.Context, email, passwordHash string) (domain.User, error) {
	row, err := r.q.CreateUserAutoRole(ctx, generated.CreateUserAutoRoleParams{
		Email:    email,
		Password: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("repository.user.Create: %w", err)
	}
	return domain.User{
		ID:        uuidToString(row.ID),
		Email:     row.Email,
		Role:      row.Role,
		IsBlocked: row.IsBlocked,
		CreatedAt: timeFromTs(row.CreatedAt),
		UpdatedAt: timeFromTs(row.UpdatedAt),
	}, nil
}

func (r *pgUserRepo) CreateWithRole(ctx context.Context, email, passwordHash, role string) (domain.User, error) {
	row, err := r.q.CreateUserAsAdmin(ctx, generated.CreateUserAsAdminParams{
		Email:    email,
		Password: passwordHash,
		Role:     role,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("repository.user.CreateWithRole: %w", err)
	}
	return domain.User{
		ID:        uuidToString(row.ID),
		Email:     row.Email,
		Role:      row.Role,
		IsBlocked: row.IsBlocked,
		CreatedAt: timeFromTs(row.CreatedAt),
		UpdatedAt: timeFromTs(row.UpdatedAt),
	}, nil
}

func (r *pgUserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("repository.user.GetByEmail: %w", err)
	}
	return domain.User{
		ID:        uuidToString(row.ID),
		Email:     row.Email,
		Password:  row.Password,
		Role:      row.Role,
		IsBlocked: row.IsBlocked,
		CreatedAt: timeFromTs(row.CreatedAt),
		UpdatedAt: timeFromTs(row.UpdatedAt),
	}, nil
}

func (r *pgUserRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	uid, err := uuidFromString(id)
	if err != nil {
		return domain.User{}, ErrNotFound
	}
	row, err := r.q.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("repository.user.GetByID: %w", err)
	}
	return domain.User{
		ID:        uuidToString(row.ID),
		Email:     row.Email,
		Role:      row.Role,
		IsBlocked: row.IsBlocked,
		CreatedAt: timeFromTs(row.CreatedAt),
		UpdatedAt: timeFromTs(row.UpdatedAt),
	}, nil
}

func (r *pgUserRepo) UpdateRole(ctx context.Context, id, role string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.UpdateUserRole(ctx, generated.UpdateUserRoleParams{
		ID:   uid,
		Role: role,
	}); err != nil {
		return fmt.Errorf("repository.user.UpdateRole: %w", err)
	}
	return nil
}

func (r *pgUserRepo) SetBlocked(ctx context.Context, id string, blocked bool) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.SetUserBlocked(ctx, generated.SetUserBlockedParams{
		ID:        uid,
		IsBlocked: blocked,
	}); err != nil {
		return fmt.Errorf("repository.user.SetBlocked: %w", err)
	}
	return nil
}

func (r *pgUserRepo) UpdateEmail(ctx context.Context, id, email string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.UpdateUserEmail(ctx, generated.UpdateUserEmailParams{
		ID:    uid,
		Email: email,
	}); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("repository.user.UpdateEmail: %w", err)
	}
	return nil
}

func (r *pgUserRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.UpdateUserPassword(ctx, generated.UpdateUserPasswordParams{
		ID:       uid,
		Password: passwordHash,
	}); err != nil {
		return fmt.Errorf("repository.user.UpdatePassword: %w", err)
	}
	return nil
}
