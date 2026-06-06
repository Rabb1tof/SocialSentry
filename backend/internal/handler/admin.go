package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// AdminHandler wires admin services into Gin handlers.
type AdminHandler struct {
	admin *service.AdminService
	subs  *service.SubscriptionService
}

// NewAdminHandler returns an AdminHandler.
func NewAdminHandler(admin *service.AdminService, subs *service.SubscriptionService) *AdminHandler {
	return &AdminHandler{admin: admin, subs: subs}
}

// ListUsers handles GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	limit, offset := parsePagination(c)
	users, total, err := h.admin.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": users,
		"meta": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetUser handles GET /api/v1/admin/users/:id — proxied through auth handler's Me.
// Returns the same user detail shape; subscription history is added here.
func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	subs, _, err := h.subs.List(c.Request.Context(), 50, 0)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	// Filter subs belonging to this user
	userSubs := make([]any, 0)
	for _, s := range subs {
		if s.UserID == userID {
			userSubs = append(userSubs, s)
		}
	}
	RespondData(c, http.StatusOK, gin.H{
		"user_id":       userID,
		"subscriptions": userSubs,
	})
}

type createUserRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"     binding:"required"`
}

// CreateUser handles POST /api/v1/admin/users.
// Lets an existing admin provision a fresh account (user OR admin) without
// going through the public /auth/register flow. Useful for onboarding new
// teammates: the admin sets a temporary password and hands it over.
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	u, err := h.admin.CreateUser(c.Request.Context(), req.Email, req.Password, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrConflict):
			RespondError(c, http.StatusConflict, "conflict", "User with this email already exists")
		case errors.Is(err, service.ErrAdminUserValidation):
			RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
		default:
			RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
		return
	}
	RespondData(c, http.StatusCreated, u)
}

type patchUserRequest struct {
	Role      *string `json:"role"`
	IsBlocked *bool   `json:"is_blocked"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}

// PatchUser handles PATCH /api/v1/admin/users/:id.
// Each field is optional and applied independently: role, blocked status, email
// and password can be changed in one call. A duplicate email maps to 409; a
// validation failure (bad email / weak password) maps to 400.
func (h *AdminHandler) PatchUser(c *gin.Context) {
	userID := c.Param("id")
	var req patchUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	// patchUserError maps the shared service/repository error set to an HTTP response.
	// Returns true if it handled (and already responded to) an error.
	patchUserError := func(err error) bool {
		switch {
		case err == nil:
			return false
		case errors.Is(err, repository.ErrNotFound):
			RespondError(c, http.StatusNotFound, "not_found", "User not found")
		case errors.Is(err, repository.ErrConflict):
			RespondError(c, http.StatusConflict, "conflict", "User with this email already exists")
		case errors.Is(err, service.ErrAdminUserValidation):
			RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
		default:
			RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
		return true
	}

	if req.Role != nil {
		if patchUserError(h.admin.SetRole(c.Request.Context(), userID, *req.Role)) {
			return
		}
	}
	if req.IsBlocked != nil {
		if patchUserError(h.admin.SetBlocked(c.Request.Context(), userID, *req.IsBlocked)) {
			return
		}
	}
	if req.Email != nil {
		if patchUserError(h.admin.SetEmail(c.Request.Context(), userID, *req.Email)) {
			return
		}
	}
	if req.Password != nil {
		if patchUserError(h.admin.SetPassword(c.Request.Context(), userID, *req.Password)) {
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// ListSubscriptions handles GET /api/v1/admin/subscriptions
func (h *AdminHandler) ListSubscriptions(c *gin.Context) {
	limit, offset := parsePagination(c)
	subs, total, err := h.subs.List(c.Request.Context(), limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": subs,
		"meta": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

type grantSubRequest struct {
	UserID    string  `json:"user_id"    binding:"required"`
	Plan      string  `json:"plan"       binding:"required"`
	ExpiresAt *string `json:"expires_at"` // RFC3339 or null
	Note      *string `json:"note"`
}

// GrantSubscription handles POST /api/v1/admin/subscriptions
func (h *AdminHandler) GrantSubscription(c *gin.Context) {
	var req grantSubRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "validation_error", "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}
	grantedBy := c.GetString("user_id")
	sub, err := h.subs.Grant(c.Request.Context(), service.GrantParams{
		UserID:    req.UserID,
		Plan:      req.Plan,
		ExpiresAt: expiresAt,
		Note:      req.Note,
		GrantedBy: grantedBy,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlan) {
			RespondError(c, http.StatusBadRequest, "validation_error", "Invalid plan")
			return
		}
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "User not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusCreated, sub)
}

type updateSubRequest struct {
	Plan      string  `json:"plan"       binding:"required"`
	IsActive  bool    `json:"is_active"`
	ExpiresAt *string `json:"expires_at"`
	Note      *string `json:"note"`
}

// UpdateSubscription handles PATCH /api/v1/admin/subscriptions/:id
func (h *AdminHandler) UpdateSubscription(c *gin.Context) {
	id := c.Param("id")
	var req updateSubRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "validation_error", "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}
	sub, err := h.subs.Update(c.Request.Context(), service.UpdateParams{
		ID:        id,
		Plan:      req.Plan,
		IsActive:  req.IsActive,
		ExpiresAt: expiresAt,
		Note:      req.Note,
	})
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Subscription not found")
			return
		}
		if errors.Is(err, service.ErrInvalidPlan) {
			RespondError(c, http.StatusBadRequest, "validation_error", "Invalid plan")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusOK, sub)
}

// RevokeSubscription handles DELETE /api/v1/admin/subscriptions/:id
func (h *AdminHandler) RevokeSubscription(c *gin.Context) {
	id := c.Param("id")
	if err := h.subs.Revoke(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Subscription not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

// GetStats handles GET /api/v1/admin/stats
func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.admin.GetStats(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusOK, gin.H{
		"total_users":          stats.TotalUsers,
		"active_subscriptions": stats.ActiveSubscriptions,
		"active_accounts":      stats.ActiveAccounts,
	})
}

// GetMySubscription handles GET /api/v1/subscription
func (h *AdminHandler) GetMySubscription(c *gin.Context) {
	userID := c.GetString("user_id")
	sub, err := h.subs.GetActive(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			RespondData(c, http.StatusOK, gin.H{"subscription": nil, "status": "none"})
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	// The admin note is an internal field — only expose it to admins. Regular
	// users see their plan/dates but not the operator's note.
	if c.GetString(middleware.ContextKeyRole) != domain.RoleAdmin {
		sub.Note = nil
	}
	RespondData(c, http.StatusOK, gin.H{"subscription": sub, "status": "active"})
}

func parsePagination(c *gin.Context) (limit, offset int) {
	limit = 20
	offset = 0
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}
	return
}
