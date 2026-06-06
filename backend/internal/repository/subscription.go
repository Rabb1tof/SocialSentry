package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// SubscriptionRepo is the storage contract for subscriptions.
type SubscriptionRepo interface {
	Create(ctx context.Context, p CreateSubscriptionParams) (domain.Subscription, error)
	GetActive(ctx context.Context, userID string) (domain.Subscription, error)
	GetByID(ctx context.Context, id string) (domain.Subscription, error)
	GetByUserID(ctx context.Context, userID string) ([]domain.Subscription, error)
	List(ctx context.Context, limit, offset int) ([]domain.Subscription, int64, error)
	Update(ctx context.Context, p UpdateSubscriptionParams) (domain.Subscription, error)
	Deactivate(ctx context.Context, id string) error
	DeactivateAllForUser(ctx context.Context, userID string) error
}

// CreateSubscriptionParams is the input to SubscriptionRepo.Create.
type CreateSubscriptionParams struct {
	UserID    string
	Plan      string
	IsActive  bool
	StartsAt  time.Time
	ExpiresAt *time.Time
	Note      *string
	GrantedBy string
}

// UpdateSubscriptionParams is the input to SubscriptionRepo.Update.
type UpdateSubscriptionParams struct {
	ID        string
	Plan      string
	IsActive  bool
	ExpiresAt *time.Time
	Note      *string
}

type pgSubscriptionRepo struct {
	q *generated.Queries
}

// NewSubscriptionRepo returns a pgx-backed SubscriptionRepo.
func NewSubscriptionRepo(q *generated.Queries) SubscriptionRepo {
	return &pgSubscriptionRepo{q: q}
}

func (r *pgSubscriptionRepo) Create(ctx context.Context, p CreateSubscriptionParams) (domain.Subscription, error) {
	userUID, err := uuidFromString(p.UserID)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("repository.sub.Create: %w", err)
	}
	grantUID, err := uuidFromString(p.GrantedBy)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("repository.sub.Create granted_by: %w", err)
	}

	var expiresAt pgtype.Timestamptz
	if p.ExpiresAt != nil {
		expiresAt = tsFromTime(*p.ExpiresAt)
	}

	row, err := r.q.CreateSubscription(ctx, generated.CreateSubscriptionParams{
		UserID:    userUID,
		Plan:      p.Plan,
		IsActive:  p.IsActive,
		StartsAt:  tsFromTime(p.StartsAt),
		ExpiresAt: expiresAt,
		Note:      p.Note,
		GrantedBy: grantUID,
	})
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("repository.sub.Create: %w", err)
	}
	return rowToSub(row), nil
}

func (r *pgSubscriptionRepo) GetActive(ctx context.Context, userID string) (domain.Subscription, error) {
	uid, err := uuidFromString(userID)
	if err != nil {
		return domain.Subscription{}, ErrNotFound
	}
	row, err := r.q.GetActiveSubscriptionByUserID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("repository.sub.GetActive: %w", err)
	}
	return rowToSub(row), nil
}

func (r *pgSubscriptionRepo) GetByID(ctx context.Context, id string) (domain.Subscription, error) {
	uid, err := uuidFromString(id)
	if err != nil {
		return domain.Subscription{}, ErrNotFound
	}
	row, err := r.q.GetSubscriptionByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("repository.sub.GetByID: %w", err)
	}
	return rowToSub(row), nil
}

func (r *pgSubscriptionRepo) GetByUserID(ctx context.Context, userID string) ([]domain.Subscription, error) {
	uid, err := uuidFromString(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.GetSubscriptionsByUserID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("repository.sub.GetByUserID: %w", err)
	}
	subs := make([]domain.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = rowToSub(row)
	}
	return subs, nil
}

func (r *pgSubscriptionRepo) List(ctx context.Context, limit, offset int) ([]domain.Subscription, int64, error) {
	rows, err := r.q.ListSubscriptions(ctx, generated.ListSubscriptionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("repository.sub.List: %w", err)
	}
	total, err := r.q.CountSubscriptions(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.sub.List count: %w", err)
	}
	subs := make([]domain.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = rowToSub(row)
	}
	return subs, total, nil
}

func (r *pgSubscriptionRepo) Update(ctx context.Context, p UpdateSubscriptionParams) (domain.Subscription, error) {
	uid, err := uuidFromString(p.ID)
	if err != nil {
		return domain.Subscription{}, ErrNotFound
	}
	var expiresAt pgtype.Timestamptz
	if p.ExpiresAt != nil {
		expiresAt = tsFromTime(*p.ExpiresAt)
	}
	row, err := r.q.UpdateSubscription(ctx, generated.UpdateSubscriptionParams{
		ID:        uid,
		Plan:      p.Plan,
		IsActive:  p.IsActive,
		ExpiresAt: expiresAt,
		Note:      p.Note,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("repository.sub.Update: %w", err)
	}
	return rowToSub(row), nil
}

func (r *pgSubscriptionRepo) Deactivate(ctx context.Context, id string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.DeactivateSubscription(ctx, uid); err != nil {
		return fmt.Errorf("repository.sub.Deactivate: %w", err)
	}
	return nil
}

func (r *pgSubscriptionRepo) DeactivateAllForUser(ctx context.Context, userID string) error {
	uid, err := uuidFromString(userID)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.DeactivateAllUserSubscriptions(ctx, uid); err != nil {
		return fmt.Errorf("repository.sub.DeactivateAllForUser: %w", err)
	}
	return nil
}

func rowToSub(row generated.Subscription) domain.Subscription {
	s := domain.Subscription{
		ID:        uuidToString(row.ID),
		UserID:    uuidToString(row.UserID),
		Plan:      row.Plan,
		IsActive:  row.IsActive,
		StartsAt:  timeFromTs(row.StartsAt),
		CreatedAt: timeFromTs(row.CreatedAt),
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		s.ExpiresAt = &t
	}
	if row.Note != nil {
		s.Note = row.Note
	}
	grantedBy := uuidToString(row.GrantedBy)
	if grantedBy != "" {
		s.GrantedBy = &grantedBy
	}
	return s
}
