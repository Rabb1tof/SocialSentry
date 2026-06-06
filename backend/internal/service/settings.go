package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// ErrInvalidPlatform is returned when a platform outside {instagram, vk} is supplied.
var ErrInvalidPlatform = errors.New("service.settings: invalid platform")

// PlatformStatePublisher notifies other processes that a platform's enabled flag changed.
// Implemented by queue.Publisher. Kept as an interface so the service has no queue import.
type PlatformStatePublisher interface {
	PublishPlatformState(ctx context.Context, platform string)
}

// SettingsService reads and mutates the global per-platform on/off flags.
//
// The DB row is the source of truth; a short-lived Redis cache (key
// "settings:platform:<platform>") keeps the per-event IsEnabled reads cheap and lets the
// flag propagate across the api and worker processes. Mutations DEL the cache and publish
// a pub/sub message so the VK WorkerManager can react immediately.
type SettingsService struct {
	q        *generated.Queries
	rdb      *redis.Client
	pub      PlatformStatePublisher
	logger   *zap.Logger
	cacheTTL time.Duration
}

// NewSettingsService wires the service. pub may be nil (publishes become no-ops).
func NewSettingsService(q *generated.Queries, rdb *redis.Client, pub PlatformStatePublisher, logger *zap.Logger) *SettingsService {
	return &SettingsService{
		q:        q,
		rdb:      rdb,
		pub:      pub,
		logger:   logger,
		cacheTTL: 60 * time.Second,
	}
}

func settingsCacheKey(platform string) string {
	return "settings:platform:" + platform
}

// List returns the stored on/off state for every platform, ordered by name.
func (s *SettingsService) List(ctx context.Context) ([]domain.PlatformSetting, error) {
	rows, err := s.q.ListPlatformSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.settings.List: %w", err)
	}
	out := make([]domain.PlatformSetting, len(rows))
	for i, r := range rows {
		ps := domain.PlatformSetting{Platform: r.Platform, Enabled: r.Enabled}
		if r.UpdatedAt.Valid {
			ps.UpdatedAt = r.UpdatedAt.Time
		}
		out[i] = ps
	}
	return out, nil
}

// IsEnabled reports whether the platform is globally enabled. A missing row defaults to
// enabled (fail-open) so a fresh DB never silently disables everything. Reads hit Redis
// first; on a cache miss it falls back to the DB and re-caches the result.
func (s *SettingsService) IsEnabled(ctx context.Context, platform string) (bool, error) {
	key := settingsCacheKey(platform)
	if s.rdb != nil {
		switch v, err := s.rdb.Get(ctx, key).Result(); {
		case err == nil:
			return v == "1", nil
		case errors.Is(err, redis.Nil):
			// cache miss — fall through to DB
		default:
			s.logger.Warn("settings cache get", zap.Error(err), zap.String("platform", platform))
		}
	}

	enabled, err := s.readDB(ctx, platform)
	if err != nil {
		return false, err
	}
	if s.rdb != nil {
		val := "0"
		if enabled {
			val = "1"
		}
		if err := s.rdb.Set(ctx, key, val, s.cacheTTL).Err(); err != nil {
			s.logger.Warn("settings cache set", zap.Error(err), zap.String("platform", platform))
		}
	}
	return enabled, nil
}

func (s *SettingsService) readDB(ctx context.Context, platform string) (bool, error) {
	row, err := s.q.GetPlatformSetting(ctx, platform)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil // no row -> default enabled
		}
		return false, fmt.Errorf("service.settings.IsEnabled: %w", err)
	}
	return row.Enabled, nil
}

// SetEnabled flips a platform's flag, invalidates the cache, and publishes the change.
func (s *SettingsService) SetEnabled(ctx context.Context, platform string, enabled bool) (domain.PlatformSetting, error) {
	if platform != domain.PlatformInstagram && platform != domain.PlatformVK {
		return domain.PlatformSetting{}, ErrInvalidPlatform
	}
	row, err := s.q.SetPlatformEnabled(ctx, generated.SetPlatformEnabledParams{
		Platform: platform,
		Enabled:  enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PlatformSetting{}, ErrInvalidPlatform
		}
		return domain.PlatformSetting{}, fmt.Errorf("service.settings.SetEnabled: %w", err)
	}

	if s.rdb != nil {
		if err := s.rdb.Del(ctx, settingsCacheKey(platform)).Err(); err != nil {
			s.logger.Warn("settings cache del", zap.Error(err), zap.String("platform", platform))
		}
	}
	if s.pub != nil {
		s.pub.PublishPlatformState(ctx, platform)
	}

	ps := domain.PlatformSetting{Platform: row.Platform, Enabled: row.Enabled}
	if row.UpdatedAt.Valid {
		ps.UpdatedAt = row.UpdatedAt.Time
	}
	return ps, nil
}
