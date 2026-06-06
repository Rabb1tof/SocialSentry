package domain

import "time"

// PlatformSetting is the global admin-controlled on/off state for one platform.
// When Enabled is false the platform's event processing and worker startup are
// suppressed and connecting new accounts of that platform is blocked.
type PlatformSetting struct {
	Platform  string    `json:"platform"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}
