package domain

import "time"

// User is a registered SocialSentry account.
// Password is never serialized; it holds the bcrypt hash from the DB.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	IsBlocked bool      `json:"is_blocked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)
