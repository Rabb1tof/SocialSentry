package instagram

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// OAuthScopes are the permissions requested when starting the Facebook Login dialog.
// These are FB-Login scopes (graph.facebook.com + Page Access Token flow) — NOT the
// `instagram_business_*` family which belongs to the separate "Instagram API with
// Instagram Login" product (graph.instagram.com + IGAA tokens). Per project policy
// we only use graph.facebook.com, so all scopes here must be FB-Login-compatible.
var OAuthScopes = []string{
	"instagram_basic",
	"instagram_manage_messages",
	"instagram_manage_comments",
	"pages_show_list",
	"pages_read_engagement",
	"pages_manage_metadata",
	"pages_messaging",
}

// AuthDialogBaseURL is the versioned Facebook Login OAuth dialog endpoint.
// Per current Meta docs, the unversioned legacy URL is treated as deprecated and
// frequently surfaces "Этот контент сейчас недоступен" / "content not available"
// for newer apps (especially those configured with Facebook Login for Business).
const AuthDialogBaseURL = "https://www.facebook.com/v25.0/dialog/oauth"

// AuthURL builds the Facebook Login dialog URL the user is redirected to in Step 1.
// state is a CSRF token the caller stashes in a session / cookie for verification on callback.
func AuthURL(appID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", strings.Join(OAuthScopes, ","))
	v.Set("response_type", "code")
	v.Set("state", state)
	return AuthDialogBaseURL + "?" + v.Encode()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// ExchangeCodeForUserToken (Step 2) trades the authorization code for a short-lived user token.
func (c *Client) ExchangeCodeForUserToken(ctx context.Context, appID, appSecret, redirectURI, code string) (string, error) {
	q := queryString(map[string]string{
		"client_id":     appID,
		"client_secret": appSecret,
		"redirect_uri":  redirectURI,
		"code":          code,
	})
	var out tokenResponse
	if err := c.doRequest(ctx, "GET", "/oauth/access_token?"+q, nil, &out); err != nil {
		return "", fmt.Errorf("instagram.ExchangeCode: %w", err)
	}
	return out.AccessToken, nil
}

// ExchangeForLongLivedUserToken (Step 3) extends a short token to ~60 days.
func (c *Client) ExchangeForLongLivedUserToken(ctx context.Context, appID, appSecret, shortToken string) (string, time.Time, error) {
	q := queryString(map[string]string{
		"grant_type":        "fb_exchange_token",
		"client_id":         appID,
		"client_secret":     appSecret,
		"fb_exchange_token": shortToken,
	})
	var out tokenResponse
	if err := c.doRequest(ctx, "GET", "/oauth/access_token?"+q, nil, &out); err != nil {
		return "", time.Time{}, fmt.Errorf("instagram.LongLivedToken: %w", err)
	}
	var expires time.Time
	if out.ExpiresIn > 0 {
		expires = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return out.AccessToken, expires, nil
}

// Page describes one Facebook Page the user manages, as returned by /me/accounts.
type Page struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
}

type pagesResponse struct {
	Data []Page `json:"data"`
}

// GetPages (Step 4) lists the user's pages with per-page access tokens. The caller
// picks the desired page; SocialSentry stores both the page id and its access_token.
func (c *Client) GetPages(ctx context.Context, userToken string) ([]Page, error) {
	var out pagesResponse
	if err := c.doRequest(ctx, "GET", "/me/accounts?"+queryString(map[string]string{"access_token": userToken}), nil, &out); err != nil {
		return nil, fmt.Errorf("instagram.GetPages: %w", err)
	}
	return out.Data, nil
}

type pageIGResponse struct {
	InstagramBusinessAccount struct {
		ID string `json:"id"`
	} `json:"instagram_business_account"`
	ID string `json:"id"`
}

// GetInstagramBusinessAccountID (Step 5) returns the Instagram Business Account ID
// linked to a Facebook Page. This is what gets stored as connected_accounts.platform_id.
func (c *Client) GetInstagramBusinessAccountID(ctx context.Context, pageID, pageToken string) (string, error) {
	q := queryString(map[string]string{
		"fields":       "instagram_business_account",
		"access_token": pageToken,
	})
	var out pageIGResponse
	if err := c.doRequest(ctx, "GET", "/"+pageID+"?"+q, nil, &out); err != nil {
		return "", fmt.Errorf("instagram.GetInstagramBusinessAccountID: %w", err)
	}
	if out.InstagramBusinessAccount.ID == "" {
		return "", fmt.Errorf("instagram.GetInstagramBusinessAccountID: page %s has no linked Instagram Business Account", pageID)
	}
	return out.InstagramBusinessAccount.ID, nil
}

// SubscribePageToApp (Step 6a) tells Meta to deliver Messenger-style webhook events
// (Instagram DMs ride on this) for the FB Page to our endpoint. Fields here MUST come
// from the Messenger Platform's page-object allowlist; `comments` is NOT one of them —
// IG comment webhooks are subscribed separately on the IG user object (see
// SubscribeIGUserToApp below).
func (c *Client) SubscribePageToApp(ctx context.Context, pageID, pageToken string) error {
	q := queryString(map[string]string{
		"subscribed_fields": "messages,messaging_postbacks,messaging_optins",
		"access_token":      pageToken,
	})
	if err := c.doRequest(ctx, "POST", "/"+pageID+"/subscribed_apps?"+q, nil, nil); err != nil {
		return fmt.Errorf("instagram.SubscribePageToApp: %w", err)
	}
	return nil
}

// SubscribeIGUserToApp (Step 6b) subscribes the Instagram Business Account itself
// to the app so that Instagram-object webhooks (comments, mentions, etc.) fire.
// The set of fields the app may subscribe to is controlled in the App Dashboard
// (Webhooks → Instagram object); this call activates whichever of those fields
// the app has enabled. Use the page access token here.
func (c *Client) SubscribeIGUserToApp(ctx context.Context, igUserID, pageToken string) error {
	q := queryString(map[string]string{
		"subscribed_fields": "comments,messages,live_comments,mentions",
		"access_token":      pageToken,
	})
	if err := c.doRequest(ctx, "POST", "/"+igUserID+"/subscribed_apps?"+q, nil, nil); err != nil {
		return fmt.Errorf("instagram.SubscribeIGUserToApp: %w", err)
	}
	return nil
}

// RefreshPageToken extends a page access token. Meta accepts fb_exchange_token for page tokens too.
func (c *Client) RefreshPageToken(ctx context.Context, appID, appSecret, currentToken string) (string, time.Time, error) {
	return c.ExchangeForLongLivedUserToken(ctx, appID, appSecret, currentToken)
}
