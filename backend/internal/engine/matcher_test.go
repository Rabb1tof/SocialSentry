package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// stubChecker is a SubscriptionChecker test double.
type stubChecker struct {
	subscribed bool
	err        error
}

func (s stubChecker) IsSubscribed(context.Context, domain.ConnectedAccount, string) (bool, error) {
	return s.subscribed, s.err
}

// fakeRepo is a minimal stub satisfying repository.TriggerRepo for the methods Match needs.
type fakeRepo struct {
	active []domain.Trigger
}

func (f *fakeRepo) Create(context.Context, any) (domain.Trigger, error) { return domain.Trigger{}, nil }
func (f *fakeRepo) GetByID(context.Context, string) (domain.Trigger, error) {
	return domain.Trigger{}, nil
}
func (f *fakeRepo) ListByAccount(context.Context, string) ([]domain.Trigger, error) { return nil, nil }
func (f *fakeRepo) ListActiveByAccount(_ context.Context, _ string) ([]domain.Trigger, error) {
	return f.active, nil
}
func (f *fakeRepo) CountByAccount(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeRepo) Update(context.Context, string, any) (domain.Trigger, error) {
	return domain.Trigger{}, nil
}
func (f *fakeRepo) Toggle(context.Context, string, bool) error { return nil }
func (f *fakeRepo) Delete(context.Context, string) error       { return nil }

func ptr(s string) *string { return &s }

// matcherWith creates a TriggerMatcher backed by the fake repo and no Redis.
// Cooldown / counter checks are no-ops when rdb is nil.
func matcherWith(triggers []domain.Trigger) *TriggerMatcher {
	m := &TriggerMatcher{cacheTTL: time.Minute}
	m.cache.Store("acc-1", triggerCacheEntry{triggers: triggers, loadedAt: time.Now()})
	_ = fakeRepo{} // silence unused if Match doesn't reach repo
	return m
}

func TestEvaluateText_MatchAll(t *testing.T) {
	tr := domain.Trigger{MatchMode: domain.MatchModeAll}
	k, ok := evaluateText(&tr, "anything")
	if !ok || k != "*" {
		t.Fatalf("got %q ok=%v; want * true", k, ok)
	}
}

func TestEvaluateText_KeywordContainsCaseInsensitive(t *testing.T) {
	tr := domain.Trigger{
		MatchMode:    domain.MatchModeKeyword,
		KeywordsMode: domain.KeywordsModeContains,
		Keywords:     []string{"Hello", "WORLD"},
	}
	k, ok := evaluateText(&tr, "Saying hello to everyone")
	if !ok || k != "Hello" {
		t.Fatalf("got %q ok=%v; want Hello true", k, ok)
	}
}

func TestEvaluateText_KeywordExactCaseSensitive(t *testing.T) {
	tr := domain.Trigger{
		MatchMode:     domain.MatchModeKeyword,
		KeywordsMode:  domain.KeywordsModeExact,
		Keywords:      []string{"START"},
		CaseSensitive: true,
	}
	if _, ok := evaluateText(&tr, "start"); ok {
		t.Fatal("expected miss (case-sensitive mismatch)")
	}
	k, ok := evaluateText(&tr, "START")
	if !ok || k != "START" {
		t.Fatalf("got %q ok=%v; want START true", k, ok)
	}
}

func TestEvaluateText_KeywordStartsWith(t *testing.T) {
	tr := domain.Trigger{
		MatchMode:    domain.MatchModeKeyword,
		KeywordsMode: domain.KeywordsModeStartsWith,
		Keywords:     []string{"hi"},
	}
	k, ok := evaluateText(&tr, "Hi there!")
	if !ok || k != "hi" {
		t.Fatalf("got %q ok=%v; want hi true", k, ok)
	}
}

func TestEvaluateText_Regex(t *testing.T) {
	tr := domain.Trigger{
		MatchMode: domain.MatchModeRegex,
		Keywords:  []string{`^order\s+#\d+$`},
	}
	if _, ok := evaluateText(&tr, "random text"); ok {
		t.Fatal("expected regex miss")
	}
	k, ok := evaluateText(&tr, "order #42")
	if !ok || k == "" {
		t.Fatalf("expected regex hit, got %q ok=%v", k, ok)
	}
}

func TestEvaluateText_RegexInvalidReturnsMiss(t *testing.T) {
	tr := domain.Trigger{MatchMode: domain.MatchModeRegex, Keywords: []string{"[bad"}}
	if _, ok := evaluateText(&tr, "anything"); ok {
		t.Fatal("invalid regex should never match")
	}
}

func TestTriggerMatchesEvent_PriorityAndKind(t *testing.T) {
	cases := []struct {
		eventType string
		kind      EventKind
		want      bool
	}{
		{domain.EventTypeDM, EventKindDM, true},
		{domain.EventTypeDM, EventKindComment, false},
		{domain.EventTypeComment, EventKindComment, true},
		{domain.EventTypeComment, EventKindDM, false},
		{domain.EventTypeCommentAndDM, EventKindDM, true},
		{domain.EventTypeCommentAndDM, EventKindComment, true},
	}
	for _, c := range cases {
		tr := domain.Trigger{EventType: c.eventType}
		got := triggerMatchesEvent(&tr, IncomingEvent{Kind: c.kind})
		if got != c.want {
			t.Errorf("event_type=%s kind=%s: got %v want %v", c.eventType, c.kind, got, c.want)
		}
	}
}

