package vk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/SevereCloud/vksdk/v3/events"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/engine"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// TokenDecrypter unwraps an encrypted access token. Mirrors handlers.TokenDecrypter
// so the dispatcher does not import the queue/handlers package.
type TokenDecrypter interface {
	DecryptToken(encoded string) (string, error)
}

// Dispatcher orchestrates one VK Long Poll event through:
//
//	account-lookup → matcher → token-decrypt → reply → log → recordFire.
//
// It is shared by every account's worker goroutine.
type Dispatcher struct {
	accounts repository.AccountRepo
	logs     repository.LogRepo
	matcher  *engine.TriggerMatcher
	decrypt  TokenDecrypter
	rdb      *redis.Client
	logger   *zap.Logger
	apiVer   string
}

// NewDispatcher wires the dispatcher. rdb may be nil (disables the send rate-limiter and the
// users.get cache used for {{name}}/{{username}}).
func NewDispatcher(
	accounts repository.AccountRepo,
	logs repository.LogRepo,
	matcher *engine.TriggerMatcher,
	decrypt TokenDecrypter,
	rdb *redis.Client,
	apiVersion string,
	logger *zap.Logger,
) *Dispatcher {
	return &Dispatcher{
		accounts: accounts,
		logs:     logs,
		matcher:  matcher,
		decrypt:  decrypt,
		rdb:      rdb,
		logger:   logger,
		apiVer:   apiVersion,
	}
}

// HandleMessageNew is the callback wired into longpoll.MessageNew.
// Looks up the account from the group_id, builds an engine.IncomingEvent, runs the matcher,
// and (on a hit) sends the reply via VK and writes a trigger_log row.
func (d *Dispatcher) HandleMessageNew(ctx context.Context, accountID string, obj events.MessageNewObject) {
	account, err := d.lookupActive(ctx, accountID)
	if err != nil {
		return
	}

	// Skip our own outgoing messages — VK echoes community-sent DMs back as message_new
	// (out=1, from_id=-group_id). Without this guard the matcher re-fires on the bot's own
	// replies. Mirrors the IG echo guard in handlers/instagram.go.
	if bool(obj.Message.Out) || isSelfSender(account, obj.Message.FromID) {
		return
	}

	text := obj.Message.Text
	senderID := strconv.Itoa(obj.Message.FromID)
	platformEventID := strconv.Itoa(obj.Message.ConversationMessageID)

	incoming := engine.IncomingEvent{
		Kind:            engine.EventKindDM,
		AccountID:       account.ID,
		SenderID:        senderID,
		Text:            text,
		PlatformEventID: platformEventID,
		OccurredAt:      time.Unix(int64(obj.Message.Date), 0),
	}
	res, err := d.matcher.Match(ctx, account, incoming)
	if err != nil {
		d.logger.Error("vk matcher", zap.Error(err), zap.String("account_id", account.ID))
		return
	}
	if res == nil {
		return
	}
	if res.Trigger == nil {
		d.recordSkipped(ctx, account.ID, "dm", platformEventID, senderID, "", text, res.Reason)
		return
	}

	// Decrypt the community token once and carry the plaintext on a copy of account, so the
	// matcher's subscription check (groups.isMember) can authenticate. The engine stays decrypt-agnostic.
	account, token, err := d.decryptedAccount(account)
	if err != nil {
		d.logger.Error("vk decrypt token", zap.Error(err), zap.String("account_id", account.ID))
		return
	}
	client, err := d.clientFor(account, token)
	if err != nil {
		d.logger.Error("vk client", zap.Error(err))
		return
	}

	// Resolve the sender's name/username for {{name}}/{{username}} (best-effort, cached).
	name, username, _ := client.GetUser(ctx, obj.Message.FromID)
	data := engine.TemplateData{
		SenderName:     name,
		SenderUsername: username,
		MatchedKeyword: res.MatchedKeyword,
		EventTime:      incoming.OccurredAt,
	}

	// Subscription gate: a non-member gets the "please subscribe" nudge instead of the real
	// answer; a member (or any lookup failure) gets the normal reply.
	nudge, blocked := d.matcher.SubscriptionGate(ctx, res.Trigger, account, senderID, engine.EventKindDM)

	chosen, ok := engine.ResolveReplyText(blocked, nudge, res.Trigger.DMText, data)
	if !ok {
		reason := "no_action_text"
		if blocked {
			reason = "not_subscribed"
		}
		d.recordSkipped(ctx, account.ID, "dm", platformEventID, senderID, res.MatchedKeyword, text, reason)
		return
	}

	// Send now, or after the trigger's reply delay (deferred path uses a fresh ctx).
	send := func(sctx context.Context) {
		if _, err := client.SendMessage(sctx, obj.Message.FromID, chosen); err != nil {
			d.recordError(sctx, res.Trigger.ID, account.ID, "dm", platformEventID, senderID, "", text, res.MatchedKeyword, err.Error())
			d.handleAPIError(sctx, account, err)
			return
		}
		d.recordOK(sctx, res.Trigger.ID, account.ID, "dm", platformEventID, senderID, "", text, res.MatchedKeyword, "sent_dm")
		if err := d.matcher.RecordFire(sctx, res.Trigger, senderID); err != nil {
			d.logger.Warn("vk record fire", zap.Error(err))
		}
	}
	engine.ScheduleReply(ctx, res.Trigger.ReplyDelaySeconds, send)
}

