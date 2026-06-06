package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/engine"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

var (
	// ErrTriggerLimitExceeded is returned when an account hits the per-plan trigger cap.
	ErrTriggerLimitExceeded = errors.New("service.trigger: trigger limit exceeded for plan")
	// ErrTriggerValidation is the umbrella sentinel for any trigger validation failure.
	// All concrete validation errors below wrap this so handlers can respond with 400 uniformly.
	ErrTriggerValidation = errors.New("service.trigger: validation error")
	// ErrInvalidEventType is returned for unknown event_type values.
	ErrInvalidEventType = fmt.Errorf("%w: invalid event_type", ErrTriggerValidation)
	// ErrInvalidMatchMode is returned for unknown match_mode values.
	ErrInvalidMatchMode = fmt.Errorf("%w: invalid match_mode", ErrTriggerValidation)
	// ErrInvalidKeywordsMode is returned for unknown keywords_mode values.
	ErrInvalidKeywordsMode = fmt.Errorf("%w: invalid keywords_mode", ErrTriggerValidation)
	// ErrInvalidRegex is returned when a regex match_mode trigger has a malformed first keyword.
	ErrInvalidRegex = fmt.Errorf("%w: invalid regex", ErrTriggerValidation)
	// ErrNoAction is returned when a trigger has no action enabled.
	ErrNoAction = fmt.Errorf("%w: at least one action (reply / dm) must be enabled", ErrTriggerValidation)
	// ErrMissingActionText is returned when an action is enabled but its text is empty.
	ErrMissingActionText = fmt.Errorf("%w: enabled action requires non-empty text", ErrTriggerValidation)
)

// ReloadPublisher notifies the worker process that the trigger set for an account changed,
// so it can invalidate its in-process matcher cache. Implementations should be non-blocking.
// A nil ReloadPublisher is a valid configuration (worker will rely on its 60s TTL).
type ReloadPublisher interface {
	PublishTriggersReload(ctx context.Context, accountID string)
}

// TriggerService handles trigger CRUD with validation and plan-limit enforcement.
type TriggerService struct {
	triggers repository.TriggerRepo
	accounts repository.AccountRepo
	subs     repository.SubscriptionRepo
	pub      ReloadPublisher
}

// NewTriggerService wires the service.
func NewTriggerService(
	triggers repository.TriggerRepo,
	accounts repository.AccountRepo,
	subs repository.SubscriptionRepo,
	pub ReloadPublisher,
) *TriggerService {
	return &TriggerService{triggers: triggers, accounts: accounts, subs: subs, pub: pub}
}

func (s *TriggerService) publishReload(ctx context.Context, accountID string) {
	if s.pub != nil {
		s.pub.PublishTriggersReload(ctx, accountID)
	}
}

// Create validates and inserts a new trigger after verifying account ownership and plan limit.
func (s *TriggerService) Create(ctx context.Context, userID, accountID string, p repository.TriggerParams) (domain.Trigger, error) {
	if err := s.ownAccount(ctx, userID, accountID); err != nil {
		return domain.Trigger{}, err
	}
	if err := validateTrigger(p); err != nil {
		return domain.Trigger{}, err
	}
	if err := s.checkLimit(ctx, userID, accountID); err != nil {
		return domain.Trigger{}, err
	}
	p.AccountID = accountID
	t, err := s.triggers.Create(ctx, p)
	if err != nil {
		return t, err
	}
	s.publishReload(ctx, accountID)
	return t, nil
}

// ListByAccount returns triggers belonging to an account the user owns.
func (s *TriggerService) ListByAccount(ctx context.Context, userID, accountID string) ([]domain.Trigger, error) {
	if err := s.ownAccount(ctx, userID, accountID); err != nil {
		return nil, err
	}
	return s.triggers.ListByAccount(ctx, accountID)
}

