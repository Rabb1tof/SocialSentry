package instagram

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	igUserCacheTTL   = time.Hour
	igFollowCacheTTL = time.Minute
)

// UserProfile is the subset of an IG user's profile used for {{name}}/{{username}} templating.
type UserProfile struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

// GetUserProfile resolves a DM sender's name/username from their IGSID. Comment events already
// carry the username in the webhook, but DM events give only the IGSID, so this hits the Graph
// API (GET /{igsid}?fields=name,username) with the Page Access Token. Best-effort + Redis-cached:
// callers treat errors/empties as "unknown" and never block the reply (some IGSIDs aren't
// resolvable depending on permissions).
func (c *Client) GetUserProfile(ctx context.Context, igsid, pageToken string) (UserProfile, error) {
	key := "ig_user:" + igsid
	if c.rdb != nil {
		if v, e := c.rdb.Get(ctx, key).Result(); e == nil {
			if i := strings.IndexByte(v, '\x1f'); i >= 0 {
				return UserProfile{Name: v[:i], Username: v[i+1:]}, nil
			}
			return UserProfile{Name: v}, nil
		}
	}
	var out UserProfile
	q := queryString(map[string]string{
		"fields":       "name,username",
		"access_token": pageToken,
	})
	if err := c.doRequest(ctx, "GET", "/"+igsid+"?"+q, nil, &out); err != nil {
		return UserProfile{}, fmt.Errorf("instagram.GetUserProfile: %w", err)
	}
	if c.rdb != nil {
		_ = c.rdb.Set(ctx, key, out.Name+"\x1f"+out.Username, igUserCacheTTL).Err()
	}
	return out, nil
}

// IsFollower reports whether the messaging user (igsid) follows the business account, via the
// Instagram Messaging User Profile field is_user_follow_business. That field is only populated
// once the user has messaged the business, so this is meaningful for DM events — not comments.
// Requires the instagram_manage_messages permission (already used for sending DMs). Redis-cached
// (short TTL). Best-effort: callers fail open (normal reply) on error.
func (c *Client) IsFollower(ctx context.Context, igsid, pageToken string) (bool, error) {
	key := "ig_follow:" + igsid
	if c.rdb != nil {
		if v, e := c.rdb.Get(ctx, key).Result(); e == nil {
			return v == "1", nil
		}
	}
	var out struct {
		IsUserFollowBusiness bool `json:"is_user_follow_business"`
	}
	q := queryString(map[string]string{
		"fields":       "is_user_follow_business",
		"access_token": pageToken,
	})
	if err := c.doRequest(ctx, "GET", "/"+igsid+"?"+q, nil, &out); err != nil {
		return false, fmt.Errorf("instagram.IsFollower: %w", err)
	}
	if c.rdb != nil {
		val := "0"
		if out.IsUserFollowBusiness {
			val = "1"
		}
		_ = c.rdb.Set(ctx, key, val, igFollowCacheTTL).Err()
	}
	return out.IsUserFollowBusiness, nil
}
