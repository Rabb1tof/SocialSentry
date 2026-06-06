// Package queue wraps Asynq with the SocialSentry-specific task names and payloads.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// TaskInstagramEvent is the Asynq task type for a raw Instagram webhook payload.
const TaskInstagramEvent = "instagram:event"

// Client is the producer side of the queue — call from API handlers to enqueue background work.
type Client struct {
	asynq *asynq.Client
}

// NewClient connects to Redis using the provided URL and returns a producer.
func NewClient(redisURL string) (*Client, error) {
	opt, err := parseRedisURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Client{asynq: asynq.NewClient(opt)}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() error { return c.asynq.Close() }

// InstagramEventPayload is the JSON body persisted in the Asynq task.
type InstagramEventPayload struct {
	RawBody    []byte    `json:"raw_body"`
	ReceivedAt time.Time `json:"received_at"`
}

// EnqueueInstagramEvent stores the raw webhook body for the worker to process.
// Retries default to 3 with exponential backoff (per worker config).
func (c *Client) EnqueueInstagramEvent(ctx context.Context, raw []byte) error {
	payload, err := json.Marshal(InstagramEventPayload{RawBody: raw, ReceivedAt: time.Now()})
	if err != nil {
		return fmt.Errorf("queue.EnqueueInstagramEvent: %w", err)
	}
	if _, err := c.asynq.EnqueueContext(ctx,
		asynq.NewTask(TaskInstagramEvent, payload),
		asynq.MaxRetry(3),
		asynq.Timeout(60*time.Second),
	); err != nil {
		return fmt.Errorf("queue.EnqueueInstagramEvent: %w", err)
	}
	return nil
}

// parseRedisURL converts the standard redis://[:password@]host:port[/db] form into asynq's RedisClientOpt.
func parseRedisURL(rawURL string) (asynq.RedisClientOpt, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("queue.parseRedisURL: %w", err)
	}
	opt := asynq.RedisClientOpt{Addr: u.Host}
	if u.User != nil {
		pwd, _ := u.User.Password()
		opt.Password = pwd
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		fmt.Sscanf(db, "%d", &opt.DB)
	}
	return opt, nil
}
