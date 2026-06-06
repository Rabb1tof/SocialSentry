package domain

import "time"

type Subscription struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Plan      string     `json:"plan"`
	IsActive  bool       `json:"is_active"`
	StartsAt  time.Time  `json:"starts_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Note      *string    `json:"note,omitempty"`
	GrantedBy *string    `json:"granted_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

const (
	PlanBasic      = "basic"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)
