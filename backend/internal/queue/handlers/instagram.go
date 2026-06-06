// Package handlers contains the Asynq task handler implementations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/engine"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/queue"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// IGSender encapsulates the IG API calls the worker needs.
type IGSender interface {
	SendDM(ctx context.Context, accountID, pageID, pageToken, recipientID, text string) (string, error)
	SendPrivateReply(ctx context.Context, accountID, pageID, pageToken, commentID, text string) (string, error)
	ReplyToComment(ctx context.Context, accountID, commentID, pageToken, text string) (string, error)
	GetUserProfile(ctx context.Context, igsid, pageToken string) (instagram.UserProfile, error)
}

// TokenDecrypter unwraps an encrypted access token.
type TokenDecrypter interface {
	DecryptToken(encoded string) (string, error)
}

// PlatformGate reports whether a platform is globally enabled (admin kill-switch).
// Implemented by service.SettingsService.
type PlatformGate interface {
	IsEnabled(ctx context.Context, platform string) (bool, error)
}

// InstagramHandler processes one webhook payload at a time.
// It is registered with the Asynq mux against queue.TaskInstagramEvent.
type InstagramHandler struct {
	accounts repository.AccountRepo
	logs     repository.LogRepo
	matcher  *engine.TriggerMatcher
	client   IGSender
	decrypt  TokenDecrypter
	settings PlatformGate
	logger   *zap.Logger
}

// NewInstagramHandler wires the handler.
func NewInstagramHandler(
	accounts repository.AccountRepo,
	logs repository.LogRepo,
	matcher *engine.TriggerMatcher,
	client IGSender,
	decrypt TokenDecrypter,
	settings PlatformGate,
	logger *zap.Logger,
) *InstagramHandler {
	return &InstagramHandler{
		accounts: accounts,
		logs:     logs,
		matcher:  matcher,
		client:   client,
		decrypt:  decrypt,
		settings: settings,
		logger:   logger,
	}
}

// Handle is the asynq.HandlerFunc body.
func (h *InstagramHandler) Handle(ctx context.Context, t *asynq.Task) error {
	// Global admin kill-switch: when Instagram is disabled, drop the event without
	// retrying (returning nil acks the task) so no reply is ever sent.
	if h.settings != nil {
		if enabled, err := h.settings.IsEnabled(ctx, domain.PlatformInstagram); err == nil && !enabled {
			return nil
		}
	}

	var p queue.InstagramEventPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("handlers.instagram: payload: %w", err)
	}
	env, err := instagram.ParseEnvelope(p.RawBody)
	if err != nil {
		return fmt.Errorf("handlers.instagram: envelope: %w", err)
	}

	// Each DM and comment is processed independently. We don't fail the whole task on a
	// single event error — that's already logged via trigger_logs.
	env.IterDMs(func(pageID string, ev instagram.MessagingEvent) {
		h.processDM(ctx, pageID, ev)
	})
	if err := env.IterComments(func(pageID string, cv instagram.CommentValue) {
		h.processComment(ctx, pageID, cv)
	}); err != nil {
		h.logger.Error("handlers.instagram: comment iter", zap.Error(err))
	}
	return nil
}

