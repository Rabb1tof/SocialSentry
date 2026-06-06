package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// AccountHandler exposes connected-account endpoints.
type AccountHandler struct {
	svc *service.AccountService
}

// NewAccountHandler wires the handler.
func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// List handles GET /api/v1/accounts.
func (h *AccountHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	accounts, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accounts})
}

// Get handles GET /api/v1/accounts/:id.
func (h *AccountHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	a, err := h.svc.Get(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Account not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusOK, a)
}

// Delete handles DELETE /api/v1/accounts/:id.
func (h *AccountHandler) Delete(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	if err := h.svc.Delete(c.Request.Context(), userID, c.Param("id")); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Account not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

type patchAccountStatusRequest struct {
	Active bool `json:"active"`
}

// PatchStatus handles PATCH /api/v1/accounts/:id/status.
func (h *AccountHandler) PatchStatus(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	var req patchAccountStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	if err := h.svc.SetActive(c.Request.Context(), userID, c.Param("id"), req.Active); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Account not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}
