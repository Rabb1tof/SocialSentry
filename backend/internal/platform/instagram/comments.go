package instagram

import (
	"context"
	"fmt"
)

// ReplyResponse is the Meta API reply to a /replies POST.
type ReplyResponse struct {
	ID string `json:"id"`
}

type replyPayload struct {
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
}

// ReplyToComment posts a public reply in the thread of a comment.
func (c *Client) ReplyToComment(ctx context.Context, accountID, commentID, pageToken, text string) (string, error) {
	if err := c.CheckRateLimit(ctx, accountID); err != nil {
		return "", err
	}
	body := replyPayload{Message: text, AccessToken: pageToken}
	var out ReplyResponse
	if err := c.doRequest(ctx, "POST", fmt.Sprintf("/%s/replies", commentID), body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}
