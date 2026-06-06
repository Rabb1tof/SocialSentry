package vk

import (
	"context"
	"testing"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// TestSubscriptionCheckerIsSubscribed covers the network-free branches: non-VK accounts
// short-circuit to (false, nil), and malformed group_id/sender_id surface an error.
// The groups.isMember happy path requires a live VK API and is exercised via the live smoke.
func TestSubscriptionCheckerIsSubscribed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		account  domain.ConnectedAccount
		senderID string
		wantOK   bool
		wantErr  bool
	}{
		{
			name:     "non-vk account short-circuits to false",
			account:  domain.ConnectedAccount{Platform: domain.PlatformInstagram, PlatformID: "123"},
			senderID: "456",
		},
		{
			name:     "bad group_id surfaces an error",
			account:  domain.ConnectedAccount{Platform: domain.PlatformVK, PlatformID: "not-a-number"},
			senderID: "456",
			wantErr:  true,
		},
		{
			name:     "bad sender_id surfaces an error",
			account:  domain.ConnectedAccount{Platform: domain.PlatformVK, PlatformID: "123"},
			senderID: "not-a-number",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// nil Redis: the non-VK branch returns before touching the cache; the error
			// branches fail on strconv before the cache too.
			c := NewSubscriptionChecker(nil, "5.199")
			ok, err := c.IsSubscribed(context.Background(), tc.account, tc.senderID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK {
				t.Errorf("IsSubscribed = %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

// TestSubscriptionCheckerCacheHit verifies a cached groups.isMember answer is returned straight
// from Redis without hitting the VK API (the seeded token is deliberately not a real one, so a
// cache miss would fail — passing proves the cache-read path short-circuits the network call).
func TestSubscriptionCheckerCacheHit(t *testing.T) {
	t.Parallel()

	rdb, _ := newTestRedis(t)
	ctx := context.Background()
	checker := NewSubscriptionChecker(rdb, "5.199")
	acc := domain.ConnectedAccount{Platform: domain.PlatformVK, PlatformID: "100", AccessToken: "plain-token"}

	tests := []struct {
		name   string
		cached string
		want   bool
	}{
		{name: "cached subscribed", cached: "1", want: true},
		{name: "cached not subscribed", cached: "0", want: false},
	}
	for _, tc := range tests {
		// Subtests share the same cache key, so they run sequentially (no t.Parallel here).
		t.Run(tc.name, func(t *testing.T) {
			if err := rdb.Set(ctx, "vk_member:100:200", tc.cached, 0).Err(); err != nil {
				t.Fatalf("seed cache: %v", err)
			}
			got, err := checker.IsSubscribed(ctx, acc, "200")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsSubscribed = %v, want %v (cache hit must not call VK)", got, tc.want)
			}
		})
	}
}
