package vk

import (
	"context"
	"fmt"
	"strings"
	"time"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
)

// userCacheTTL is how long a resolved VK user name/screen-name is trusted.
const userCacheTTL = time.Hour

// GetUser resolves a VK user's display name and screen-name (username) for {{name}}/{{username}}
// template substitution. VK Long Poll events carry only a numeric from_id, so this calls
// users.get. Best-effort + Redis-cached: callers treat empty returns as "unknown" and must
// never block a reply on a lookup failure.
func (c *Client) GetUser(ctx context.Context, userID int) (name, username string, err error) {
	key := fmt.Sprintf("vk_user:%d", userID)
	if c.rdb != nil {
		if v, e := c.rdb.Get(ctx, key).Result(); e == nil {
			n, u := splitCachedUser(v)
			return n, u, nil
		}
	}
	if err := c.CheckRateLimit(ctx); err != nil {
		return "", "", err
	}
	resp, err := c.VK.UsersGet(vksdk.Params{
		"user_ids": userID,
		"fields":   "screen_name",
	})
	if err != nil {
		return "", "", fmt.Errorf("vk.GetUser: %w", err)
	}
	if len(resp) == 0 {
		return "", "", nil
	}
	u := resp[0]
	name = strings.TrimSpace(u.FirstName + " " + u.LastName)
	username = u.ScreenName
	if c.rdb != nil {
		_ = c.rdb.Set(ctx, key, name+"\x1f"+username, userCacheTTL).Err()
	}
	return name, username, nil
}

// splitCachedUser unpacks the "name\x1fusername" cache value.
func splitCachedUser(v string) (string, string) {
	if i := strings.IndexByte(v, '\x1f'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}
