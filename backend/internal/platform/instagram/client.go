// Package instagram is a client for the Instagram Graph API (graph.facebook.com).
// All operations go through graph.facebook.com using a Page Access Token —
// no IGAA tokens, no graph.instagram.com.
package instagram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultBaseURL is the production Meta Graph API base. v25.0 (Oct 2025) is
	// the latest stable version Meta supports; older versions deprecate every
	// ~3 months. Override per-client via Client.BaseURL if needed.
	DefaultBaseURL = "https://graph.facebook.com/v25.0"
	// DefaultRateLimit per page per hour.
	DefaultRateLimit       = 200
	defaultRequestTimeout  = 20 * time.Second
	rateLimitWindowSeconds = 3600
)

// Client wraps an HTTP client with retries, structured error decoding, and per-account rate-limiting.
type Client struct {
	BaseURL    string
	httpClient *http.Client
	rdb        *redis.Client
	rateLimit  int
}

// NewClient returns a Client with sane defaults.
func NewClient(rdb *redis.Client) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: defaultRequestTimeout},
		rdb:        rdb,
		rateLimit:  DefaultRateLimit,
	}
}

// SetBaseURL overrides the API base — used by tests against an httptest.Server.
func (c *Client) SetBaseURL(u string) { c.BaseURL = u }

// SetRateLimit overrides the per-account hourly cap.
func (c *Client) SetRateLimit(n int) { c.rateLimit = n }

// APIError is the structured error returned by the Meta Graph API.
type APIError struct {
	Status       int
	Code         int    `json:"code"`
	Subcode      int    `json:"error_subcode"`
	Message      string `json:"message"`
	FBTraceID    string `json:"fbtrace_id"`
	ErrorUserMsg string `json:"error_user_msg"`
}

// Error implements error.
func (e *APIError) Error() string {
	return fmt.Sprintf("instagram: status=%d code=%d subcode=%d: %s", e.Status, e.Code, e.Subcode, e.Message)
}

// IsExpiredToken matches code 190 (invalid / expired access token).
func (e *APIError) IsExpiredToken() bool { return e.Code == 190 }

// IsOutsideWindow matches code 10 + subcode 2534022 (messaging outside 24h window).
func (e *APIError) IsOutsideWindow() bool { return e.Code == 10 && e.Subcode == 2534022 }

// IsRateLimited matches code 613 (rate limit hit at Meta's end).
func (e *APIError) IsRateLimited() bool { return e.Code == 613 }

// ErrRateLimited is returned when the local Redis bucket rejects the call before it hits Meta.
var ErrRateLimited = errors.New("instagram: local rate limit exceeded")

// CheckRateLimit increments the per-account hourly counter and returns ErrRateLimited if it's over.
func (c *Client) CheckRateLimit(ctx context.Context, accountID string) error {
	if c.rdb == nil {
		return nil
	}
	key := fmt.Sprintf("ratelimit:ig:%s", accountID)
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("instagram.CheckRateLimit: %w", err)
	}
	if count == 1 {
		_, _ = c.rdb.Expire(ctx, key, rateLimitWindowSeconds*time.Second).Result()
	}
	if c.rateLimit > 0 && int(count) > c.rateLimit {
		return ErrRateLimited
	}
	return nil
}

// doRequest sends a request with the access_token appended to the query string
// (Meta accepts it either in the query or body — query is cleaner for GET, body for POST).
// It decodes the typed APIError on non-2xx, and the provided target on 2xx.
func (c *Client) doRequest(ctx context.Context, method, path string, body any, target any) error {
	fullURL := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("instagram.doRequest encode: %w", err)
		}
		bodyReader = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("instagram.doRequest build: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("instagram.doRequest send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if target == nil || len(rawBody) == 0 {
			return nil
		}
		if err := json.Unmarshal(rawBody, target); err != nil {
			return fmt.Errorf("instagram.doRequest decode: %w (body=%s)", err, truncate(rawBody, 200))
		}
		return nil
	}

	apiErr := decodeAPIError(rawBody, resp.StatusCode)
	return apiErr
}

func decodeAPIError(body []byte, status int) *APIError {
	var envelope struct {
		Error APIError `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	envelope.Error.Status = status
	if envelope.Error.Message == "" {
		envelope.Error.Message = truncate(body, 200)
	}
	return &envelope.Error
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// queryString joins URL params for GET requests.
func queryString(values map[string]string) string {
	v := url.Values{}
	for k, val := range values {
		v.Set(k, val)
	}
	return v.Encode()
}
