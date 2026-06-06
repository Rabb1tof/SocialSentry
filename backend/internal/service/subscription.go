package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

var (
	ErrSubscriptionNotFound = errors.New("service.subscription: not found")
	ErrInvalidPlan          = errors.New("service.subscription: invalid plan")
)

var validPlans = map[string]bool{
	domain.PlanBasic:      true,
	domain.PlanPro:        true,
	domain.PlanEnterprise: true,
}

// SubscriptionService handles subscription lifecycle operations.
type SubscriptionService struct {
	subs  repository.SubscriptionRepo
	users repository.UserRepo
}

// NewSubscriptionService wires the service.
func NewSubscriptionService(subs repository.SubscriptionRepo, users repository.UserRepo) *SubscriptionService {
	return &SubscriptionService{subs: subs, users: users}
}

// GetActive returns the caller's active subscription, or ErrSubscriptionNotFound.
func (s *SubscriptionService) GetActive(ctx context.Context, userID string) (domain.Subscription, error) {
	sub, err := s.subs.GetActive(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Subscription{}, ErrSubscriptionNotFound
		}
		return domain.Subscription{}, fmt.Errorf("service.subscription.GetActive: %w", err)
	}
	return sub, nil
}

// GrantParams is the input for granting a subscription.
type GrantParams struct {
	UserID    string
	Plan      string
	ExpiresAt *time.Time
	Note      *string
	GrantedBy string
}

// Grant creates a new active subscription for a user (admin only).
// Any existing active subscriptions for the user are deactivated first.
func (s *SubscriptionService) Grant(ctx context.Context, p GrantParams) (domain.Subscription, error) {
	if !validPlans[p.Plan] {
		return domain.Subscription{}, ErrInvalidPlan
	}
	if _, err := s.users.GetByID(ctx, p.UserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Subscription{}, ErrSubscriptionNotFound
		}
		return domain.Subscription{}, fmt.Errorf("service.subscription.Grant: lookup user: %w", err)
	}

	// Deactivate existing active subs so only one is active at a time.
	if err := s.subs.DeactivateAllForUser(ctx, p.UserID); err != nil {
		return domain.Subscription{}, fmt.Errorf("service.subscription.Grant: deactivate old: %w", err)
	}

	sub, err := s.subs.Create(ctx, repository.CreateSubscriptionParams{
		UserID:    p.UserID,
		Plan:      p.Plan,
		IsActive:  true,
		StartsAt:  time.Now(),
		ExpiresAt: p.ExpiresAt,
		Note:      p.Note,
		GrantedBy: p.GrantedBy,
	})
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("service.subscription.Grant: %w", err)
	}
	return sub, nil
}

// UpdateParams is the input for modifying a subscription.
type UpdateParams struct {
	ID        string
	Plan      string
	IsActive  bool
	ExpiresAt *time.Time
	Note      *string
}

// Update modifies plan/status/expiry of an existing subscription.
func (s *SubscriptionService) Update(ctx context.Context, p UpdateParams) (domain.Subscription, error) {
	if !validPlans[p.Plan] {
		return domain.Subscription{}, ErrInvalidPlan
	}
	sub, err := s.subs.Update(ctx, repository.UpdateSubscriptionParams{
		ID:        p.ID,
		Plan:      p.Plan,
		IsActive:  p.IsActive,
		ExpiresAt: p.ExpiresAt,
		Note:      p.Note,
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Subscription{}, ErrSubscriptionNotFound
		}
		return domain.Subscription{}, fmt.Errorf("service.subscription.Update: %w", err)
	}
	return sub, nil
}

// Revoke deactivates a subscription immediately.
func (s *SubscriptionService) Revoke(ctx context.Context, id string) error {
	if err := s.subs.Deactivate(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("service.subscription.Revoke: %w", err)
	}
	return nil
}

// List returns a paginated list of all subscriptions.
func (s *SubscriptionService) List(ctx context.Context, limit, offset int) ([]domain.Subscription, int64, error) {
	subs, total, err := s.subs.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("service.subscription.List: %w", err)
	}
	return subs, total, nil
}
