package engine

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// EventKind is the type of incoming platform event.
type EventKind string

const (
	EventKindDM      EventKind = "dm"
	EventKindComment EventKind = "comment"
)

// IncomingEvent is the normalized shape passed to the matcher from any platform worker.
type IncomingEvent struct {
	Kind            EventKind
	AccountID       string
	SenderID        string // platform-scoped user id
	SenderUsername  string
	SenderName      string
	Text            string
	PlatformEventID string // platform message id / comment id (for dedup + Private Reply)
	OccurredAt      time.Time
}

// MatchResult is the outcome of a match attempt.
// When Trigger is nil and Reason != "", the event was skipped (cooldown, limit, etc.).
type MatchResult struct {
	Trigger        *domain.Trigger
	MatchedKeyword string
	Reason         string // populated when Trigger is nil — useful for logging
}

// SubscriptionChecker is consulted by the matcher when a trigger has check_subscription enabled.
// A real implementation calls groups.isMember on VK or skips on IG (no equivalent API).
type SubscriptionChecker interface {
	IsSubscribed(ctx context.Context, account domain.ConnectedAccount, senderID string) (bool, error)
}

// TriggerMatcher loads triggers, matches incoming events, and tracks per-user cooldown + reply counters.
type TriggerMatcher struct {
	triggers repository.TriggerRepo
	rdb      *redis.Client
	cache    sync.Map // accountID → []domain.Trigger (snapshot)
	cacheTTL time.Duration
	checker  SubscriptionChecker
}

// NewTriggerMatcher returns a matcher.
func NewTriggerMatcher(triggers repository.TriggerRepo, rdb *redis.Client, checker SubscriptionChecker) *TriggerMatcher {
	return &TriggerMatcher{
		triggers: triggers,
		rdb:      rdb,
		cacheTTL: 60 * time.Second,
		checker:  checker,
	}
}

type triggerCacheEntry struct {
	triggers []domain.Trigger
	loadedAt time.Time
}

// InvalidateCache drops the cached triggers for an account. Call this from API
// handlers after create/update/delete so the next event reloads from the DB.
func (m *TriggerMatcher) InvalidateCache(accountID string) {
	m.cache.Delete(accountID)
}

// loadTriggers returns the active triggers for an account, sorted by priority DESC.
// Uses an in-process cache (TTL 60s) backed by the DB.
func (m *TriggerMatcher) loadTriggers(ctx context.Context, accountID string) ([]domain.Trigger, error) {
	if v, ok := m.cache.Load(accountID); ok {
		entry := v.(triggerCacheEntry)
		if time.Since(entry.loadedAt) < m.cacheTTL {
			return entry.triggers, nil
		}
	}
	rows, err := m.triggers.ListActiveByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("engine.matcher.loadTriggers: %w", err)
	}
	m.cache.Store(accountID, triggerCacheEntry{triggers: rows, loadedAt: time.Now()})
	return rows, nil
}

// Match runs the matcher pipeline against an event and returns the first trigger that fires.
// Returns nil + nil when nothing matched; nil + a Reason when a trigger was skipped (cooldown / limit).
func (m *TriggerMatcher) Match(ctx context.Context, account domain.ConnectedAccount, ev IncomingEvent) (*MatchResult, error) {
	triggers, err := m.loadTriggers(ctx, ev.AccountID)
	if err != nil {
		return nil, err
	}

	for i := range triggers {
		t := &triggers[i]
		if !triggerMatchesEvent(t, ev) {
			continue
		}
		matched, ok := evaluateText(t, ev.Text)
		if !ok {
			continue
		}
		// Cooldown and per-user limits are checked AFTER text match so we don't burn
		// counters on triggers that wouldn't fire anyway.
		if t.CooldownSeconds > 0 {
			cooling, err := m.inCooldown(ctx, t.ID, ev.SenderID)
			if err != nil {
				return nil, err
			}
			if cooling {
				return &MatchResult{Reason: "cooldown"}, nil
			}
		}
		if t.MaxRepliesPerUser > 0 {
			over, err := m.overLimit(ctx, t.ID, ev.SenderID, t.MaxRepliesPerUser)
			if err != nil {
				return nil, err
			}
			if over {
				return &MatchResult{Reason: "max_replies_reached"}, nil
			}
		}
		return &MatchResult{Trigger: t, MatchedKeyword: matched}, nil
	}
	return nil, nil
}

// triggerMatchesEvent checks event_type compatibility.
func triggerMatchesEvent(t *domain.Trigger, ev IncomingEvent) bool {
	switch t.EventType {
	case domain.EventTypeDM:
		return ev.Kind == EventKindDM
	case domain.EventTypeComment:
		return ev.Kind == EventKindComment
	case domain.EventTypeCommentAndDM:
		return ev.Kind == EventKindDM || ev.Kind == EventKindComment
	}
	return false
}

