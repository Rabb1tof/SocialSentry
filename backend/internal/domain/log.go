package domain

import "time"

type TriggerLog struct {
	ID              string    `json:"id"`
	TriggerID       string    `json:"trigger_id"`
	AccountID       string    `json:"account_id"`
	EventType       string    `json:"event_type"`
	PlatformEventID *string   `json:"platform_event_id,omitempty"`
	SenderID        string    `json:"sender_id"`
	SenderUsername  *string   `json:"sender_username,omitempty"`
	IncomingText    *string   `json:"incoming_text,omitempty"`
	MatchedKeyword  *string   `json:"matched_keyword,omitempty"`
	ActionTaken     string    `json:"action_taken"`
	ErrorMessage    *string   `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

const (
	ActionTakenRepliedComment = "replied_comment"
	ActionTakenSentDM         = "sent_dm"
	ActionTakenBoth           = "both"
	ActionTakenSkipped        = "skipped"
	ActionTakenError          = "error"
)
