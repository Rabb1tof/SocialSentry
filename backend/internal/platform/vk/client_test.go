package vk

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestErrorClassifiers verifies the VK error-code helpers used by the dispatcher/worker to
// decide whether to flip an account to 'error'. They must match the underlying *vksdk.Error
// code directly and through fmt.Errorf("%w") wrapping, and ignore unrelated errors.
func TestErrorClassifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		wantAuth     bool
		wantFlood    bool
		wantNoAccess bool
	}{
		{name: "nil error"},
		{name: "auth code 5", err: &vksdk.Error{Code: 5}, wantAuth: true},
		{name: "flood code 9", err: &vksdk.Error{Code: 9}, wantFlood: true},
		{name: "no-access code 15", err: &vksdk.Error{Code: 15}, wantNoAccess: true},
		{name: "wrapped auth", err: fmt.Errorf("vk.SendMessage: %w", &vksdk.Error{Code: 5}), wantAuth: true},
		{name: "wrapped no-access", err: fmt.Errorf("vk.ReplyToWallComment: %w", &vksdk.Error{Code: 15}), wantNoAccess: true},
		{name: "unrelated vk code", err: &vksdk.Error{Code: 1}},
		{name: "non-vk error", err: errors.New("boom")},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAuthError(tc.err); got != tc.wantAuth {
				t.Errorf("IsAuthError = %v, want %v", got, tc.wantAuth)
			}
			if got := IsFloodControl(tc.err); got != tc.wantFlood {
				t.Errorf("IsFloodControl = %v, want %v", got, tc.wantFlood)
			}
			if got := IsNoAccess(tc.err); got != tc.wantNoAccess {
				t.Errorf("IsNoAccess = %v, want %v", got, tc.wantNoAccess)
			}
		})
	}
}

// newTestRedis spins up an in-memory miniredis and returns a connected go-redis client.
// Both are torn down via t.Cleanup.
func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// TestClientCheckRateLimit verifies the per-account token bucket allows up to rateLimit calls
// per window and rejects the next one with ErrRateLimited, refilling once the 1s TTL lapses.
func TestClientCheckRateLimit(t *testing.T) {
	t.Parallel()

	rdb, mr := newTestRedis(t)
	ctx := context.Background()
	c := NewClient("tok", 123, "acc-rl", "5.199", rdb)
	c.SetRateLimit(3)

	for i := 1; i <= 3; i++ {
		if err := c.CheckRateLimit(ctx); err != nil {
			t.Fatalf("call %d within budget: unexpected error: %v", i, err)
		}
	}
	if err := c.CheckRateLimit(ctx); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("4th call: got %v, want ErrRateLimited", err)
	}

	// The counter carries a 1s TTL; advancing past it refills the bucket.
	mr.FastForward(2 * time.Second)
	if err := c.CheckRateLimit(ctx); err != nil {
		t.Fatalf("after TTL expiry: unexpected error: %v", err)
	}
}

// TestClientCheckRateLimitNilRedis confirms a nil Redis disables rate limiting (no-op).
func TestClientCheckRateLimitNilRedis(t *testing.T) {
	t.Parallel()

	c := NewClient("tok", 123, "acc", "5.199", nil)
	if err := c.CheckRateLimit(context.Background()); err != nil {
		t.Fatalf("nil redis must be a no-op, got %v", err)
	}
}

// TestIsCantSendToUser covers the VK code 901 classifier (user disallows community DMs).
func TestIsCantSendToUser(t *testing.T) {
	t.Parallel()
	if !IsCantSendToUser(&vksdk.Error{Code: 901}) {
		t.Error("code 901 should be IsCantSendToUser")
	}
	if !IsCantSendToUser(fmt.Errorf("vk.SendMessage: %w", &vksdk.Error{Code: 901})) {
		t.Error("wrapped code 901 should be IsCantSendToUser")
	}
	if IsCantSendToUser(&vksdk.Error{Code: 15}) {
		t.Error("code 15 should not be IsCantSendToUser")
	}
	if IsCantSendToUser(errors.New("boom")) || IsCantSendToUser(nil) {
		t.Error("non-vk / nil should not be IsCantSendToUser")
	}
}

// TestNextBackoff verifies exponential growth capped at maxWorkerBackoff.
func TestNextBackoff(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want time.Duration }{
		{time.Second, 2 * time.Second},
		{8 * time.Second, 16 * time.Second},
		{20 * time.Second, maxWorkerBackoff},
		{maxWorkerBackoff, maxWorkerBackoff},
	}
	for _, c := range cases {
		if got := nextBackoff(c.in); got != c.want {
			t.Errorf("nextBackoff(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