// HandleWallReplyNew is the callback wired into longpoll.WallReplyNew.
func (d *Dispatcher) HandleWallReplyNew(ctx context.Context, accountID string, obj events.WallReplyNewObject) {
	account, err := d.lookupActive(ctx, accountID)
	if err != nil {
		return
	}

	// Skip comments authored by the community itself (e.g. our own reply_to_comment),
	// which VK delivers back through wall_reply_new with from_id=-group_id.
	if isSelfSender(account, obj.FromID) {
		return
	}

	senderID := strconv.Itoa(obj.FromID)
	platformEventID := strconv.Itoa(obj.ID)
	text := obj.Text

	incoming := engine.IncomingEvent{
		Kind:            engine.EventKindComment,
		AccountID:       account.ID,
		SenderID:        senderID,
		Text:            text,
		PlatformEventID: platformEventID,
		OccurredAt:      time.Unix(int64(obj.Date), 0),
	}
	res, err := d.matcher.Match(ctx, account, incoming)
	if err != nil {
		d.logger.Error("vk matcher", zap.Error(err))
		return
	}
	if res == nil {
		return
	}
	if res.Trigger == nil {
		d.recordSkipped(ctx, account.ID, "comment", platformEventID, senderID, "", text, res.Reason)
		return
	}

	t := res.Trigger
	matched := res.MatchedKeyword

	// Decrypt the community token once and carry the plaintext on a copy of account, so the
	// matcher's subscription check (groups.isMember) can authenticate. The engine stays decrypt-agnostic.
	account, token, err := d.decryptedAccount(account)
	if err != nil {
		d.logger.Error("vk decrypt token", zap.Error(err), zap.String("account_id", account.ID))
		return
	}
	client, err := d.clientFor(account, token)
	if err != nil {
		d.logger.Error("vk client", zap.Error(err))
		return
	}

	// Resolve the commenter's name/username for {{name}}/{{username}} (best-effort, cached).
	name, username, _ := client.GetUser(ctx, obj.FromID)
	data := engine.TemplateData{
		SenderName:     name,
		SenderUsername: username,
		MatchedKeyword: matched,
		EventTime:      incoming.OccurredAt,
	}

	// Send now, or after the trigger's reply delay (deferred path uses a fresh ctx).
	send := func(sctx context.Context) {
		// Subscription gate: a non-member gets the "please subscribe" nudge on the trigger's
		// enabled channels instead of the real answer.
		nudge, blocked := d.matcher.SubscriptionGate(sctx, t, account, senderID, engine.EventKindComment)

		// A comment trigger can post a public reply in the thread AND/OR send a private DM to the
		// commenter. Each enabled channel sends its own text (or the nudge when blocked).
		action := ""
		var lastErr error

		if t.ReplyToComment {
			if chosen, ok := engine.ResolveReplyText(blocked, nudge, t.ReplyCommentText, data); ok {
				// owner_id for community wall = -group_id
				if _, err := client.ReplyToWallComment(sctx, -client.GroupID, obj.PostID, obj.ID, chosen); err != nil {
					lastErr = err
				} else {
					action = "replied_comment"
				}
			}
		}

		if t.SendPrivateReply && lastErr == nil {
			if chosen, ok := engine.ResolveReplyText(blocked, nudge, t.PrivateReplyText, data); ok {
				// DM the commenter directly. NOTE: VK rejects messages.send to a user who has not
				// allowed messages from the community (error 901) — there is no IG-style comment window.
				if _, err := client.SendMessage(sctx, obj.FromID, chosen); err != nil {
					lastErr = err
				} else if action == "replied_comment" {
					action = "both"
				} else {
					action = "sent_dm"
				}
			}
		}

		if lastErr != nil {
			d.recordError(sctx, t.ID, account.ID, "comment", platformEventID, senderID, "", text, matched, lastErr.Error())
			d.handleAPIError(sctx, account, lastErr)
			return
		}
		if action == "" {
			reason := "no_action_text"
			if blocked {
				reason = "not_subscribed"
			}
			d.recordSkipped(sctx, account.ID, "comment", platformEventID, senderID, matched, text, reason)
			return
		}
		d.recordOK(sctx, t.ID, account.ID, "comment", platformEventID, senderID, "", text, matched, action)
		if err := d.matcher.RecordFire(sctx, t, senderID); err != nil {
			d.logger.Warn("vk record fire", zap.Error(err))
		}
	}
	engine.ScheduleReply(ctx, t.ReplyDelaySeconds, send)
}

