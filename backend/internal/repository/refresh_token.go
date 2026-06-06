package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// RefreshTokenRepo is the storage contract for refresh tokens.
// The stored Token value is the SHA-256 hex digest of the raw refresh token,
// not the raw token — the raw value never touches the database.
type RefreshTokenRepo interface {
	Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	Get(ctx context.Context, tokenHash string) (domain.RefreshToken, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteAllForUser(ctx context.Context, userID string) error
}

type pgRefreshTokenRepo struct {
	q *generated.Queries
}

// NewRefreshTokenRepo returns a pgx-backed RefreshTokenRepo.
func NewRefreshTokenRepo(q *generated.Queries) RefreshTokenRepo {
	return &pgRefreshTokenRepo{q: q}
}

func (r *pgRefreshTokenRepo) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	uid, err := uuidFromString(userID)
	if err != nil {
		return fmt.Errorf("repository.refresh.Create: %w", err)
	}
	if err := r.q.CreateRefreshToken(ctx, generated.CreateRefreshTokenParams{
		UserID:    uid,
		Token:     tokenHash,
		ExpiresAt: tsFromTime(expiresAt),
	}); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("repository.refresh.Create: %w", err)
	}
	return nil
}

func (r *pgRefreshTokenRepo) Get(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	row, err := r.q.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, ErrNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("repository.refresh.Get: %w", err)
	}
	return domain.RefreshToken{
		ID:        uuidToString(row.ID),
		UserID:    uuidToString(row.UserID),
		Token:     row.Token,
		ExpiresAt: timeFromTs(row.ExpiresAt),
		CreatedAt: timeFromTs(row.CreatedAt),
	}, nil
}

func (r *pgRefreshTokenRepo) Delete(ctx context.Context, tokenHash string) error {
	if err := r.q.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return fmt.Errorf("repository.refresh.Delete: %w", err)
	}
	return nil
}

func (r *pgRefreshTokenRepo) DeleteAllForUser(ctx context.Context, userID string) error {
	uid, err := uuidFromString(userID)
	if err != nil {
		return fmt.Errorf("repository.refresh.DeleteAllForUser: %w", err)
	}
	if err := r.q.DeleteAllRefreshTokensForUser(ctx, uid); err != nil {
		return fmt.Errorf("repository.refresh.DeleteAllForUser: %w", err)
	}
	return nil
}
