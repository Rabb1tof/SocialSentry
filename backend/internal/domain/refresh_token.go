package domain

import "time"

// RefreshToken is the server-side record of an issued refresh token.
// The Token field stores the SHA-256 hash of the raw token, not the raw value.
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
