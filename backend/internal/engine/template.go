// Package engine contains the trigger matcher and worker manager.
package engine

import (
	"strings"
	"time"
)

// TemplateData holds the values used to substitute placeholders in reply texts.
type TemplateData struct {
	SenderName     string
	SenderUsername string
	MatchedKeyword string
	EventTime      time.Time
}

// ResolveReplyText picks the final body for one reply channel and templates it.
//
// When blocked (sender failed the subscription gate) the channel sends the nudge text;
// otherwise it sends the channel's own configured text. Returns ("", false) when the
// resulting text is empty/whitespace so the caller skips that channel. Templating is applied
// here — to whatever text is actually sent — so {{name}} etc. always resolve, including in
// the subscription nudge.
func ResolveReplyText(blocked bool, nudge string, ownText *string, data TemplateData) (string, bool) {
	raw := ""
	if blocked {
		raw = nudge
	} else if ownText != nil {
		raw = *ownText
	}
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	return ApplyTemplate(raw, data), true
}

// ApplyTemplate replaces {{name}}, {{username}}, {{keyword}}, {{time}}, {{date}}
// in the provided text with values from data.
func ApplyTemplate(text string, data TemplateData) string {
	t := data.EventTime
	if t.IsZero() {
		t = time.Now()
	}
	replacer := strings.NewReplacer(
		"{{name}}", data.SenderName,
		"{{username}}", data.SenderUsername,
		"{{keyword}}", data.MatchedKeyword,
		"{{time}}", t.Format("15:04"),
		"{{date}}", t.Format("02.01.2006"),
	)
	return replacer.Replace(text)
}
