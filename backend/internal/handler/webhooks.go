package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/queue"
)

// WebhookHandler exposes Meta webhook endpoints.
type WebhookHandler struct {
	verifyToken string
	appSecret   string
	enqueuer    *queue.Client
	logger      *zap.Logger
}

// NewWebhookHandler wires the handler.
func NewWebhookHandler(verifyToken, appSecret string, enqueuer *queue.Client, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{
		verifyToken: verifyToken,
		appSecret:   appSecret,
		enqueuer:    enqueuer,
		logger:      logger,
	}
}

// Verify handles Meta's GET /webhooks/instagram challenge during webhook subscription setup.
// Meta sends ?hub.mode=subscribe&hub.verify_token=...&hub.challenge=N — we reply with the challenge
// number as plain text when the verify token matches.
func (h *WebhookHandler) Verify(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken {
		c.String(http.StatusOK, challenge)
		return
	}
	c.Status(http.StatusForbidden)
}

// Receive handles POST /webhooks/instagram. Verifies the HMAC signature, immediately ACKs with 200,
// then enqueues the raw body for the worker to process asynchronously. Meta times out after ~20s,
// so we must NOT block on platform API calls inside this handler.
func (h *WebhookHandler) Receive(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	signature := c.GetHeader("X-Hub-Signature-256")
	if !instagram.VerifySignature(body, signature, h.appSecret) {
		// Opt-in diagnostic: when the configured webhook secret fails to verify,
		// and META_WEBHOOK_DEBUG_SECRETS (comma-separated candidate secrets) is set,
		// log which candidate — if any — actually signed this body. This identifies
		// the Meta app that OWNS the subscription so its secret can be set as
		// META_WEBHOOK_APP_SECRET. Off by default; logs only 6-char prefixes, never
		// full secrets or request bodies.
		matchedIndex := -1
		matchedPrefix := ""
		candidates := splitSecrets(os.Getenv("META_WEBHOOK_DEBUG_SECRETS"))
		for i, cand := range candidates {
			cmac := hmac.New(sha256.New, []byte(cand))
			cmac.Write(body)
			if hmac.Equal([]byte("sha256="+hex.EncodeToString(cmac.Sum(nil))), []byte(signature)) {
				matchedIndex = i
				if len(cand) >= 6 {
					matchedPrefix = cand[:6]
				}
				break
			}
		}

		h.logger.Warn("instagram webhook: signature mismatch",
			zap.Int("body_len", len(body)),
			zap.Int("candidate_count", len(candidates)),
			zap.Int("matched_candidate_index", matchedIndex),
			zap.String("matched_candidate_prefix", matchedPrefix),
		)
		c.Status(http.StatusForbidden)
		return
	}
	// Reply 200 immediately — Meta expects this within 20s.
	c.Status(http.StatusOK)

	if h.enqueuer != nil {
		if err := h.enqueuer.EnqueueInstagramEvent(c.Request.Context(), body); err != nil {
			h.logger.Error("instagram webhook: enqueue failed", zap.Error(err))
		}
	}
}

// splitSecrets parses a comma-separated list of candidate app secrets, trimming
// whitespace and dropping empties. TEMP — part of the webhook-fix diagnostic.
func splitSecrets(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