// Get returns a single trigger if its account belongs to the user.
func (s *TriggerService) Get(ctx context.Context, userID, triggerID string) (domain.Trigger, error) {
	t, err := s.triggers.GetByID(ctx, triggerID)
	if err != nil {
		return domain.Trigger{}, err
	}
	if err := s.ownAccount(ctx, userID, t.AccountID); err != nil {
		return domain.Trigger{}, repository.ErrNotFound
	}
	return t, nil
}

// Update validates and replaces a trigger after ownership check.
func (s *TriggerService) Update(ctx context.Context, userID, triggerID string, p repository.TriggerParams) (domain.Trigger, error) {
	existing, err := s.triggers.GetByID(ctx, triggerID)
	if err != nil {
		return domain.Trigger{}, err
	}
	if err := s.ownAccount(ctx, userID, existing.AccountID); err != nil {
		return domain.Trigger{}, repository.ErrNotFound
	}
	if err := validateTrigger(p); err != nil {
		return domain.Trigger{}, err
	}
	t, err := s.triggers.Update(ctx, triggerID, p)
	if err != nil {
		return t, err
	}
	s.publishReload(ctx, existing.AccountID)
	return t, nil
}

// Toggle flips is_active.
func (s *TriggerService) Toggle(ctx context.Context, userID, triggerID string, active bool) error {
	existing, err := s.triggers.GetByID(ctx, triggerID)
	if err != nil {
		return err
	}
	if err := s.ownAccount(ctx, userID, existing.AccountID); err != nil {
		return repository.ErrNotFound
	}
	if err := s.triggers.Toggle(ctx, triggerID, active); err != nil {
		return err
	}
	s.publishReload(ctx, existing.AccountID)
	return nil
}

// TestEventKind is the user-supplied event kind for the /test endpoint.
// Accepts only "dm" or "comment"; anything else is rejected as a validation error.
type TestEventKind string

const (
	TestEventDM      TestEventKind = "dm"
	TestEventComment TestEventKind = "comment"
)

// TestParams is the input to TriggerService.Test.
type TestParams struct {
	Text           string
	SenderID       string
	SenderName     string
	SenderUsername string
	Kind           TestEventKind
}

// ErrInvalidTestEventKind is returned when the requested kind is not "dm" or "comment".
var ErrInvalidTestEventKind = fmt.Errorf("%w: kind must be 'dm' or 'comment'", ErrTriggerValidation)

// Test runs the matcher against a fake event for a single trigger, without touching
// Redis (no cooldown/counter reads) or recording any state. Ownership is verified
// before the test runs so users can only test triggers on accounts they own.
//
// The endpoint is gated by auth only (no active-subscription requirement) so users
// without a paid plan can still author and preview triggers before purchasing.
func (s *TriggerService) Test(ctx context.Context, userID, triggerID string, p TestParams) (engine.TestResult, error) {
	t, err := s.triggers.GetByID(ctx, triggerID)
	if err != nil {
		return engine.TestResult{}, err
	}
	if err := s.ownAccount(ctx, userID, t.AccountID); err != nil {
		return engine.TestResult{}, repository.ErrNotFound
	}
	switch p.Kind {
	case TestEventDM, TestEventComment:
	default:
		return engine.TestResult{}, ErrInvalidTestEventKind
	}
	ev := engine.IncomingEvent{
		Kind:           engine.EventKind(p.Kind),
		AccountID:      t.AccountID,
		SenderID:       p.SenderID,
		SenderUsername: p.SenderUsername,
		SenderName:     p.SenderName,
		Text:           p.Text,
		OccurredAt:     time.Now(),
	}
	return engine.TestTrigger(t, ev), nil
}

// Delete removes a trigger after ownership check.
func (s *TriggerService) Delete(ctx context.Context, userID, triggerID string) error {
	existing, err := s.triggers.GetByID(ctx, triggerID)
	if err != nil {
		return err
	}
	if err := s.ownAccount(ctx, userID, existing.AccountID); err != nil {
		return repository.ErrNotFound
	}
	if err := s.triggers.Delete(ctx, triggerID); err != nil {
		return err
	}
	s.publishReload(ctx, existing.AccountID)
	return nil
}

