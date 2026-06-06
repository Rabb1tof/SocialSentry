package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Channel names — match the constants expected by the VK WorkerManager.
const (
	ChannelWorkerStart    = "worker:start"
	ChannelWorkerStop     = "worker:stop"
	ChannelTriggersReload = "triggers:reload"
	// ChannelPlatformState signals that a platform's global enabled flag changed.
	// Published as "platform:state:<platform>"; the VK WorkerManager subscribes to
	// the "vk" suffix to start/stop its workers live.
	ChannelPlatformState = "platform:state"
)

// Publisher implements both service.ReloadPublisher and service.WorkerLifecyclePublisher
// on top of a Redis client. All publishes are best-effort — failures are logged, not returned.
type Publisher struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewPublisher wires the publisher.
func NewPublisher(rdb *redis.Client, logger *zap.Logger) *Publisher {
	return &Publisher{rdb: rdb, logger: logger}
}

func (p *Publisher) publish(ctx context.Context, channel string) {
	if p.rdb == nil {
		return
	}
	if err := p.rdb.Publish(ctx, channel, "").Err(); err != nil {
		p.logger.Warn("pubsub publish", zap.Error(err), zap.String("channel", channel))
	}
}

// PublishTriggersReload tells the worker to invalidate its cached triggers for the account.
func (p *Publisher) PublishTriggersReload(ctx context.Context, accountID string) {
	p.publish(ctx, fmt.Sprintf("%s:%s", ChannelTriggersReload, accountID))
}

// PublishWorkerStart tells the worker to spawn (or restart) the per-account goroutine.
func (p *Publisher) PublishWorkerStart(ctx context.Context, accountID string) {
	p.publish(ctx, fmt.Sprintf("%s:%s", ChannelWorkerStart, accountID))
}

// PublishWorkerStop tells the worker to terminate the per-account goroutine.
func (p *Publisher) PublishWorkerStop(ctx context.Context, accountID string) {
	p.publish(ctx, fmt.Sprintf("%s:%s", ChannelWorkerStop, accountID))
}

// PublishPlatformState tells subscribers that the given platform's enabled flag changed.
// Subscribers re-read the authoritative value (DB / cache) rather than trusting a payload.
func (p *Publisher) PublishPlatformState(ctx context.Context, platform string) {
	p.publish(ctx, fmt.Sprintf("%s:%s", ChannelPlatformState, platform))
}
