package domain

import (
	"encoding/json"
	"time"
)

// ConnectedAccount represents an Instagram or VK account a user has linked.
// AccessToken is stored encrypted (AES-256-GCM) and never serialized.
type ConnectedAccount struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	Platform       string          `json:"platform"`
	PlatformID     string          `json:"platform_id"`
	DisplayName    *string         `json:"display_name,omitempty"`
	AvatarURL      *string         `json:"avatar_url,omitempty"`
	AccessToken    string          `json:"-"`
	TokenExpiresAt *time.Time      `json:"token_expires_at,omitempty"`
	PageID         *string         `json:"page_id,omitempty"`
	Extra          json.RawMessage `json:"extra,omitempty"`
	IsActive       bool            `json:"is_active"`
	Status         string          `json:"status"`
	StatusMessage  *string         `json:"status_message,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

const (
	PlatformInstagram = "instagram"
	PlatformVK        = "vk"

	AccountStatusRunning = "running"
	AccountStatusPaused  = "paused"
	AccountStatusError   = "error"
)