// decryptedAccount returns a copy of account whose AccessToken has been replaced with the
// decrypted community token, alongside that plaintext token. The decrypted copy is what must
// be handed to the matcher's SubscriptionGate so VK subscription checks (groups.isMember) can
// authenticate; the engine itself never sees ciphertext-vs-plaintext concerns.
func (d *Dispatcher) decryptedAccount(account domain.ConnectedAccount) (domain.ConnectedAccount, string, error) {
	token, err := d.decrypt.DecryptToken(account.AccessToken)
	if err != nil {
		return domain.ConnectedAccount{}, "", fmt.Errorf("vk.decryptedAccount: %w", err)
	}
	account.AccessToken = token
	return account, token, nil
}

// clientFor builds a per-event VK client from an already-decrypted community token.
// The vksdk.NewVK call is cheap (just sets fields) so we don't cache.
func (d *Dispatcher) clientFor(account domain.ConnectedAccount, token string) (*Client, error) {
	groupID, err := strconv.Atoi(account.PlatformID)
	if err != nil {
		return nil, fmt.Errorf("vk.clientFor group_id %q: %w", account.PlatformID, err)
	}
	return NewClient(token, groupID, account.ID, d.apiVer, d.rdb), nil
}

func (d *Dispatcher) lookupActive(ctx context.Context, accountID string) (domain.ConnectedAccount, error) {
	a, err := d.accounts.GetByID(ctx, accountID)
	if err != nil {
		return domain.ConnectedAccount{}, err
	}
	if !a.IsActive || a.Status == domain.AccountStatusError {
		return domain.ConnectedAccount{}, errors.New("account paused or in error")
	}
	return a, nil
}

func (d *Dispatcher) recordOK(ctx context.Context, triggerID, accountID, eventType, platformEventID, senderID, senderUsername, incomingText, matchedKeyword, action string) {
	_, err := d.logs.Create(ctx, repository.CreateLogParams{
		TriggerID:       triggerID,
		AccountID:       accountID,
		EventType:       eventType,
		PlatformEventID: strPtr(platformEventID),
		SenderID:        senderID,
		SenderUsername:  strPtrOrNil(senderUsername),
		IncomingText:    strPtrOrNil(incomingText),
		MatchedKeyword:  strPtrOrNil(matchedKeyword),
		ActionTaken:     action,
	})
	if err != nil {
		d.logger.Warn("vk log insert", zap.Error(err))
	}
}

