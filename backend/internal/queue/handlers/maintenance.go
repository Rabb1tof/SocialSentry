package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

const (
	// TaskRefreshIGTokens is the Asynq task type fired by the daily scheduler to refresh
	// Instagram Page Access Tokens that are within 10 days of expiry.
	TaskRefreshIGTokens = "instagram:refresh_tokens"
	// TaskLogRetention deletes trigger_logs older than the per-plan retention window.
	TaskLogRetention = "logs:retention_cleanup"

	// refreshWindowDays is how far ahead we look. Page tokens live ~60 days; refreshing
	// at 50 leaves ample buffer for one missed run.
	refreshWindowDays = 10
)

// TokenEncDec is the small interface MaintenanceHandler needs from the account service:
// decrypt the ciphertext token for the platform call, and encrypt the new one before persist.
type TokenEncDec interface {
	DecryptToken(encoded string) (string, error)
	EncryptToken(plaintext string) (string, error)
}

// MaintenanceHandler bundles both scheduled jobs into one type so cmd/worker can register
// them with a single constructor call.
type MaintenanceHandler struct {
	accounts repository.AccountRepo
	queries  *generated.Queries // for DeleteLogsOlderThan + UpdateAccountToken
	encdec   TokenEncDec
	ig       *instagram.Client
	cfg      config.MetaConfig
	logger   *zap.Logger
}

// NewMaintenanceHandler wires the handler.
func NewMaintenanceHandler(
	accounts repository.AccountRepo,
	queries *generated.Queries,
	encdec TokenEncDec,
	ig *instagram.Client,
	cfg config.MetaConfig,
	logger *zap.Logger,
) *MaintenanceHandler {
	return &MaintenanceHandler{
		accounts: accounts,
		queries:  queries,
		encdec:   encdec,
		ig:       ig,
		cfg:      cfg,
		logger:   logger,
	}
}

// RefreshIGTokens scans for Instagram accounts whose token expires within 10 days and
// extends each via Meta's fb_exchange_token flow. Failures are logged per account but
// do not fail the task (one bad token shouldn't abort the whole batch).
func (h *MaintenanceHandler) RefreshIGTokens(ctx context.Context, _ *asynq.Task) error {
	if h.cfg.AppID == "" || h.cfg.AppSecret == "" {
		h.logger.Warn("maintenance.RefreshIGTokens: META_APP_ID / META_APP_SECRET not configured, skipping")
		return nil
	}

	accounts, err := h.accounts.ListIGNearExpiry(ctx, refreshWindowDays)
	if err != nil {
		return fmt.Errorf("maintenance.RefreshIGTokens: list: %w", err)
	}
	h.logger.Info("ig token refresh: scanning",
		zap.Int("candidates", len(accounts)),
		zap.Int("window_days", refreshWindowDays),
	)

	var refreshed, failed int
	for _, a := range accounts {
		if err := h.refreshOne(ctx, a); err != nil {
			h.logger.Error("ig token refresh: account failed",
				zap.Error(err),
				zap.String("account_id", a.ID),
			)
			failed++
			continue
		}
		refreshed++
	}
	h.logger.Info("ig token refresh: done",
		zap.Int("refreshed", refreshed),
		zap.Int("failed", failed),
	)
	return nil
}

func (h *MaintenanceHandler) refreshOne(ctx context.Context, a domain.ConnectedAccount) error {
	currentTok, err := h.encdec.DecryptToken(a.AccessToken)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	newTok, expires, err := h.ig.RefreshPageToken(ctx, h.cfg.AppID, h.cfg.AppSecret, currentTok)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	if newTok == "" {
		return fmt.Errorf("refresh: empty token returned")
	}
	encNew, err := h.encdec.EncryptToken(newTok)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	if err := h.accounts.UpdateToken(ctx, a.ID, encNew, ptrTime(expires)); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// LogRetention deletes trigger_logs older than the per-plan retention window
// (basic=7d, pro=30d, enterprise=90d). Runs each plan in turn so failures on one
// don't block the others.
func (h *MaintenanceHandler) LogRetention(ctx context.Context, _ *asynq.Task) error {
	plans := []struct {
		Name string
		Days int32
	}{
		{Name: "basic", Days: 7},
		{Name: "pro", Days: 30},
		{Name: "enterprise", Days: 90},
	}
	var totalErrors int
	for _, p := range plans {
		if err := h.queries.DeleteLogsOlderThan(ctx, generated.DeleteLogsOlderThanParams{
			Plan:    p.Name,
			Column2: p.Days,
		}); err != nil {
			h.logger.Error("log retention: delete failed",
				zap.Error(err),
				zap.String("plan", p.Name),
				zap.Int32("days", p.Days),
			)
			totalErrors++
			continue
		}
		h.logger.Info("log retention: trimmed",
			zap.String("plan", p.Name),
			zap.Int32("days", p.Days),
		)
	}
	if totalErrors > 0 {
		return fmt.Errorf("log retention: %d plan(s) failed", totalErrors)
	}
	return nil
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
