package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// AccountRepo is the storage contract for connected platform accounts.
type AccountRepo interface {
	Create(ctx context.Context, p CreateAccountParams) (domain.ConnectedAccount, error)
	GetByID(ctx context.Context, id string) (domain.ConnectedAccount, error)
	GetByPageID(ctx context.Context, platform, pageOrPlatformID string) (domain.ConnectedAccount, error)
	ListByUser(ctx context.Context, userID string) ([]domain.ConnectedAccount, error)
	ListAllActive(ctx context.Context) ([]domain.ConnectedAccount, error)
	ListIGNearExpiry(ctx context.Context, daysAhead int) ([]domain.ConnectedAccount, error)
	CountActiveByUser(ctx context.Context, userID string) (int64, error)
	Delete(ctx context.Context, id string) error
	SetStatus(ctx context.Context, id, status, message string) error
	SetActive(ctx context.Context, id string, active bool, status string) error
	UpdateToken(ctx context.Context, id, accessToken string, expiresAt *time.Time) error
}

// CreateAccountParams is the input to AccountRepo.Create.
type CreateAccountParams struct {
	UserID         string
	Platform       string
	PlatformID     string
	DisplayName    *string
	AvatarURL      *string
	AccessToken    string
	TokenExpiresAt *time.Time
	PageID         *string
	Extra          json.RawMessage
}

type pgAccountRepo struct {
	q *generated.Queries
}

// NewAccountRepo returns a pgx-backed AccountRepo.
func NewAccountRepo(q *generated.Queries) AccountRepo {
	return &pgAccountRepo{q: q}
}

func (r *pgAccountRepo) Create(ctx context.Context, p CreateAccountParams) (domain.ConnectedAccount, error) {
	userUID, err := uuidFromString(p.UserID)
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("repository.account.Create: %w", err)
	}
	var expiresAt pgtype.Timestamptz
	if p.TokenExpiresAt != nil {
		expiresAt = tsFromTime(*p.TokenExpiresAt)
	}
	extra := []byte(p.Extra)
	if len(extra) == 0 {
		extra = []byte("{}")
	}
	row, err := r.q.CreateConnectedAccount(ctx, generated.CreateConnectedAccountParams{
		UserID:         userUID,
		Platform:       p.Platform,
		PlatformID:     p.PlatformID,
		DisplayName:    p.DisplayName,
		AvatarUrl:      p.AvatarURL,
		AccessToken:    p.AccessToken,
		TokenExpiresAt: expiresAt,
		PageID:         p.PageID,
		Extra:          extra,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ConnectedAccount{}, ErrConflict
		}
		return domain.ConnectedAccount{}, fmt.Errorf("repository.account.Create: %w", err)
	}
	return rowToAccount(row), nil
}

func (r *pgAccountRepo) GetByID(ctx context.Context, id string) (domain.ConnectedAccount, error) {
	uid, err := uuidFromString(id)
	if err != nil {
		return domain.ConnectedAccount{}, ErrNotFound
	}
	row, err := r.q.GetAccountByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ConnectedAccount{}, ErrNotFound
		}
		return domain.ConnectedAccount{}, fmt.Errorf("repository.account.GetByID: %w", err)
	}
	return rowToAccount(row), nil
}

func (r *pgAccountRepo) GetByPageID(ctx context.Context, platform, pageOrPlatformID string) (domain.ConnectedAccount, error) {
	row, err := r.q.GetAccountByPageID(ctx, generated.GetAccountByPageIDParams{
		Platform: platform,
		PageID:   &pageOrPlatformID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ConnectedAccount{}, ErrNotFound
		}
		return domain.ConnectedAccount{}, fmt.Errorf("repository.account.GetByPageID: %w", err)
	}
	return rowToAccount(row), nil
}

