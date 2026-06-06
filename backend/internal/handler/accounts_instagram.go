package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// InstagramConnectHandler drives the OAuth handshake with Meta.
type InstagramConnectHandler struct {
	cfg         config.Config
	client      *instagram.Client
	accounts    *service.AccountService
	rdb         *redis.Client
	frontendURL string // where to redirect the browser after callback (success or error)
	logger      *zap.Logger
}

// NewInstagramConnectHandler wires the handler.
func NewInstagramConnectHandler(cfg config.Config, client *instagram.Client, accounts *service.AccountService, rdb *redis.Client, frontendURL string, logger *zap.Logger) *InstagramConnectHandler {
	return &InstagramConnectHandler{
		cfg:         cfg,
		client:      client,
		accounts:    accounts,
		rdb:         rdb,
		frontendURL: frontendURL,
		logger:      logger,
	}
}

// Connect handles POST /api/v1/accounts/instagram/connect.
// Generates a CSRF state token, stashes it in Redis bound to the calling user, and returns
// the Facebook Login dialog URL the frontend should redirect the browser to.
func (h *InstagramConnectHandler) Connect(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	if userID == "" {
		respond401(c)
		return
	}
	if h.cfg.Meta.AppID == "" || h.cfg.Meta.CallbackURL == "" {
		RespondError(c, http.StatusServiceUnavailable, "config_missing", "META_APP_ID or META_CALLBACK_URL not configured")
		return
	}

	state, err := newState()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	if err := h.rdb.Set(c.Request.Context(), stateKey(state), userID, 10*time.Minute).Err(); err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	authURL := instagram.AuthURL(h.cfg.Meta.AppID, h.cfg.Meta.CallbackURL, state)
	RespondData(c, http.StatusOK, gin.H{"auth_url": authURL, "state": state})
}

// Callback handles GET /api/v1/accounts/instagram/callback.
// This is a *public* endpoint — the browser arrives here after Meta redirects them, so we
// authenticate via the state token from the original /connect call, not via JWT.
func (h *InstagramConnectHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")
	errDesc := c.Query("error_description")

	if errParam != "" {
		h.redirectFailure(c, errParam, errDesc)
		return
	}
	if code == "" || state == "" {
		h.redirectFailure(c, "bad_request", "missing code or state")
		return
	}

	ctx := c.Request.Context()
	userID, err := h.rdb.GetDel(ctx, stateKey(state)).Result()
	if err != nil || userID == "" {
		h.redirectFailure(c, "invalid_state", "OAuth state expired or unknown")
		return
	}

	result, err := h.client.CompleteOAuth(ctx, h.cfg.Meta.AppID, h.cfg.Meta.AppSecret, h.cfg.Meta.CallbackURL, code)
	if err != nil {
		h.logger.Error("ig OAuth", zap.Error(err), zap.String("user_id", userID))
		h.redirectFailure(c, "oauth_failed", err.Error())
		return
	}
	if result.CommentsWebhookError != nil {
		h.logger.Warn("ig OAuth: comments webhook not enabled — DM-only connection",
			zap.Error(result.CommentsWebhookError),
			zap.String("user_id", userID),
			zap.String("ig_business_account_id", result.IGBusinessAccountID),
		)
	}

	pageName := result.PageName
	account, err := h.accounts.CreateConnected(ctx, repository.CreateAccountParams{
		UserID:         userID,
		Platform:       domain.PlatformInstagram,
		PlatformID:     result.IGBusinessAccountID,
		DisplayName:    &pageName,
		AccessToken:    result.PageAccessToken, // plaintext — encrypted inside the service
		TokenExpiresAt: result.TokenExpiresAt,
		PageID:         &result.PageID,
		Extra:          result.Extra,
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			h.redirectFailure(c, "already_connected", "this Instagram account is already linked")
			return
		}
		if errors.Is(err, service.ErrAccountLimitExceeded) {
			h.redirectFailure(c, "limit_exceeded", "your plan's account cap has been reached")
			return
		}
		if errors.Is(err, service.ErrAccountPlatformNotAllowed) {
			h.redirectFailure(c, "platform_not_allowed", "your plan does not allow Instagram on top of an existing platform")
			return
		}
		h.logger.Error("ig persist", zap.Error(err))
		h.redirectFailure(c, "internal_error", "failed to persist account")
		return
	}

	h.redirectSuccess(c, account.ID)
}

func (h *InstagramConnectHandler) redirectSuccess(c *gin.Context, accountID string) {
	target := h.frontendURL + "/accounts?connected=instagram&account_id=" + url.QueryEscape(accountID)
	c.Redirect(http.StatusFound, target)
}

func (h *InstagramConnectHandler) redirectFailure(c *gin.Context, code, msg string) {
	q := url.Values{}
	q.Set("ig_error", code)
	if msg != "" {
		q.Set("ig_error_message", msg)
	}
	c.Redirect(http.StatusFound, h.frontendURL+"/accounts?"+q.Encode())
}

func stateKey(state string) string { return "ig_oauth_state:" + state }

func newState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("handler.newState: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// respond401 mirrors middleware.respond401 — duplicated here to avoid an import cycle.
func respond401(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": "Missing or invalid access token",
	})
}

// ensure context is used; clears an unused-import warning some linters emit when this file
// is the only one in the package importing context. The constant is a no-op for callers.
var _ context.Context = context.Background()