func (s *TriggerService) ownAccount(ctx context.Context, userID, accountID string) error {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return repository.ErrNotFound
	}
	return nil
}

func (s *TriggerService) checkLimit(ctx context.Context, userID, accountID string) error {
	sub, err := s.subs.GetActive(ctx, userID)
	if err != nil {
		return ErrTriggerLimitExceeded
	}
	limits := PlanLimitsByName(sub.Plan)
	if limits.MaxTriggersPerAccount <= 0 {
		return nil
	}
	count, err := s.triggers.CountByAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("service.trigger.checkLimit: %w", err)
	}
	if int(count) >= limits.MaxTriggersPerAccount {
		return ErrTriggerLimitExceeded
	}
	return nil
}

func validateTrigger(p repository.TriggerParams) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrTriggerValidation)
	}
	switch p.EventType {
	case domain.EventTypeDM, domain.EventTypeComment, domain.EventTypeCommentAndDM:
	default:
		return ErrInvalidEventType
	}
	switch p.MatchMode {
	case domain.MatchModeKeyword, domain.MatchModeAll, domain.MatchModeRegex:
	default:
		return ErrInvalidMatchMode
	}
	switch p.KeywordsMode {
	case domain.KeywordsModeContains, domain.KeywordsModeExact, domain.KeywordsModeStartsWith:
	default:
		return ErrInvalidKeywordsMode
	}
	if p.MatchMode == domain.MatchModeKeyword && len(p.Keywords) == 0 {
		return fmt.Errorf("%w: keyword match_mode requires at least one keyword", ErrTriggerValidation)
	}
	if p.MatchMode == domain.MatchModeRegex {
		if len(p.Keywords) == 0 || strings.TrimSpace(p.Keywords[0]) == "" {
			return ErrInvalidRegex
		}
		if _, err := regexp.Compile(p.Keywords[0]); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidRegex, err)
		}
	}

	hasCommentAction := p.ReplyToComment || p.SendPrivateReply
	hasDMAction := p.SendDM
	wantsComment := p.EventType == domain.EventTypeComment || p.EventType == domain.EventTypeCommentAndDM
	wantsDM := p.EventType == domain.EventTypeDM || p.EventType == domain.EventTypeCommentAndDM

	if wantsComment && !hasCommentAction {
		return fmt.Errorf("%w: enable reply_to_comment or send_private_reply", ErrNoAction)
	}
	if wantsDM && !hasDMAction {
		return fmt.Errorf("%w: enable send_dm", ErrNoAction)
	}
	if p.ReplyToComment && emptyPtr(p.ReplyCommentText) {
		return fmt.Errorf("%w: reply_comment_text", ErrMissingActionText)
	}
	if p.SendPrivateReply && emptyPtr(p.PrivateReplyText) {
		return fmt.Errorf("%w: private_reply_text", ErrMissingActionText)
	}
	if p.SendDM && emptyPtr(p.DMText) {
		return fmt.Errorf("%w: dm_text", ErrMissingActionText)
	}

	if p.CheckSubscription {
		if emptyPtr(p.ReplyIfSubscribed) && emptyPtr(p.ReplyIfUnsubscribed) {
			return fmt.Errorf("%w: check_subscription requires at least one of reply_if_subscribed/reply_if_unsubscribed", ErrTriggerValidation)
		}
	}
	if p.CooldownSeconds < 0 {
		return fmt.Errorf("%w: cooldown_seconds must be non-negative", ErrTriggerValidation)
	}
	if p.MaxRepliesPerUser < 0 {
		return fmt.Errorf("%w: max_replies_per_user must be non-negative", ErrTriggerValidation)
	}
	return nil
}

func emptyPtr(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}
