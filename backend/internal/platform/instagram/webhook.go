package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// VerifySignature returns true when the HMAC-SHA256 of body, keyed with appSecret,
// matches the value in the X-Hub-Signature-256 header (sent by Meta on every POST).
// The header has the form "sha256=<hex>".
func VerifySignature(body []byte, headerValue, appSecret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(headerValue, prefix) {
		return false
	}
	want, err := hex.DecodeString(headerValue[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), want)
}

// WebhookEnvelope is the top-level webhook payload Meta sends to our endpoint.
type WebhookEnvelope struct {
	Object string         `json:"object"`
	Entry  []WebhookEntry `json:"entry"`
}

// WebhookEntry corresponds to one page's events in a batch.
type WebhookEntry struct {
	ID        string           `json:"id"` // Facebook Page ID
	Time      int64            `json:"time"`
	Messaging []MessagingEvent `json:"messaging,omitempty"`
	Changes   []FieldChange    `json:"changes,omitempty"`
}

// MessagingEvent is a single DM event under entry.messaging[].
type MessagingEvent struct {
	Sender    SenderInfo `json:"sender"`
	Recipient SenderInfo `json:"recipient"`
	Timestamp int64      `json:"timestamp"`
	Message   *struct {
		MID  string `json:"mid"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
}

// SenderInfo is the {id: "..."} payload Meta uses for both sender and recipient.
type SenderInfo struct {
	ID string `json:"id"`
}

// FieldChange is one entry.changes[] element — used for comments.
type FieldChange struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

// CommentValue is the parsed value of a "comments" field change.
type CommentValue struct {
	From struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	Media struct {
		ID               string `json:"id"`
		MediaProductType string `json:"media_product_type"`
	} `json:"media"`
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ParseEnvelope unmarshals the raw body. The returned envelope can be iterated to extract DMs and comments.
func ParseEnvelope(body []byte) (*WebhookEnvelope, error) {
	var env WebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("instagram.ParseEnvelope: %w", err)
	}
	return &env, nil
}

// IterDMs walks the envelope and yields every message event found.
// fn returns the FB page id and the messaging event for each.
func (e *WebhookEnvelope) IterDMs(fn func(pageID string, ev MessagingEvent)) {
	for _, entry := range e.Entry {
		for _, ev := range entry.Messaging {
			if ev.Message == nil {
				continue
			}
			fn(entry.ID, ev)
		}
	}
}

// IterComments walks the envelope and yields every comment event found.
// Skips entries whose change.field is not "comments".
func (e *WebhookEnvelope) IterComments(fn func(pageID string, c CommentValue)) error {
	for _, entry := range e.Entry {
		for _, ch := range entry.Changes {
			if ch.Field != "comments" {
				continue
			}
			var cv CommentValue
			if err := json.Unmarshal(ch.Value, &cv); err != nil {
				return fmt.Errorf("instagram.IterComments: %w", err)
			}
			fn(entry.ID, cv)
		}
	}
	return nil
}
