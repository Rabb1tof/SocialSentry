package vk

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// Redis pub/sub channels — kept here so the API publishes against the same names the worker subscribes to.
const (
	ChannelWorkerStart    = "worker:start"
	ChannelWorkerStop     = "worker:stop"
	ChannelTriggersReload = "triggers:reload"
	ChannelPlatformState  = "platform:state"
)

// WorkerManager spawns one VK AccountWorker goroutine per active VK account
// and reacts to Redis pub/sub messages from the API to start/stop them on the fly.
type WorkerManager struct {
	accounts   repository.AccountRepo
	dispatcher *Dispatcher
	matcher    cacheInvalidator
	settings   platformGate
	rdb        *redis.Client
	logger     *zap.Logger

	registry *workerRegistry
	parentMu sync.Mutex
	parent   context.Context
	cancel   context.CancelFunc
}

// cacheInvalidator is the slice of engine.TriggerMatcher we need (avoid an import cycle).
type cacheInvalidator interface {
	InvalidateCache(accountID string)
}

// platformGate reports whether a platform is globally enabled (admin kill-switch).
// Implemented by service.SettingsService. May be nil (treated as always enabled).
type platformGate interface {
	IsEnabled(ctx context.Context, platform string) (bool, error)
}

// NewWorkerManager wires the manager. settings may be nil (platform always enabled).
func NewWorkerManager(
	accounts repository.AccountRepo,
	dispatcher *Dispatcher,
	matcher cacheInvalidator,
	settings platformGate,
	rdb *redis.Client,
	logger *zap.Logger,
) *WorkerManager {
	return &WorkerManager{
		accounts:   accounts,
		dispatcher: dispatcher,
		matcher:    matcher,
		settings:   settings,
		rdb:        rdb,
		logger:     logger,
		registry:   newWorkerRegistry(),
	}
}

// StartAll launches workers for every currently-active VK account, then subscribes to
// the Redis pub/sub channels for runtime updates. Returns when ctx is cancelled.
func (m *WorkerManager) StartAll(ctx context.Context) error {
	m.parentMu.Lock()
	m.parent, m.cancel = context.WithCancel(ctx)
	parent := m.parent
	m.parentMu.Unlock()

	if err := m.startAllVK(parent); err != nil {
		return fmt.Errorf("vk.WorkerManager.StartAll: %w", err)
	}

	// Subscribe to Redis pub/sub for runtime control. Errors from pubsub are logged but
	// don't kill the manager — the workers spawned at boot continue running.
	go m.runPubSub(parent)

	<-parent.Done()
	m.shutdownAll()
	return nil
}

// startAllVK spawns a worker for every active VK account, unless the VK platform is
// globally disabled (admin kill-switch), in which case none are started. Idempotent:
// spawn skips accounts that already have a running worker.
func (m *WorkerManager) startAllVK(parent context.Context) error {
	if m.settings != nil {
		enabled, err := m.settings.IsEnabled(parent, domain.PlatformVK)
		if err != nil {
			m.logger.Warn("vk: platform settings read failed, assuming enabled", zap.Error(err))
		} else if !enabled {
			m.logger.Info("vk worker manager: platform disabled, no workers started")
			return nil
		}
	}

	all, err := m.accounts.ListAllActive(parent)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	count := 0
	for _, a := range all {
		if a.Platform != domain.PlatformVK {
			continue
		}
		m.spawn(parent, a)
		count++
	}
	m.logger.Info("vk workers started", zap.Int("workers", count))
	return nil
}

// stopAllWorkers terminates every running worker WITHOUT cancelling the manager's parent
// context, so the pub/sub loop survives and workers can be respawned when the platform is
// re-enabled.
func (m *WorkerManager) stopAllWorkers() {
	workers := m.registry.all()
	for _, w := range workers {
		m.stop(w.AccountID())
	}
	m.logger.Info("vk workers stopped (platform disabled)", zap.Int("workers", len(workers)))
}

// Shutdown cancels every running worker and returns once they've all stopped.
func (m *WorkerManager) Shutdown() {
	m.parentMu.Lock()
	cancel := m.cancel
	m.parentMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// spawn (re-)starts a worker for the given account if it isn't already running.
func (m *WorkerManager) spawn(parent context.Context, account domain.ConnectedAccount) {
	if _, ok := m.registry.get(account.ID); ok {
		return
	}
	worker := NewAccountWorker(account, m.dispatcher, m.logger)
	m.registry.put(worker)
	go func() {
		worker.Run(parent)
		// Drop ourselves from the registry when Run returns (e.g. ctx cancel or fatal error).
		m.registry.drop(account.ID)
	}()
}

// stop terminates the worker for the given account_id, if any.
func (m *WorkerManager) stop(accountID string) {
	w, ok := m.registry.drop(accountID)
	if !ok {
		return
	}
	w.Stop()
}

func (m *WorkerManager) shutdownAll() {
	for _, w := range m.registry.all() {
		w.Stop()
	}
	m.logger.Info("vk worker manager stopped")
}

// runPubSub subscribes to worker:start:*, worker:stop:*, triggers:reload:* and dispatches.
// Messages are expected to be "<channel-prefix>:<account_id>".
func (m *WorkerManager) runPubSub(parent context.Context) {
	if m.rdb == nil {
		return
	}
	sub := m.rdb.PSubscribe(parent,
		ChannelWorkerStart+":*",
		ChannelWorkerStop+":*",
		ChannelTriggersReload+":*",
		ChannelPlatformState+":*",
	)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()

	for {
		select {
		case <-parent.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			m.handlePubSub(parent, msg.Channel)
		}
	}
}

func (m *WorkerManager) handlePubSub(parent context.Context, channel string) {
	switch {
	case strings.HasPrefix(channel, ChannelWorkerStart+":"):
		accountID := strings.TrimPrefix(channel, ChannelWorkerStart+":")
		a, err := m.accounts.GetByID(parent, accountID)
		if err != nil {
			m.logger.Warn("vk pubsub start: account lookup", zap.Error(err), zap.String("account_id", accountID))
			return
		}
		if a.Platform != domain.PlatformVK {
			return
		}
		// Don't spawn if the VK platform is globally disabled.
		if m.settings != nil {
			if enabled, err := m.settings.IsEnabled(parent, domain.PlatformVK); err == nil && !enabled {
				return
			}
		}
		m.spawn(parent, a)
	case strings.HasPrefix(channel, ChannelWorkerStop+":"):
		accountID := strings.TrimPrefix(channel, ChannelWorkerStop+":")
		m.stop(accountID)
	case strings.HasPrefix(channel, ChannelTriggersReload+":"):
		accountID := strings.TrimPrefix(channel, ChannelTriggersReload+":")
		if m.matcher != nil {
			m.matcher.InvalidateCache(accountID)
		}
	case strings.HasPrefix(channel, ChannelPlatformState+":"):
		platform := strings.TrimPrefix(channel, ChannelPlatformState+":")
		if platform != domain.PlatformVK || m.settings == nil {
			return
		}
		enabled, err := m.settings.IsEnabled(parent, domain.PlatformVK)
		if err != nil {
			m.logger.Warn("vk pubsub platform state: settings read", zap.Error(err))
			return
		}
		if enabled {
			if err := m.startAllVK(parent); err != nil {
				m.logger.Error("vk pubsub platform state: start all", zap.Error(err))
			}
		} else {
			m.stopAllWorkers()
		}
	}
}
