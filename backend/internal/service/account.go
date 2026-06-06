package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/pkg/crypto"
)

var (
	// ErrAccountLimitExceeded is returned when the user has reached the account cap of their plan.
	ErrAccountLimitExceeded = errors.New("service.account: account limit exceeded for plan")
	// ErrAccountPlatformNotAllowed is returned when the basic plan tries to mix platforms.
	ErrAccountPlatformNotAllowed = errors.New("service.account: platform not allowed by plan")
)

// WorkerLifecyclePublisher notifies the worker process that an account's runtime state
// changed (created/started/stopped). Implementations should be non-blocking.
type WorkerLifecyclePublisher interface {
	PublishWorkerStart(ctx context.Context, accountID string)
	PublishWorkerStop(ctx context.Context, accountID string)
}

// AccountService handles connected-account lifecycle operations.
type AccountService struct {
	accounts repository.AccountRepo
	subs     repository.SubscriptionRepo
	encKey   []byte
	pub      WorkerLifecyclePublisher
}

// NewAccountService wires the service. pub may be nil — the worker will fall back
// to its boot-time enumeration of active accounts.
func NewAccountService(accounts repository.AccountRepo, subs repository.SubscriptionRepo, encKey []byte, pub WorkerLifecyclePublisher) *AccountService {
	return &AccountService{accounts: accounts, subs: subs, encKey: encKey, pub: pub}
}

// ListByUser returns all accounts owned by the user.
func (s *AccountService) ListByUser(ctx context.Context, userID string) ([]domain.ConnectedAccount, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.account.ListByUser: %w", err)
	}
	// Never leak the encrypted access token through the API.
	for i := range accounts {
		accounts[i].AccessToken = ""
	}
	return accounts, nil
}

// Get returns a single account if it belongs to the user.
func (s *AccountService) Get(ctx context.Context, userID, accountID string) (domain.ConnectedAccount, error) {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return domain.ConnectedAccount{}, err
	}
	if a.UserID != userID {
		return domain.ConnectedAccount{}, repository.ErrNotFound
	}
	a.AccessToken = ""
	return a, nil
}

// Delete removes an account after verifying ownership.
func (s *AccountService) Delete(ctx context.Context, userID, accountID string) error {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return repository.ErrNotFound
	}
	if err := s.accounts.Delete(ctx, accountID); err != nil {
		return fmt.Errorf("service.account.Delete: %w", err)
	}
	if s.pub != nil {
		s.pub.PublishWorkerStop(ctx, accountID)
	}
	return nil
}

// SetActive flips the active flag (pause / resume).
func (s *AccountService) SetActive(ctx context.Context, userID, accountID string, active bool) error {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return repository.ErrNotFound
	}
	status := domain.AccountStatusRunning
	if !active {
		status = domain.AccountStatusPaused
	}
	if err := s.accounts.SetActive(ctx, accountID, active, status); err != nil {
		return fmt.Errorf("service.account.SetActive: %w", err)
	}
	if s.pub != nil {
		if active {
			s.pub.PublishWorkerStart(ctx, accountID)
		} else {
			s.pub.PublishWorkerStop(ctx, accountID)
		}
	}
	return nil
}

// CreateConnected persists a new account after enforcing plan limits.
// AccessTokenRaw is encrypted with AES-256-GCM using the configured ENCRYPTION_KEY.
// Caller must set p.AccessToken to the *plaintext* token; this method handles encryption.
func (s *AccountService) CreateConnected(ctx context.Context, p repository.CreateAccountParams) (domain.ConnectedAccount, error) {
	if p.AccessToken == "" {
		return domain.ConnectedAccount{}, errors.New("service.account.CreateConnected: access token is required")
	}
	if err := s.checkPlanLimits(ctx, p.UserID, p.Platform); err != nil {
		return domain.ConnectedAccount{}, err
	}
	enc, err := crypto.Encrypt(p.AccessToken, s.encKey)
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("service.account.CreateConnected encrypt: %w", err)
	}
	p.AccessToken = enc
	a, err := s.accounts.Create(ctx, p)
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("service.account.CreateConnected: %w", err)
	}
	a.AccessToken = ""
	if s.pub != nil {
		s.pub.PublishWorkerStart(ctx, a.ID)
	}
	return a, nil
}

// DecryptToken returns the plaintext access token for internal use (workers / platform clients).
// NEVER expose this through the HTTP API.
func (s *AccountService) DecryptToken(encoded string) (string, error) {
	return crypto.Decrypt(encoded, s.encKey)
}

// EncryptToken is the inverse of DecryptToken. Used by the IG token-refresh cron to
// persist a freshly-rotated Page Access Token without going through the API handler.
func (s *AccountService) EncryptToken(plaintext string) (string, error) {
	return crypto.Encrypt(plaintext, s.encKey)
}

// checkPlanLimits enforces account count and platform constraints based on the user's active subscription.
func (s *AccountService) checkPlanLimits(ctx context.Context, userID, platform string) error {
	sub, err := s.subs.GetActive(ctx, userID)
	if err != nil {
		// No active sub at all: middleware should have blocked the request before getting here.
		// Guard anyway so internal callers can't bypass it.
		return ErrAccountLimitExceeded
	}
	limits := PlanLimitsByName(sub.Plan)

	if limits.MaxAccounts > 0 {
		count, err := s.accounts.CountActiveByUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("service.account.checkPlanLimits: %w", err)
		}
		if int(count) >= limits.MaxAccounts {
			return ErrAccountLimitExceeded
		}
	}

	if !contains(limits.AllowedPlatforms, platform) {
		return ErrAccountPlatformNotAllowed
	}

	if !limits.MultiplePlatforms {
		existing, err := s.accounts.ListByUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("service.account.checkPlanLimits: %w", err)
		}
		for _, a := range existing {
			if a.Platform != platform {
				return ErrAccountPlatformNotAllowed
			}
		}
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