func (r *pgAccountRepo) ListByUser(ctx context.Context, userID string) ([]domain.ConnectedAccount, error) {
	uid, err := uuidFromString(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.ListAccountsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("repository.account.ListByUser: %w", err)
	}
	out := make([]domain.ConnectedAccount, len(rows))
	for i, row := range rows {
		out[i] = rowToAccount(row)
	}
	return out, nil
}

func (r *pgAccountRepo) ListAllActive(ctx context.Context) ([]domain.ConnectedAccount, error) {
	rows, err := r.q.ListActiveAccountsAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.account.ListAllActive: %w", err)
	}
	out := make([]domain.ConnectedAccount, len(rows))
	for i, row := range rows {
		out[i] = rowToAccount(row)
	}
	return out, nil
}

func (r *pgAccountRepo) ListIGNearExpiry(ctx context.Context, daysAhead int) ([]domain.ConnectedAccount, error) {
	rows, err := r.q.ListIGAccountsNearExpiry(ctx, int32(daysAhead))
	if err != nil {
		return nil, fmt.Errorf("repository.account.ListIGNearExpiry: %w", err)
	}
	out := make([]domain.ConnectedAccount, len(rows))
	for i, row := range rows {
		out[i] = rowToAccount(row)
	}
	return out, nil
}

func (r *pgAccountRepo) CountActiveByUser(ctx context.Context, userID string) (int64, error) {
	uid, err := uuidFromString(userID)
	if err != nil {
		return 0, ErrNotFound
	}
	return r.q.CountActiveAccountsByUser(ctx, uid)
}

func (r *pgAccountRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.DeleteAccount(ctx, uid); err != nil {
		return fmt.Errorf("repository.account.Delete: %w", err)
	}
	return nil
}

func (r *pgAccountRepo) SetStatus(ctx context.Context, id, status, message string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	var msg *string
	if message != "" {
		msg = &message
	}
	if err := r.q.SetAccountStatus(ctx, generated.SetAccountStatusParams{
		ID:            uid,
		Status:        status,
		StatusMessage: msg,
	}); err != nil {
		return fmt.Errorf("repository.account.SetStatus: %w", err)
	}
	return nil
}

func (r *pgAccountRepo) SetActive(ctx context.Context, id string, active bool, status string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.SetAccountActive(ctx, generated.SetAccountActiveParams{
		ID:       uid,
		IsActive: active,
		Status:   status,
	}); err != nil {
		return fmt.Errorf("repository.account.SetActive: %w", err)
	}
	return nil
}

func (r *pgAccountRepo) UpdateToken(ctx context.Context, id, accessToken string, expiresAt *time.Time) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	var ts pgtype.Timestamptz
	if expiresAt != nil {
		ts = tsFromTime(*expiresAt)
	}
	if err := r.q.UpdateAccountToken(ctx, generated.UpdateAccountTokenParams{
		ID:             uid,
		AccessToken:    accessToken,
		TokenExpiresAt: ts,
	}); err != nil {
		return fmt.Errorf("repository.account.UpdateToken: %w", err)
	}
	return nil
}

func rowToAccount(row generated.ConnectedAccount) domain.ConnectedAccount {
	a := domain.ConnectedAccount{
		ID:            uuidToString(row.ID),
		UserID:        uuidToString(row.UserID),
		Platform:      row.Platform,
		PlatformID:    row.PlatformID,
		DisplayName:   row.DisplayName,
		AvatarURL:     row.AvatarUrl,
		AccessToken:   row.AccessToken,
		PageID:        row.PageID,
		IsActive:      row.IsActive,
		Status:        row.Status,
		StatusMessage: row.StatusMessage,
		CreatedAt:     timeFromTs(row.CreatedAt),
		UpdatedAt:     timeFromTs(row.UpdatedAt),
	}
	if row.TokenExpiresAt.Valid {
		t := row.TokenExpiresAt.Time
		a.TokenExpiresAt = &t
	}
	if len(row.Extra) > 0 {
		a.Extra = json.RawMessage(row.Extra)
	}
	return a
}