func (h *InstagramHandler) processDM(ctx context.Context, pageID string, ev instagram.MessagingEvent) {
	account, err := h.lookupActive(ctx, pageID)
	if err != nil {
		return
	}
	// Skip echoes — Meta sometimes delivers our own outgoing as inbound to the same webhook.
	if account.PageID != nil && ev.Sender.ID == *account.PageID {
		return
	}

	text := ""
	mid := ""
	if ev.Message != nil {
		text = ev.Message.Text
		mid = ev.Message.MID
	}

	incoming := engine.IncomingEvent{
		Kind:            engine.EventKindDM,
		AccountID:       account.ID,
		SenderID:        ev.Sender.ID,
		Text:            text,
		PlatformEventID: mid,
		OccurredAt:      time.Unix(ev.Timestamp/1000, 0),
	}
	res, err := h.matcher.Match(ctx, account, incoming)
	if err != nil {
		h.logger.Error("matcher", zap.Error(err), zap.String("account_id", account.ID))
		return
	}
	if res == nil {
		return
	}
	if res.Trigger == nil {
		// Skipped (cooldown / limit) — record one for visibility.
		h.recordLog(ctx, "", account.ID, "dm", &mid, ev.Sender.ID, nil, &text, nil, "skipped", &res.Reason)
		return
	}

	token, err := h.decrypt.DecryptToken(account.AccessToken)
	if err != nil {
		h.logger.Error("decrypt token", zap.Error(err), zap.String("account_id", account.ID))
		return
	}
	// Carry the decrypted Page Access Token on the account copy so the subscription gate's IG
	// branch (is_user_follow_business) can authenticate. Mirrors the VK decrypted-account flow.
	account.AccessToken = token
	pageIDStr := ""
	if account.PageID != nil {
		pageIDStr = *account.PageID
	}

	// Resolve the sender's name/username for {{name}}/{{username}} (best-effort, cached).
	// DM webhooks carry only the IGSID, so this hits the Graph API.
	profile, _ := h.client.GetUserProfile(ctx, ev.Sender.ID, token)
	data := engine.TemplateData{
		SenderName:     profile.Name,
		SenderUsername: profile.Username,
		MatchedKeyword: res.MatchedKeyword,
		EventTime:      incoming.OccurredAt,
	}
	matched := res.MatchedKeyword

	// Subscription gate: on IG this checks is_user_follow_business (DM context only).
	nudge, blocked := h.matcher.SubscriptionGate(ctx, res.Trigger, account, ev.Sender.ID, engine.EventKindDM)
	chosen, ok := engine.ResolveReplyText(blocked, nudge, res.Trigger.DMText, data)
	if !ok {
		reason := "no_action_text"
		if blocked {
			reason = "not_subscribed"
		}
		h.recordLog(ctx, res.Trigger.ID, account.ID, "dm", &mid, ev.Sender.ID, nil, &text, &matched, "skipped", &reason)
		return
	}

	// Send now, or after the trigger's reply delay. The deferred path runs on a
	// fresh background context (the webhook task context is already done).
	send := func(sctx context.Context) {
		_, sendErr := h.client.SendDM(sctx, account.ID, pageIDStr, token, ev.Sender.ID, chosen)
		if sendErr != nil {
			msg := sendErr.Error()
			h.recordLog(sctx, res.Trigger.ID, account.ID, "dm", &mid, ev.Sender.ID, nil, &text, &matched, "error", &msg)
			h.handleAPIError(sctx, account, sendErr)
			return
		}
		h.recordLog(sctx, res.Trigger.ID, account.ID, "dm", &mid, ev.Sender.ID, nil, &text, &matched, "sent_dm", nil)
		if err := h.matcher.RecordFire(sctx, res.Trigger, ev.Sender.ID); err != nil {
			h.logger.Warn("record fire", zap.Error(err))
		}
	}
	engine.ScheduleReply(ctx, res.Trigger.ReplyDelaySeconds, send)
}

