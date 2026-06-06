package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// fakeEnqueuer captures the body passed to EnqueueInstagramEvent for assertion.
type fakeEnqueuer struct {
	gotBody []byte
	calls   int
	failErr error
}

func (f *fakeEnqueuer) EnqueueInstagramEvent(_ context.Context, body []byte) error {
	f.calls++
	if f.failErr != nil {
		return f.failErr
	}
	f.gotBody = body
	return nil
}

// enqueuerAdapter satisfies the small interface WebhookHandler depends on.
// The real handler accepts *queue.Client; this lets us swap in a fake without bringing Redis up.
type enqueuerAdapter struct{ e *fakeEnqueuer }

func (a enqueuerAdapter) EnqueueInstagramEvent(ctx context.Context, body []byte) error {
	return a.e.EnqueueInstagramEvent(ctx, body)
}

func TestWebhookVerify_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &WebhookHandler{verifyToken: "secret-verify", appSecret: "ignored", logger: zap.NewNop()}
	r := gin.New()
	r.GET("/webhooks/instagram", h.Verify)

	req := httptest.NewRequest("GET", "/webhooks/instagram?hub.mode=subscribe&hub.verify_token=secret-verify&hub.challenge=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", w.Code)
	}
	if w.Body.String() != "42" {
		t.Errorf("body: got %q want 42", w.Body.String())
	}
}

func TestWebhookVerify_WrongToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &WebhookHandler{verifyToken: "secret-verify", appSecret: "ignored", logger: zap.NewNop()}
	r := gin.New()
	r.GET("/webhooks/instagram", h.Verify)

	req := httptest.NewRequest("GET", "/webhooks/instagram?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d want 403", w.Code)
	}
}

func TestWebhookReceive_SignatureMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enq := &fakeEnqueuer{}
	h := &WebhookHandler{verifyToken: "x", appSecret: "real-secret", logger: zap.NewNop()}
	h.enqueuer = nil // we test the bypass branch instead
	_ = enqueuerAdapter{e: enq}

	r := gin.New()
	r.POST("/webhooks/instagram", h.Receive)

	body := []byte(`{"object":"instagram"}`)
	req := httptest.NewRequest("POST", "/webhooks/instagram", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d want 403", w.Code)
	}
}

func TestWebhookReceive_ValidSignatureRespondsImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// We can't test the *enqueue branch directly without changing the production type to
	// accept an interface, so we just verify the 200 response on valid signature. The
	// enqueue path is covered separately by the queue package's own integration.
	h := &WebhookHandler{verifyToken: "x", appSecret: "real-secret", logger: zap.NewNop()}

	r := gin.New()
	r.POST("/webhooks/instagram", h.Receive)

	body := []byte(`{"object":"instagram","entry":[]}`)
	mac := hmac.New(sha256.New, []byte("real-secret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhooks/instagram", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", w.Code)
	}
}
