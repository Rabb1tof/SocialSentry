package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

const (
	refreshCookieName = "socialsentry_refresh"
	refreshCookiePath = "/api/v1/auth"
)

// AuthHandler wires the AuthService into Gin handlers.
type AuthHandler struct {
	svc    *service.AuthService
	users  repository.UserRepo
	isProd bool
}

// NewAuthHandler returns an AuthHandler. isProd controls the Secure cookie flag.
func NewAuthHandler(svc *service.AuthService, users repository.UserRepo, isProd bool) *AuthHandler {
	return &AuthHandler{svc: svc, users: users, isProd: isProd}
}

type authRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid email")
		return
	case errors.Is(err, service.ErrWeakPassword):
		RespondError(c, http.StatusBadRequest, "validation_error", "Password must be at least 8 characters")
		return
	case errors.Is(err, service.ErrUserExists):
		RespondError(c, http.StatusConflict, "user_exists", "Email already registered")
		return
	case err != nil:
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	RespondData(c, http.StatusCreated, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}

	user, tokens, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidCreds):
		RespondError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	case errors.Is(err, service.ErrUserBlocked):
		RespondError(c, http.StatusForbidden, "user_blocked", "Account is blocked")
		return
	case err != nil:
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	h.setRefreshCookie(c, tokens.RefreshTokenRaw, maxAgeSeconds(tokens.RefreshExpiresAt))
	RespondData(c, http.StatusOK, gin.H{
		"access_token": tokens.AccessToken,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		RespondError(c, http.StatusUnauthorized, "unauthorized", "Missing refresh token")
		return
	}

	_, tokens, err := h.svc.Refresh(c.Request.Context(), raw)
	switch {
	case errors.Is(err, service.ErrInvalidRefresh):
		h.clearRefreshCookie(c)
		RespondError(c, http.StatusUnauthorized, "unauthorized", "Invalid or expired refresh token")
		return
	case errors.Is(err, service.ErrUserBlocked):
		h.clearRefreshCookie(c)
		RespondError(c, http.StatusForbidden, "user_blocked", "Account is blocked")
		return
	case err != nil:
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	h.setRefreshCookie(c, tokens.RefreshTokenRaw, maxAgeSeconds(tokens.RefreshExpiresAt))
	RespondData(c, http.StatusOK, gin.H{
		"access_token": tokens.AccessToken,
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookieName)
	_ = h.svc.Logout(c.Request.Context(), raw)
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

// Me handles GET /api/v1/me. Requires the auth middleware to have run.
func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		RespondError(c, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	user, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "User not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	RespondData(c, http.StatusOK, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, value string, maxAgeSec int) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, value, maxAgeSec, refreshCookiePath, "", h.isProd, true)
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", h.isProd, true)
}

func maxAgeSeconds(expiresAt time.Time) int {
	sec := int(time.Until(expiresAt).Seconds())
	if sec < 0 {
		return 0
	}
	return sec
}
