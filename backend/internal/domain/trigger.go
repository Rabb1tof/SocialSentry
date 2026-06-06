package domain

import "time"

type Trigger struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	EventType string `json:"event_type"`
	MatchMode string `json:"match_mode"`

	Keywords      []string `json:"keywords"`
	KeywordsMode  string   `json:"keywords_mode"`
	CaseSensitive bool     `json:"case_sensitive"`

	ReplyToComment   bool    `json:"reply_to_comment"`
	ReplyCommentText *string `json:"reply_comment_text,omitempty"`
	SendPrivateReply bool    `json:"send_private_reply"`
	PrivateReplyText *string `json:"private_reply_text,omitempty"`

	SendDM bool    `json:"send_dm"`
	DMText *string `json:"dm_text,omitempty"`

	CheckSubscription   bool    `json:"check_subscription"`
	ReplyIfSubscribed   *string `json:"reply_if_subscribed,omitempty"`
	ReplyIfUnsubscribed *string `json:"reply_if_unsubscribed,omitempty"`

	CooldownSeconds   int `json:"cooldown_seconds"`
	MaxRepliesPerUser int `json:"max_replies_per_user"`
	Priority          int `json:"priority"`

	// ReplyDelaySeconds delays the reply by N seconds after the incoming event
	// (0 = reply immediately). Makes the bot feel less robotic.
	ReplyDelaySeconds int `json:"reply_delay_seconds"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	EventTypeDM           = "dm"
	EventTypeComment      = "comment"
	EventTypeCommentAndDM = "comment_and_dm"

	MatchModeKeyword = "keyword"
	MatchModeAll     = "all"
	MatchModeRegex   = "regex"

	KeywordsModeContains   = "contains"
	KeywordsModeExact      = "exact"
	KeywordsModeStartsWith = "starts_with"
)
