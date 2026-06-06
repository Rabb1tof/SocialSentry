package queue

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// ChannelTriggerFired carries per-log realtime events from the worker to the API's
// WebSocket hub. The payload is a JSON-encoded TriggerFiredEvent.
const ChannelTriggerFired = "events:trigger_fired"

// TriggerFiredEvent is the minimal realtime notification emitted whenever a trigger log
// row is written (sent / skipped / error). The browser uses account_id to refresh the
// logs view and action_taken to decide whether to surface a toast.
type TriggerFiredEvent struct {
	AccountID      string `json:"account_id"`
	EventType      string `json:"event_type"`
	ActionTaken    string `json:"action_taken"`
	SenderUsername string `json:"sender_username,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// PublishTriggerFired emits a realtime event. Best-effort: a marshal/publish failure is
// logged, never returned — realtime delivery must never break the worker pipeline.
func (p *Publisher) PublishTriggerFired(ctx context.Context, evt TriggerFiredEvent) {
	if p.rdb == nil {
		return
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		p.logger.Warn("pubsub marshal trigger_fired", zap.Error(err))
		return
	}
	if err := p.rdb.Publish(ctx, ChannelTriggerFired, payload).Err(); err != nil {
		p.logger.Warn("pubsub publish", zap.Error(err), zap.String("channel", ChannelTriggerFired))
	}
}

// PublishingLogRepo decorates a repository.LogRepo so every successful Create also emits a
// TriggerFiredEvent over Redis. This is the single chokepoint that gives both the Instagram
// handler and the VK dispatcher live log updates without either knowing about WebSockets.
type PublishingLogRepo struct {
	repository.LogRepo
	pub *Publisher
}

// NewPublishingLogRepo wraps inner so writes fan out to the realtime channel.
func NewPublishingLogRepo(inner repository.LogRepo, pub *Publisher) *PublishingLogRepo {
	return &PublishingLogRepo{LogRepo: inner, pub: pub}
}

// Create writes the log via the wrapped repo, then publishes a realtime event.
func (r *PublishingLogRepo) Create(ctx context.Context, p repository.CreateLogParams) (domain.TriggerLog, error) {
	row, err := r.LogRepo.Create(ctx, p)
	if err != nil {
		return row, err
	}
	username := ""
	if row.SenderUsername != nil {
		username = *row.SenderUsername
	}
	r.pub.PublishTriggerFired(ctx, TriggerFiredEvent{
		AccountID:      row.AccountID,
		EventType:      row.EventType,
		ActionTaken:    row.ActionTaken,
		SenderUsername: username,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	})
	return row, nil
}