func (h *InstagramHandler) processComment(ctx context.Context, pageID string, cv instagram.CommentValue) {
	account, err := h.lookupActive(ctx, pageID)
	if err != nil {
		return
	}

	incoming := engine.IncomingEvent{
		Kind:            engine.EventKindComment,
		AccountID:       account.ID,
		SenderID:        cv.From.ID,
		SenderUsername:  cv.From.Username,
		Text:            cv.Text,
		PlatformEventID: cv.ID,
		OccurredAt:      time.Now(),
	}
	res, err := h.matcher.Match(ctx, account, incoming)
	if err != nil {
		h.logger.Error("matcher", zap.Error(err))
		return
	}
	if res == nil {
		return
	}
	if res.Trigger == nil {
		h.recordLog(ctx, "", account.ID, "comment", &cv.ID, cv.From.ID, &cv.From.Username, &cv.Text, nil, "skipped", &res.Reason)
		return
	}

	token, err := h.decrypt.DecryptToken(account.AccessToken)
	if err != nil {
		h.logger.Error("decrypt token", zap.Error(err))
		return
	}
	pageIDStr := ""
	if account.PageID != nil {
		pageIDStr = *account.PageID
	}

	t := res.Trigger
	matched := res.MatchedKeyword
	// IG comment webhooks already carry the commenter's username — use it for {{name}}/{{username}}.
	data := engine.TemplateData{
		SenderName:     cv.From.Username,
		SenderUsername: cv.From.Username,
		MatchedKeyword: matched,
		EventTime:      incoming.OccurredAt,
	}
	// Send now, or after the trigger's reply delay (deferred path uses a fresh ctx).
	send := func(sctx context.Context) {
		// Subscription gate: skipped for IG comments (follow status is only available in the
		// messaging context, not for commenters).
		nudge, blocked := h.matcher.SubscriptionGate(sctx, t, account, cv.From.ID, engine.EventKindComment)

		action := ""
		var lastErr error

		if t.ReplyToComment {
			if chosen, ok := engine.ResolveReplyText(blocked, nudge, t.ReplyCommentText, data); ok {
				if _, err := h.client.ReplyToComment(sctx, account.ID, cv.ID, token, chosen); err != nil {
					lastErr = err
				} else {
					action = "replied_comment"
				}
			}
		}
		if t.SendPrivateReply && lastErr == nil {
			if chosen, ok := engine.ResolveReplyText(blocked, nudge, t.PrivateReplyText, data); ok {
				if _, err := h.client.SendPrivateReply(sctx, account.ID, pageIDStr, token, cv.ID, chosen); err != nil {
					lastErr = err
				} else if action == "replied_comment" {
					action = "both"
				} else {
					action = "sent_dm"
				}
			}
		}

		if lastErr != nil {
			msg := lastErr.Error()
			h.recordLog(sctx, t.ID, account.ID, "comment", &cv.ID, cv.From.ID, &cv.From.Username, &cv.Text, &matched, "error", &msg)
			h.handleAPIError(sctx, account, lastErr)
			return
		}
		if action == "" {
			// Trigger matched but produced no enabled action / empty text (or a non-subscriber with no nudge).
			skipped := "no_action_text"
			if blocked {
				skipped = "not_subscribed"
			}
			h.recordLog(sctx, t.ID, account.ID, "comment", &cv.ID, cv.From.ID, &cv.From.Username, &cv.Text, &matched, "skipped", &skipped)
			return
		}
		h.recordLog(sctx, t.ID, account.ID, "comment", &cv.ID, cv.From.ID, &cv.From.Username, &cv.Text, &matched, action, nil)
		if err := h.matcher.RecordFire(sctx, t, cv.From.ID); err != nil {
			h.logger.Warn("record fire", zap.Error(err))
		}
	}
	engine.ScheduleReply(ctx, t.ReplyDelaySeconds, send)
}

// lookupActive finds the account behind a webhook entry.id (Facebook Page ID).
// Returns an error when the account is missing, inactive, or in error state — caller silently drops.
func (h *InstagramHandler) lookupActive(ctx context.Context, pageID string) (domain.ConnectedAccount, error) {
	a, err := h.accounts.GetByPageID(ctx, domain.PlatformInstagram, pageID)
	if err != nil {
		return domain.ConnectedAccount{}, err
	}
	if !a.IsActive || a.Status == domain.AccountStatusError {
		return domain.ConnectedAccount{}, errors.New("account paused or in error")
	}
	return a, nil
}

func (h *InstagramHandler) recordLog(
	ctx context.Context,
	triggerID, accountID, eventType string,
	platformEventID *string,
	senderID string,
	senderUsername *string,
	incomingText, matchedKeyword *string,
	actionTaken string,
	errorMessage *string,
) {
	// triggerID may be empty for skipped/ingress events (cooldown, limit, no_action_text).
	// Since migration 008 made trigger_logs.trigger_id NULLABLE, the repository inserts
	// these with a NULL trigger_id instead of dropping them.
	if _, err := h.logs.Create(ctx, repository.CreateLogParams{
		TriggerID:       triggerID,
		AccountID:       accountID,
		EventType:       eventType,
		PlatformEventID: platformEventID,
		SenderID:        senderID,
		SenderUsername:  senderUsername,
		IncomingText:    incomingText,
		MatchedKeyword:  matchedKeyword,
		ActionTaken:     actionTaken,
		ErrorMessage:    errorMessage,
	}); err != nil {
		h.logger.Warn("trigger log insert", zap.Error(err))
	}
}

// handleAPIError checks well-known Meta error codes and flips the account status when needed.
func (h *InstagramHandler) handleAPIError(ctx context.Context, account domain.ConnectedAccount, err error) {
	var apiErr *instagram.APIError
	if !errors.As(err, &apiErr) {
		return
	}
	switch {
	case apiErr.IsExpiredToken():
		_ = h.accounts.SetStatus(ctx, account.ID, domain.AccountStatusError, "Page Access Token expired or revoked. Reconnect required.")
	case apiErr.IsOutsideWindow():
		h.logger.Info("ig: outside 24h window, skipping", zap.String("account_id", account.ID))
	case apiErr.IsRateLimited():
		h.logger.Warn("ig: rate limited at Meta", zap.String("account_id", account.ID))
	default:
		h.logger.Error("ig: unhandled api error", zap.Error(err), zap.String("account_id", account.ID))
	}
}
