package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ConnectResult captures everything SocialSentry needs to persist after a successful OAuth flow.
type ConnectResult struct {
	PageID              string
	PageName            string
	PageAccessToken     string // long-lived page token (encrypt before persisting)
	IGBusinessAccountID string
	TokenExpiresAt      *time.Time
	Extra               json.RawMessage // serialized {page_name: ...}
	// CommentsWebhookError is non-nil when the IG-user-object subscription
	// (comments/mentions webhooks) couldn't be enabled — DMs still work. The
	// caller can surface this to the user as a soft warning.
	CommentsWebhookError error
}

// CompleteOAuth runs the full Step 2–6 sequence from instagram-api.md.
// Caller is responsible for verifying the state CSRF token before invoking this.
// On success the result is ready to be stored via AccountService.CreateConnected.
func (c *Client) CompleteOAuth(ctx context.Context, appID, appSecret, redirectURI, code string) (ConnectResult, error) {
	shortToken, err := c.ExchangeCodeForUserToken(ctx, appID, appSecret, redirectURI, code)
	if err != nil {
		return ConnectResult{}, err
	}
	longToken, _, err := c.ExchangeForLongLivedUserToken(ctx, appID, appSecret, shortToken)
	if err != nil {
		return ConnectResult{}, err
	}
	pages, err := c.GetPages(ctx, longToken)
	if err != nil {
		return ConnectResult{}, err
	}
	if len(pages) == 0 {
		return ConnectResult{}, fmt.Errorf("instagram.CompleteOAuth: user has no Facebook pages")
	}
	// Pick the first page that has MESSAGING task.
	var chosen Page
	for _, p := range pages {
		for _, task := range p.Tasks {
			if task == "MESSAGING" {
				chosen = p
				break
			}
		}
		if chosen.ID != "" {
			break
		}
	}
	if chosen.ID == "" {
		chosen = pages[0]
	}

	igID, err := c.GetInstagramBusinessAccountID(ctx, chosen.ID, chosen.AccessToken)
	if err != nil {
		return ConnectResult{}, err
	}
	// Page-object subscription → enables Instagram DM webhooks (Messenger Platform).
	if err := c.SubscribePageToApp(ctx, chosen.ID, chosen.AccessToken); err != nil {
		return ConnectResult{}, err
	}
	// IG-user-object subscription → enables comment/mention webhooks. Soft-fail:
	// requires the Instagram Graph API product + Webhooks → Instagram object to be
	// configured in the App Dashboard. Until then DMs work, comments don't.
	commentsErr := c.SubscribeIGUserToApp(ctx, igID, chosen.AccessToken)

	// Page tokens from /me/accounts are long-lived; we still set a ~60-day refresh window.
	expiry := time.Now().Add(50 * 24 * time.Hour)
	extra, _ := json.Marshal(map[string]string{"page_name": chosen.Name})

	return ConnectResult{
		PageID:               chosen.ID,
		PageName:             chosen.Name,
		PageAccessToken:      chosen.AccessToken,
		IGBusinessAccountID:  igID,
		TokenExpiresAt:       &expiry,
		Extra:                extra,
		CommentsWebhookError: commentsErr,
	}, nil
}
