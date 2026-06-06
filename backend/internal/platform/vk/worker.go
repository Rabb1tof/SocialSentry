package vk

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/SevereCloud/vksdk/v3/events"
	longpoll "github.com/SevereCloud/vksdk/v3/longpoll-bot"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

const maxWorkerBackoff = 30 * time.Second

// AccountWorker runs the Bots Long Poll for one VK community in a goroutine.
// Cancel its ctx to terminate the loop cleanly.
type AccountWorker struct {
	account    domain.ConnectedAccount
	dispatcher *Dispatcher
	logger     *zap.Logger
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewAccountWorker builds a worker but does NOT start it. Call Run in a goroutine.
func NewAccountWorker(account domain.ConnectedAccount, dispatcher *Dispatcher, logger *zap.Logger) *AccountWorker {
	return &AccountWorker{
		account:    account,
		dispatcher: dispatcher,
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// Run drives the Long Poll loop and keeps it alive across transient failures. It blocks until
// ctx is cancelled (Stop / shutdown / platform-disable) or a fatal auth/access error occurs.
//
// Transient errors (e.g. "context deadline exceeded" on the long-poll HTTP GET, network blips,
// an expired LP key) are recovered: the worker logs a warning, backs off, recreates the long
// poll, and continues — it no longer dies permanently on a single timeout. Only invalid token
// (code 5) / access denied (code 15) are fatal and flip the account into 'error' (the user must
// reconnect — we never auto-restart those).
func (w *AccountWorker) Run(parent context.Context) {
	defer close(w.done)
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	defer cancel()

	token, err := w.dispatcher.decrypt.DecryptToken(w.account.AccessToken)
	if err != nil {
		w.logger.Error("vk worker: decrypt token", zap.Error(err), zap.String("account_id", w.account.ID))
		_ = w.dispatcher.accounts.SetStatus(ctx, w.account.ID, domain.AccountStatusError, "could not decrypt VK token")
		return
	}
	groupID, err := strconv.Atoi(w.account.PlatformID)
	if err != nil {
		w.logger.Error("vk worker: bad group_id", zap.Error(err))
		_ = w.dispatcher.accounts.SetStatus(ctx, w.account.ID, domain.AccountStatusError, "invalid VK group_id")
		return
	}

	client := NewClient(token, groupID, w.account.ID, w.dispatcher.apiVer, nil)
	accountID := w.account.ID
	backoff := time.Second

	for ctx.Err() == nil {
		lp, err := longpoll.NewLongPoll(client.VK, groupID)
		if err != nil {
			if IsAuthError(err) || IsNoAccess(err) {
				w.logger.Error("vk worker: longpoll init fatal", zap.Error(err), zap.String("account_id", accountID))
				w.dispatcher.handleAPIError(ctx, w.account, err)
				return
			}
			w.logger.Warn("vk worker: longpoll init failed, retrying",
				zap.Error(err), zap.Duration("backoff", backoff), zap.String("account_id", accountID))
			if !sleepCtx(ctx, backoff) {
				break
			}
			backoff = nextBackoff(backoff)
			continue
		}

		// Wire callbacks. Each one shells out to the dispatcher.
		lp.MessageNew(func(callCtx context.Context, obj events.MessageNewObject) {
			w.dispatcher.HandleMessageNew(callCtx, accountID, obj)
		})
		lp.WallReplyNew(func(callCtx context.Context, obj events.WallReplyNewObject) {
			w.dispatcher.HandleWallReplyNew(callCtx, accountID, obj)
		})

		w.logger.Info("vk worker started", zap.String("account_id", accountID), zap.Int("group_id", groupID))

		start := time.Now()
		runErr := lp.RunWithContext(ctx)

		switch {
		case ctx.Err() != nil || runErr == nil || errors.Is(runErr, context.Canceled):
			// Clean stop.
			w.logger.Info("vk worker stopped", zap.String("account_id", accountID))
			return
		case IsAuthError(runErr) || IsNoAccess(runErr):
			w.logger.Error("vk worker exited (fatal)", zap.Error(runErr), zap.String("account_id", accountID))
			w.dispatcher.handleAPIError(ctx, w.account, runErr)
			return
		default:
			// Transient — recover. A long healthy run resets the backoff.
			if time.Since(start) > time.Minute {
				backoff = time.Second
			}
			w.logger.Warn("vk worker: transient error, restarting long poll",
				zap.Error(runErr), zap.Duration("backoff", backoff), zap.String("account_id", accountID))
			if !sleepCtx(ctx, backoff) {
				break
			}
			backoff = nextBackoff(backoff)
		}
	}
	w.logger.Info("vk worker stopped", zap.String("account_id", accountID))
}

// sleepCtx waits for d or until ctx is cancelled. Returns false if ctx was cancelled (so the
// caller stops promptly on Stop/shutdown rather than sleeping out the backoff).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// nextBackoff doubles d, capped at maxWorkerBackoff.
func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > maxWorkerBackoff {
		return maxWorkerBackoff
	}
	return d
}

// Stop cancels the loop and waits for it to finish, up to a short grace period.
func (w *AccountWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	<-w.done
}

// AccountID returns the underlying connected_accounts.id this worker is bound to.
func (w *AccountWorker) AccountID() string { return w.account.ID }

// workerRegistry is a tiny concurrent map keyed by account_id. Exposed via the engine.WorkerManager.
type workerRegistry struct {
	mu      sync.Mutex
	workers map[string]*AccountWorker
}

func newWorkerRegistry() *workerRegistry {
	return &workerRegistry{workers: map[string]*AccountWorker{}}
}

func (r *workerRegistry) get(id string) (*AccountWorker, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.workers[id]
	return w, ok
}

func (r *workerRegistry) put(w *AccountWorker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[w.AccountID()] = w
}

func (r *workerRegistry) drop(id string) (*AccountWorker, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.workers[id]
	if ok {
		delete(r.workers, id)
	}
	return w, ok
}

func (r *workerRegistry) all() []*AccountWorker {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*AccountWorker, 0, len(r.workers))
	for _, w := range r.workers {
		out = append(out, w)
	}
	return out
}