func TestMatch_PriorityWinsFirst(t *testing.T) {
	triggers := []domain.Trigger{
		{
			ID:        "low",
			IsActive:  true,
			EventType: domain.EventTypeDM,
			MatchMode: domain.MatchModeAll,
			Priority:  1,
		},
		{
			ID:        "high",
			IsActive:  true,
			EventType: domain.EventTypeDM,
			MatchMode: domain.MatchModeAll,
			Priority:  10,
		},
	}
	// The repo would sort priority DESC; the cache stores rows as-given.
	// We mimic that by ordering manually here.
	ordered := []domain.Trigger{triggers[1], triggers[0]}
	m := matcherWith(ordered)
	res, err := m.Match(context.Background(), domain.ConnectedAccount{}, IncomingEvent{
		Kind: EventKindDM, AccountID: "acc-1", Text: "anything",
	})
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if res == nil || res.Trigger == nil {
		t.Fatal("expected a match")
	}
	if res.Trigger.ID != "high" {
		t.Errorf("priority wrong: got %q want high", res.Trigger.ID)
	}
}

func TestMatch_NoMatchReturnsNil(t *testing.T) {
	m := matcherWith([]domain.Trigger{
		{
			IsActive:     true,
			EventType:    domain.EventTypeDM,
			MatchMode:    domain.MatchModeKeyword,
			KeywordsMode: domain.KeywordsModeContains,
			Keywords:     []string{"foo"},
		},
	})
	res, err := m.Match(context.Background(), domain.ConnectedAccount{}, IncomingEvent{
		Kind: EventKindDM, AccountID: "acc-1", Text: "bar baz",
	})
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil result, got %+v", res)
	}
}

func TestApplyTemplate_AllPlaceholders(t *testing.T) {
	at := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	got := ApplyTemplate("Hello {{name}} ({{username}})! You said {{keyword}} at {{time}} on {{date}}.",
		TemplateData{
			SenderName:     "Alice",
			SenderUsername: "alice42",
			MatchedKeyword: "hello",
			EventTime:      at,
		},
	)
	want := "Hello Alice (alice42)! You said hello at 14:30 on 15.03.2026."
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestApplyTemplate_ZeroTimeUsesNow(t *testing.T) {
	// We can't assert the exact value, but it should not be the literal Go zero-year.
	got := ApplyTemplate("date={{date}}", TemplateData{})
	if got == "date=01.01.0001" {
		t.Fatalf("expected zero time to fall back to time.Now; got %q", got)
	}
}

func TestSubscriptionGate(t *testing.T) {
	vk := domain.ConnectedAccount{Platform: domain.PlatformVK, PlatformID: "1"}
	ig := domain.ConnectedAccount{Platform: domain.PlatformInstagram}
	const nudge = "subscribe pls"
	on := &domain.Trigger{CheckSubscription: true, ReplyIfUnsubscribed: ptr(nudge)}
	off := &domain.Trigger{CheckSubscription: false}

	tests := []struct {
		name        string
		trig        *domain.Trigger
		acc         domain.ConnectedAccount
		kind        EventKind
		checker     SubscriptionChecker
		wantBlocked bool
		wantNudge   string
	}{
		{"feature off", off, vk, EventKindDM, stubChecker{}, false, ""},
		{"vk subscribed (dm)", on, vk, EventKindDM, stubChecker{subscribed: true}, false, ""},
		{"vk not subscribed (dm) -> nudge", on, vk, EventKindDM, stubChecker{subscribed: false}, true, nudge},
		{"vk not subscribed (comment) -> nudge", on, vk, EventKindComment, stubChecker{subscribed: false}, true, nudge},
		{"ig dm subscribed", on, ig, EventKindDM, stubChecker{subscribed: true}, false, ""},
		{"ig dm not subscribed -> nudge", on, ig, EventKindDM, stubChecker{subscribed: false}, true, nudge},
		{"ig comment skips check", on, ig, EventKindComment, stubChecker{subscribed: false}, false, ""},
		{"checker error fails open", on, vk, EventKindDM, stubChecker{err: errors.New("boom")}, false, ""},
		{"nil checker", on, vk, EventKindDM, nil, false, ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := &TriggerMatcher{checker: tc.checker}
			gotNudge, gotBlocked := m.SubscriptionGate(context.Background(), tc.trig, tc.acc, "123", tc.kind)
			if gotBlocked != tc.wantBlocked || gotNudge != tc.wantNudge {
				t.Errorf("SubscriptionGate = (%q, %v), want (%q, %v)", gotNudge, gotBlocked, tc.wantNudge, tc.wantBlocked)
			}
		})
	}
}

func TestResolveReplyText(t *testing.T) {
	data := TemplateData{SenderName: "Иван", SenderUsername: "ivan", MatchedKeyword: "hi"}
	own := ptr("Hello {{name}} (@{{username}}) {{keyword}}")

	if got, ok := ResolveReplyText(false, "nudge", own, data); !ok || got != "Hello Иван (@ivan) hi" {
		t.Errorf("own text: got (%q, %v)", got, ok)
	}
	if got, ok := ResolveReplyText(true, "Подпишись, {{name}}!", own, data); !ok || got != "Подпишись, Иван!" {
		t.Errorf("nudge text: got (%q, %v)", got, ok)
	}
	if _, ok := ResolveReplyText(false, "x", ptr("   "), data); ok {
		t.Error("whitespace own text should yield ok=false")
	}
	if _, ok := ResolveReplyText(false, "x", nil, data); ok {
		t.Error("nil own text should yield ok=false")
	}
	if _, ok := ResolveReplyText(true, "", own, data); ok {
		t.Error("blocked with empty nudge should yield ok=false")
	}
}
