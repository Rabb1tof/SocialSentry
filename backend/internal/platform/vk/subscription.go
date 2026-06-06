package vk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
	"github.com/redis/go-redis/v9"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// SubscriptionCacheTTL is how long a groups.isMember answer is trusted. Kept short so a fresh
// subscribe/unsubscribe is reflected near-real-time (the gate decides whether to send the real
// reply or a "please subscribe" nudge). One isMember call per cache-miss fits VK's 20 rps budget.
const SubscriptionCacheTTL = 30 * time.Second

// SubscriptionChecker implements engine.SubscriptionChecker by calling groups.isMember
// with a Redis-backed cache.
type SubscriptionChecker struct {
	rdb        *redis.Client
	apiVersion string
}

// NewSubscriptionChecker returns a checker. apiVersion is forwarded to the per-call vksdk.VK
// created from the account's decrypted access token.
func NewSubscriptionChecker(rdb *redis.Client, apiVersion string) *SubscriptionChecker {
	return &SubscriptionChecker{rdb: rdb, apiVersion: apiVersion}
}

// IsSubscribed asks VK whether senderID is a member of the community attached to account.
// Only fires for VK accounts; IG accounts cause a no-op false-with-nil-error so the matcher
// falls back to the default reply text.
//
// The caller must pre-decrypt account.AccessToken before invoking. The VK dispatcher satisfies
// this via Dispatcher.decryptedAccount before the matcher reaches ChooseReplyText.
func (s *SubscriptionChecker) IsSubscribed(ctx context.Context, account domain.ConnectedAccount, senderID string) (bool, error) {
	if account.Platform != domain.PlatformVK {
		return false, nil
	}
	groupID, err := strconv.Atoi(account.PlatformID)
	if err != nil {
		return false, fmt.Errorf("vk.IsSubscribed: bad group_id %q: %w", account.PlatformID, err)
	}
	userID, err := strconv.Atoi(senderID)
	if err != nil {
		return false, fmt.Errorf("vk.IsSubscribed: bad sender %q: %w", senderID, err)
	}

	if s.rdb != nil {
		key := cacheKey(groupID, userID)
		switch v, err := s.rdb.Get(ctx, key).Result(); {
		case err == nil:
			return v == "1", nil
		case errors.Is(err, redis.Nil):
			// fall through to API call
		default:
			// transient Redis trouble — don't fail the matcher; just skip cache
		}
	}

	v := vksdk.NewVK(account.AccessToken)
	if s.apiVersion != "" {
		v.Version = s.apiVersion
	}
	result, err := v.GroupsIsMember(vksdk.Params{
		"group_id": groupID,
		"user_id":  userID,
	})
	if err != nil {
		return false, fmt.Errorf("vk.IsSubscribed: %w", err)
	}
	is := result == 1

	if s.rdb != nil {
		val := "0"
		if is {
			val = "1"
		}
		if err := s.rdb.Set(ctx, cacheKey(groupID, userID), val, SubscriptionCacheTTL).Err(); err != nil {
			// best-effort cache write
			_ = err
		}
	}
	return is, nil
}

func cacheKey(groupID, userID int) string {
	return fmt.Sprintf("vk_member:%d:%d", groupID, userID)
}
