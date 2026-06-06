package engine

import "github.com/rabb1tof/socialsentry/backend/internal/domain"

// TestReply is one outgoing message a trigger would produce for a simulated event.
// A single trigger can produce multiple replies (e.g. a public comment reply *and* a
// private DM to the commenter).
type TestReply struct {
	// Channel is one of "comment_reply", "private_reply", "dm".
	Channel string `json:"channel"`
	// Text is the templated reply body, with {{name}} / {{keyword}} / {{time}} / {{date}} substituted.
	Text string `json:"text"`
}

// TestResult is the outcome of running a single trigger against a fake event,
// returned by the POST /accounts/:id/triggers/:tid/test endpoint.
type TestResult struct {
	// EventTypeMatched is true when the trigger's event_type allows the requested EventKind.
	EventTypeMatched bool `json:"event_type_matched"`
	// TextMatched is true when the trigger's match_mode + keywords/regex matched the input text.
	TextMatched bool `json:"text_matched"`
	// MatchedKeyword is the keyword (or "*", or the matched regex) that fired.
	MatchedKeyword string `json:"matched_keyword,omitempty"`
	// WouldFire is the simplest summary: would the trigger actually run?
	WouldFire bool `json:"would_fire"`
	// Replies is the list of messages that would be sent. Empty when no enabled action
	// applies to the event kind (e.g. trigger only replies to comments but the test event was a DM).
	Replies []TestReply `json:"replies"`
}

// TestTrigger runs a single trigger through the matching pipeline against a fake event
// WITHOUT touching Redis (no cooldown / counter / cache reads) and WITHOUT recording any state.
// It is the engine's contribution to the "Test trigger" feature in the editor UI.
//
// The function is intentionally a free function (not a method on *TriggerMatcher) because
// it needs no DB / Redis state and can therefore be called by the API process — which doesn't
// otherwise need a matcher instance.
func TestTrigger(t domain.Trigger, ev IncomingEvent) TestResult {
	res := TestResult{Replies: []TestReply{}}
	if !triggerMatchesEvent(&t, ev) {
		return res
	}
	res.EventTypeMatched = true

	matched, ok := evaluateText(&t, ev.Text)
	if !ok {
		return res
	}
	res.TextMatched = true
	res.MatchedKeyword = matched
	res.WouldFire = true

	data := TemplateData{
		SenderName:     ev.SenderName,
		SenderUsername: ev.SenderUsername,
		MatchedKeyword: matched,
		EventTime:      ev.OccurredAt,
	}

	switch ev.Kind {
	case EventKindDM:
		if t.SendDM && t.DMText != nil && *t.DMText != "" {
			res.Replies = append(res.Replies, TestReply{
				Channel: "dm",
				Text:    ApplyTemplate(*t.DMText, data),
			})
		}
	case EventKindComment:
		if t.ReplyToComment && t.ReplyCommentText != nil && *t.ReplyCommentText != "" {
			res.Replies = append(res.Replies, TestReply{
				Channel: "comment_reply",
				Text:    ApplyTemplate(*t.ReplyCommentText, data),
			})
		}
		if t.SendPrivateReply && t.PrivateReplyText != nil && *t.PrivateReplyText != "" {
			res.Replies = append(res.Replies, TestReply{
				Channel: "private_reply",
				Text:    ApplyTemplate(*t.PrivateReplyText, data),
			})
		}
	}
	return res
}
