package vk

import (
	"context"
	"fmt"
	"time"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
)

// SendMessage delivers a DM from the community to a user. random_id must be unique
// across the recent past — we use UnixNano which is good enough for our throughput.
// Returns the platform message id assigned by VK.
func (c *Client) SendMessage(ctx context.Context, userID int, text string) (int, error) {
	if err := c.CheckRateLimit(ctx); err != nil {
		return 0, err
	}
	id, err := c.VK.MessagesSend(vksdk.Params{
		"user_id":   userID,
		"message":   text,
		"random_id": time.Now().UnixNano(),
	})
	if err != nil {
		return 0, fmt.Errorf("vk.SendMessage: %w", err)
	}
	return id, nil
}