func (d *Dispatcher) recordError(ctx context.Context, triggerID, accountID, eventType, platformEventID, senderID, senderUsername, incomingText, matchedKeyword, errMsg string) {
	emsg := errMsg
	_, err := d.logs.Create(ctx, repository.CreateLogParams{
		TriggerID:       triggerID,
		AccountID:       accountID,
		EventType:       eventType,
		PlatformEventID: strPtr(platformEventID),
		SenderID:        senderID,
		SenderUsername:  strPtrOrNil(senderUsername),
		IncomingText:    strPtrOrNil(incomingText),
		MatchedKeyword:  strPtrOrNil(matchedKeyword),
		ActionTaken:     "error",
		ErrorMessage:    &emsg,
	})
	if err != nil {
		d.logger.Warn("vk log insert", zap.Error(err))
	}
}

// recordSkipped writes a trigger_log row with a NULL trigger_id for events that matched
// no trigger or were suppressed (cooldown, max_replies_reached, no_action_text). The skip
// reason is stored in error_message for observability. trigger_id became NULLABLE in
// migration 008.
func (d *Dispatcher) recordSkipped(ctx context.Context, accountID, eventType, platformEventID, senderID, matchedKeyword, incomingText, reason string) {
	d.logger.Info("vk skipped",
		zap.String("account_id", accountID),
		zap.String("event_type", eventType),
		zap.String("reason", reason),
	)
	_, err := d.logs.Create(ctx, repository.CreateLogParams{
		TriggerID:       "", // NULL — no trigger fired
		AccountID:       accountID,
		EventType:       eventType,
		PlatformEventID: strPtrOrNil(platformEventID),
		SenderID:        senderID,
		IncomingText:    strPtrOrNil(incomingText),
		MatchedKeyword:  strPtrOrNil(matchedKeyword),
		ActionTaken:     "skipped",
		ErrorMessage:    strPtrOrNil(reason),
	})
	if err != nil {
		d.logger.Warn("vk log insert (skipped)", zap.Error(err))
	}
}

// handleAPIError flips the account into 'error' on auth/access failures and downgrades expected
// or transient conditions to a non-error log.
func (d *Dispatcher) handleAPIError(ctx context.Context, account domain.ConnectedAccount, err error) {
	switch {
	case err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		// Clean shutdown / transient long-poll timeout — not an account problem.
		return
	case IsAuthError(err):
		_ = d.accounts.SetStatus(ctx, account.ID, domain.AccountStatusError, "VK token invalid or revoked. Reconnect required.")
	case IsNoAccess(err):
		_ = d.accounts.SetStatus(ctx, account.ID, domain.AccountStatusError, "VK access denied (code 15). Reconnect or grant permissions.")
	case IsCantSendToUser(err):
		// Code 901: user hasn't allowed community DMs (typical for comment→DM). Expected, not a failure.
		d.logger.Info("vk: cannot DM user (no permission, code 901)", zap.String("account_id", account.ID))
	case IsFloodControl(err):
		d.logger.Warn("vk: flood control", zap.String("account_id", account.ID))
	default:
		d.logger.Error("vk: unhandled api error", zap.Error(err), zap.String("account_id", account.ID))
	}
}

// isSelfSender reports whether an event was authored by the community itself
// (our own outgoing DM or our own comment reply). VK identifies the community as
// the negative of its group_id, and account.PlatformID holds that group_id.
// Without this guard the matcher would re-fire on the bot's own output.
func isSelfSender(account domain.ConnectedAccount, fromID int) bool {
	gid, err := strconv.Atoi(account.PlatformID)
	if err != nil {
		return false
	}
	return fromID == -gid
}

func strPtr(s string) *string { return &s }

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