// evaluateText applies the match_mode to the text. Returns the matched keyword (or "*" / "regex")
// and a flag.
func evaluateText(t *domain.Trigger, text string) (string, bool) {
	switch t.MatchMode {
	case domain.MatchModeAll:
		return "*", true
	case domain.MatchModeRegex:
		if len(t.Keywords) == 0 {
			return "", false
		}
		re, err := regexp.Compile(t.Keywords[0])
		if err != nil {
			return "", false
		}
		if re.MatchString(text) {
			return t.Keywords[0], true
		}
		return "", false
	case domain.MatchModeKeyword:
		needle := text
		if !t.CaseSensitive {
			needle = strings.ToLower(needle)
		}
		for _, k := range t.Keywords {
			cmp := k
			if !t.CaseSensitive {
				cmp = strings.ToLower(cmp)
			}
			switch t.KeywordsMode {
			case domain.KeywordsModeExact:
				if strings.TrimSpace(needle) == cmp {
					return k, true
				}
			case domain.KeywordsModeStartsWith:
				if strings.HasPrefix(strings.TrimSpace(needle), cmp) {
					return k, true
				}
			default: // contains
				if strings.Contains(needle, cmp) {
					return k, true
				}
			}
		}
	}
	return "", false
}

// inCooldown reports whether a trigger:sender cooldown key is still set in Redis.
func (m *TriggerMatcher) inCooldown(ctx context.Context, triggerID, senderID string) (bool, error) {
	if m.rdb == nil {
		return false, nil
	}
	key := fmt.Sprintf("trigger_cd:%s:%s", triggerID, senderID)
	n, err := m.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("engine.matcher.inCooldown: %w", err)
	}
	return n > 0, nil
}

// overLimit reports whether the per-user reply counter has hit max for this trigger.
func (m *TriggerMatcher) overLimit(ctx context.Context, triggerID, senderID string, max int) (bool, error) {
	if m.rdb == nil {
		return false, nil
	}
	key := fmt.Sprintf("trigger_count:%s:%s", triggerID, senderID)
	val, err := m.rdb.Get(ctx, key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("engine.matcher.overLimit: %w", err)
	}
	return val >= max, nil
}

// RecordFire marks that a trigger has just fired for the given sender, updating cooldown + counter.
// Call this after the platform send succeeds.
func (m *TriggerMatcher) RecordFire(ctx context.Context, t *domain.Trigger, senderID string) error {
	if m.rdb == nil {
		return nil
	}
	if t.CooldownSeconds > 0 {
		key := fmt.Sprintf("trigger_cd:%s:%s", t.ID, senderID)
		if err := m.rdb.Set(ctx, key, "1", time.Duration(t.CooldownSeconds)*time.Second).Err(); err != nil {
			return fmt.Errorf("engine.matcher.RecordFire cooldown: %w", err)
		}
	}
	if t.MaxRepliesPerUser > 0 {
		key := fmt.Sprintf("trigger_count:%s:%s", t.ID, senderID)
		// Counters live for 30 days; reset by a future maintenance task.
		if _, err := m.rdb.Incr(ctx, key).Result(); err != nil {
			return fmt.Errorf("engine.matcher.RecordFire incr: %w", err)
		}
		if _, err := m.rdb.Expire(ctx, key, 30*24*time.Hour).Result(); err != nil {
			return fmt.Errorf("engine.matcher.RecordFire expire: %w", err)
		}
	}
	return nil
}

// SubscriptionGate implements the check_subscription feature as a GATE (VK-only).
//
// When the trigger matched but the sender is NOT a member of the community, it returns
// blocked=true plus the "please subscribe" nudge text (reply_if_unsubscribed) — the caller
// then sends that nudge instead of the real answer. When the sender IS subscribed (or the
// feature is off, the account isn't VK, or the membership lookup fails) it returns
// ("", false) and the trigger runs normally with its own action texts.
//
// Fail-open by design: a transient checker error must never withhold a legitimate reply.
//
// VK membership (groups.isMember) works for both DMs and comments. Instagram can only determine
// follow status in the messaging context (is_user_follow_business is populated once the user has
// messaged the business), so the IG check is skipped for comment events.
func (m *TriggerMatcher) SubscriptionGate(
	ctx context.Context,
	t *domain.Trigger,
	account domain.ConnectedAccount,
	senderID string,
	kind EventKind,
) (nudge string, blocked bool) {
	if !t.CheckSubscription || m.checker == nil {
		return "", false
	}
	if account.Platform == domain.PlatformInstagram && kind != EventKindDM {
		return "", false
	}
	subscribed, err := m.checker.IsSubscribed(ctx, account, senderID)
	if err != nil || subscribed {
		return "", false
	}
	if t.ReplyIfUnsubscribed != nil {
		return *t.ReplyIfUnsubscribed, true
	}
	return "", true
}
