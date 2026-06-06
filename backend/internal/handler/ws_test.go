package handler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/queue"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
)

// newTestRedis spins up an in-memory Redis for pub/sub tests.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// publishUntil keeps publishing payload to the channel until fn() succeeds or the deadline
// passes. Redis pub/sub is fire-and-forget, so this absorbs the race between a subscriber
// activating and the first publish.
func publishUntil(t *testing.T, rdb *redis.Client, channel string, payload []byte, fn func() bool) bool {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := rdb.Publish(context.Background(), channel, payload).Err(); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if fn() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-tick.C:
		}
	}
}

func TestHubFanOutScopesByAccount(t *testing.T) {
	rdb := newTestRedis(t)
	hub := NewHub(rdb, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	owner := &wsClient{accounts: map[string]bool{"acct-1": true}, send: make(chan []byte, 8)}
	other := &wsClient{accounts: map[string]bool{"acct-2": true}, send: make(chan []byte, 8)}
	hub.register(owner)
	hub.register(other)

	payload, _ := json.Marshal(queue.TriggerFiredEvent{
		AccountID: "acct-1", EventType: "dm", ActionTaken: "sent_dm", SenderUsername: "alice",
	})

	got := publishUntil(t, rdb, queue.ChannelTriggerFired, payload, func() bool {
		select {
		case <-owner.send:
			return true
		default:
			return false
		}
	})
	if !got {
		t.Fatal("owner (acct-1) never received the event")
	}

	// The non-owner (acct-2) must not have received anything, even after the bursts above.
	select {
	case <-other.send:
		t.Fatal("non-owner (acct-2) received an event scoped to acct-1")
	default:
	}
}

// fakeLogRepo is a minimal repository.LogRepo that echoes a fixed row.
type fakeLogRepo struct{ row domain.TriggerLog }

func (f *fakeLogRepo) Create(context.Context, repository.CreateLogParams) (domain.TriggerLog, error) {
	return f.row, nil
}
func (f *fakeLogRepo) ListByAccount(context.Context, string, int, int) ([]domain.TriggerLog, int64, error) {
	return nil, 0, nil
}
func (f *fakeLogRepo) ListByTrigger(context.Context, string, int, int) ([]domain.TriggerLog, error) {
	return nil, nil
}
func (f *fakeLogRepo) ListByUser(context.Context, string, int, int) ([]domain.TriggerLog, error) {
	return nil, nil
}

func TestPublishingLogRepoEmitsEvent(t *testing.T) {
	rdb := newTestRedis(t)
	pub := queue.NewPublisher(rdb, zap.NewNop())

	username := "bob"
	inner := &fakeLogRepo{row: domain.TriggerLog{
		ID: "log-1", AccountID: "acct-7", EventType: "comment",
		ActionTaken: "replied_comment", SenderUsername: &username, CreatedAt: time.Now(),
	}}
	repo := queue.NewPublishingLogRepo(inner, pub)

	sub := rdb.Subscribe(context.Background(), queue.ChannelTriggerFired)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()

	// Call Create in a loop until a subscriber-visible event lands (pub/sub race absorber).
	var received queue.TriggerFiredEvent
	deadline := time.After(2 * time.Second)
	ok := false
	for !ok {
		if _, err := repo.Create(context.Background(), repository.CreateLogParams{AccountID: "acct-7"}); err != nil {
			t.Fatalf("create: %v", err)
		}
		select {
		case msg := <-ch:
			if err := json.Unmarshal([]byte(msg.Payload), &received); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			ok = true
		case <-deadline:
			t.Fatal("no trigger_fired event published by the decorator")
		case <-time.After(40 * time.Millisecond):
		}
	}

	if received.AccountID != "acct-7" || received.ActionTaken != "replied_comment" || received.SenderUsername != "bob" {
		t.Fatalf("unexpected event payload: %+v", received)
	}
}
