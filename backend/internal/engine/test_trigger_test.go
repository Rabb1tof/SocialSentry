package engine

import (
	"testing"
	"time"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

func sptr(s string) *string { return &s }

func TestTestTrigger_DMHit(t *testing.T) {
	tr := domain.Trigger{
		EventType:    domain.EventTypeDM,
		MatchMode:    domain.MatchModeKeyword,
		KeywordsMode: domain.KeywordsModeContains,
		Keywords:     []string{"hello"},
		SendDM:       true,
		DMText:       sptr("Hi {{name}}! You said {{keyword}}."),
	}
	ev := IncomingEvent{
		Kind:       EventKindDM,
		SenderName: "Alice",
		Text:       "hello there",
		OccurredAt: time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC),
	}
	got := TestTrigger(tr, ev)

	if !got.EventTypeMatched || !got.TextMatched || !got.WouldFire {
		t.Fatalf("expected all flags true, got %+v", got)
	}
	if got.MatchedKeyword != "hello" {
		t.Errorf("MatchedKeyword: got %q want hello", got.MatchedKeyword)
	}
	if len(got.Replies) != 1 || got.Replies[0].Channel != "dm" {
		t.Fatalf("expected 1 dm reply, got %+v", got.Replies)
	}
	want := "Hi Alice! You said hello."
	if got.Replies[0].Text != want {
		t.Errorf("DM text: got %q want %q", got.Replies[0].Text, want)
	}
}

func TestTestTrigger_TextMiss(t *testing.T) {
	tr := domain.Trigger{
		EventType:    domain.EventTypeDM,
		MatchMode:    domain.MatchModeKeyword,
		KeywordsMode: domain.KeywordsModeContains,
		Keywords:     []string{"never-found"},
		SendDM:       true,
		DMText:       sptr("x"),
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindDM, Text: "hello"})

	if !got.EventTypeMatched {
		t.Error("EventTypeMatched should be true (DM trigger + DM event)")
	}
	if got.TextMatched {
		t.Error("TextMatched should be false")
	}
	if got.WouldFire {
		t.Error("WouldFire should be false")
	}
	if len(got.Replies) != 0 {
		t.Errorf("Replies should be empty, got %d", len(got.Replies))
	}
}

func TestTestTrigger_EventTypeMismatch(t *testing.T) {
	tr := domain.Trigger{
		EventType:    domain.EventTypeDM,
		MatchMode:    domain.MatchModeAll,
		KeywordsMode: domain.KeywordsModeContains,
		SendDM:       true,
		DMText:       sptr("x"),
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindComment, Text: "anything"})

	if got.EventTypeMatched {
		t.Error("EventTypeMatched should be false (DM trigger + comment event)")
	}
	if got.WouldFire {
		t.Error("WouldFire should be false")
	}
}

func TestTestTrigger_CommentBothActions(t *testing.T) {
	tr := domain.Trigger{
		EventType:        domain.EventTypeComment,
		MatchMode:        domain.MatchModeAll,
		KeywordsMode:     domain.KeywordsModeContains,
		ReplyToComment:   true,
		ReplyCommentText: sptr("Public reply"),
		SendPrivateReply: true,
		PrivateReplyText: sptr("Private DM"),
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindComment, Text: "great post!"})

	if !got.WouldFire {
		t.Fatal("WouldFire should be true")
	}
	if len(got.Replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(got.Replies))
	}
	if got.Replies[0].Channel != "comment_reply" || got.Replies[0].Text != "Public reply" {
		t.Errorf("comment_reply: got %+v", got.Replies[0])
	}
	if got.Replies[1].Channel != "private_reply" || got.Replies[1].Text != "Private DM" {
		t.Errorf("private_reply: got %+v", got.Replies[1])
	}
}

func TestTestTrigger_CommentReplyEnabledButEmptyText(t *testing.T) {
	tr := domain.Trigger{
		EventType:        domain.EventTypeComment,
		MatchMode:        domain.MatchModeAll,
		KeywordsMode:     domain.KeywordsModeContains,
		ReplyToComment:   true,
		ReplyCommentText: sptr(""), // empty
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindComment, Text: "x"})
	if !got.WouldFire {
		t.Fatal("WouldFire should be true (matcher fires)")
	}
	if len(got.Replies) != 0 {
		t.Errorf("expected no replies for empty text, got %d", len(got.Replies))
	}
}

func TestTestTrigger_RegexHit(t *testing.T) {
	tr := domain.Trigger{
		EventType: domain.EventTypeDM,
		MatchMode: domain.MatchModeRegex,
		Keywords:  []string{`^order\s+#\d+$`},
		SendDM:    true,
		DMText:    sptr("Got order"),
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindDM, Text: "order #42"})
	if !got.WouldFire {
		t.Fatal("expected WouldFire")
	}
	if got.MatchedKeyword == "" {
		t.Error("expected non-empty MatchedKeyword for regex")
	}
}

func TestTestTrigger_CommentAndDMEventTypeAcceptsEither(t *testing.T) {
	tr := domain.Trigger{
		EventType:    domain.EventTypeCommentAndDM,
		MatchMode:    domain.MatchModeAll,
		KeywordsMode: domain.KeywordsModeContains,
		SendDM:       true,
		DMText:       sptr("dm"),
	}
	got := TestTrigger(tr, IncomingEvent{Kind: EventKindDM, Text: "x"})
	if !got.EventTypeMatched || !got.WouldFire {
		t.Errorf("DM event should fire, got %+v", got)
	}

	tr.ReplyToComment = true
	tr.ReplyCommentText = sptr("comment")
	got = TestTrigger(tr, IncomingEvent{Kind: EventKindComment, Text: "x"})
	if !got.EventTypeMatched || !got.WouldFire {
		t.Errorf("Comment event should fire, got %+v", got)
	}
}
