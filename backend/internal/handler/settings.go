package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// SettingsHandler exposes the global per-platform on/off flags.
type SettingsHandler struct {
	settings *service.SettingsService
}

// NewSettingsHandler returns a SettingsHandler.
func NewSettingsHandler(settings *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

// ListPlatformSettings handles GET /api/v1/admin/platform-settings.
func (h *SettingsHandler) ListPlatformSettings(c *gin.Context) {
	items, err := h.settings.List(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusOK, items)
}

type setPlatformEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// SetPlatformEnabled handles PATCH /api/v1/admin/platform-settings/:platform.
func (h *SettingsHandler) SetPlatformEnabled(c *gin.Context) {
	platform := c.Param("platform")
	var req setPlatformEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Field 'enabled' (boolean) is required")
		return
	}
	ps, err := h.settings.SetEnabled(c.Request.Context(), platform, *req.Enabled)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlatform) {
			RespondError(c, http.StatusBadRequest, "validation_error", "Platform must be 'instagram' or 'vk'")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	RespondData(c, http.StatusOK, ps)
}

// PlatformAvailability handles GET /api/v1/platform-settings — an auth-only read used by
// the frontend to hide connect buttons for disabled platforms. Returns a flat map keyed
// by platform, defaulting unknown platforms to enabled.
func (h *SettingsHandler) PlatformAvailability(c *gin.Context) {
	items, err := h.settings.List(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	out := gin.H{domain.PlatformInstagram: true, domain.PlatformVK: true}
	for _, ps := range items {
		out[ps.Platform] = ps.Enabled
	}
	RespondData(c, http.StatusOK, out)
}
