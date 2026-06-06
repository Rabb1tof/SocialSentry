package instagram

import (
	"context"
	"fmt"
)

// SendDMResponse is the Meta API reply to a /messages POST.
type SendDMResponse struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// sendMessagePayload is the request body for POST /{page_id}/messages.
// Recipient is either {id: ig_scoped_user_id} (DM reply) or {comment_id: ...} (Private Reply).
type sendMessagePayload struct {
	Recipient     map[string]string `json:"recipient"`
	Message       map[string]string `json:"message"`
	MessagingType string            `json:"messaging_type"`
	AccessToken   string            `json:"access_token"`
}

// SendDM sends a reply to an Instagram DM.
// senderIGScopedID is the value pulled from the webhook event's sender.id field.
// Meta enforces a 24-hour window from the original user message.
func (c *Client) SendDM(ctx context.Context, accountID, pageID, pageToken, senderIGScopedID, text string) (string, error) {
	if err := c.CheckRateLimit(ctx, accountID); err != nil {
		return "", err
	}
	body := sendMessagePayload{
		Recipient:     map[string]string{"id": senderIGScopedID},
		Message:       map[string]string{"text": text},
		MessagingType: "RESPONSE",
		AccessToken:   pageToken,
	}
	var out SendDMResponse
	if err := c.doRequest(ctx, "POST", fmt.Sprintf("/%s/messages", pageID), body, &out); err != nil {
		return "", err
	}
	return out.MessageID, nil
}

// SendPrivateReply sends a DM in response to a comment.
// The recipient is identified by comment_id (not user id).
// Window: 7 days from the comment, single message only.
func (c *Client) SendPrivateReply(ctx context.Context, accountID, pageID, pageToken, commentID, text string) (string, error) {
	if err := c.CheckRateLimit(ctx, accountID); err != nil {
		return "", err
	}
	body := sendMessagePayload{
		Recipient:     map[string]string{"comment_id": commentID},
		Message:       map[string]string{"text": text},
		MessagingType: "RESPONSE",
		AccessToken:   pageToken,
	}
	var out SendDMResponse
	if err := c.doRequest(ctx, "POST", fmt.Sprintf("/%s/messages", pageID), body, &out); err != nil {
		return "", err
	}
	return out.MessageID, nil
}
