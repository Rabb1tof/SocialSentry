// Package vk is a client + Bots Long Poll dispatcher for VK communities.
// All operations go through the SevereCloud/vksdk/v3 SDK using a Community Token.
package vk

import (
	"context"
	"errors"
	"fmt"
	"time"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultRateLimit is the per-token cap VK enforces (20 rps).
	// We use a slightly conservative 18 to leave headroom for burst.
	DefaultRateLimit       = 18
	rateLimitWindowSeconds = 1
)

// Client wraps a vksdk *VK with a per-account Redis token bucket so that all sends
// (messages, comments, member lookups) share a single rate-limit budget.
type Client struct {
	VK        *vksdk.VK
	GroupID   int
	AccountID string
	rdb       *redis.Client
	rateLimit int
}

// NewClient builds a Client. apiVersion is e.g. "5.199".
func NewClient(token string, groupID int, accountID, apiVersion string, rdb *redis.Client) *Client {
	v := vksdk.NewVK(token)
	if apiVersion != "" {
		v.Version = apiVersion
	}
	return &Client{
		VK:        v,
		GroupID:   groupID,
		AccountID: accountID,
		rdb:       rdb,
		rateLimit: DefaultRateLimit,
	}
}

// SetRateLimit overrides the per-second cap.
func (c *Client) SetRateLimit(n int) { c.rateLimit = n }

// ErrRateLimited is returned when the local Redis bucket rejects the call.
var ErrRateLimited = errors.New("vk: local rate limit exceeded")

// CheckRateLimit increments the per-account per-second counter and returns ErrRateLimited
// when the budget is exhausted. Counter expires after 1s so the bucket refills naturally.
func (c *Client) CheckRateLimit(ctx context.Context) error {
	if c.rdb == nil {
		return nil
	}
	key := fmt.Sprintf("ratelimit:vk:%s", c.AccountID)
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("vk.CheckRateLimit: %w", err)
	}
	if count == 1 {
		_, _ = c.rdb.Expire(ctx, key, rateLimitWindowSeconds*time.Second).Result()
	}
	if c.rateLimit > 0 && int(count) > c.rateLimit {
		return ErrRateLimited
	}
	return nil
}

// IsAuthError reports whether err looks like VK error code 5 (invalid token).
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	var vkErr *vksdk.Error
	if errors.As(err, &vkErr) {
		return vkErr.Code == 5
	}
	return false
}

// IsFloodControl reports whether err looks like VK error code 9 (flood control).
func IsFloodControl(err error) bool {
	if err == nil {
		return false
	}
	var vkErr *vksdk.Error
	if errors.As(err, &vkErr) {
		return vkErr.Code == 9
	}
	return false
}

// IsNoAccess reports whether err looks like VK error code 15 (access denied).
func IsNoAccess(err error) bool {
	if err == nil {
		return false
	}
	var vkErr *vksdk.Error
	if errors.As(err, &vkErr) {
		return vkErr.Code == 15
	}
	return false
}

// IsCantSendToUser reports whether err is VK error code 901 ("Can't send messages for users
// without permission") — the user hasn't allowed messages from the community. Expected for
// comment→DM when the commenter has no open dialog; treated as a benign skip, not a failure.
func IsCantSendToUser(err error) bool {
	if err == nil {
		return false
	}
	var vkErr *vksdk.Error
	if errors.As(err, &vkErr) {
		return vkErr.Code == 901
	}
	return false
}

// GroupInfo describes the community a token is bound to.
type GroupInfo struct {
	ID   int
	Name string
}

// VerifyToken calls groups.getById and returns the community name. Used by the
// connect handler to confirm the token actually belongs to the claimed group.
func (c *Client) VerifyToken(ctx context.Context) (GroupInfo, error) {
	if err := c.CheckRateLimit(ctx); err != nil {
		return GroupInfo{}, err
	}
	resp, err := c.VK.GroupsGetByID(vksdk.Params{
		"group_id": c.GroupID,
	})
	if err != nil {
		return GroupInfo{}, fmt.Errorf("vk.VerifyToken: %w", err)
	}
	if len(resp.Groups) == 0 {
		return GroupInfo{}, errors.New("vk.VerifyToken: no group returned")
	}
	g := resp.Groups[0]
	return GroupInfo{ID: g.ID, Name: g.Name}, nil
}
