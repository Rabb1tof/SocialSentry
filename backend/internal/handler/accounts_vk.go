package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/vk"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// VKConnectHandler accepts a Community Token + group_id and provisions a connected account.
// Unlike Instagram there is no OAuth dance — VK community tokens are pasted in by the user.
type VKConnectHandler struct {
	accounts   *service.AccountService
	rdb        *redis.Client
	apiVersion string
	logger     *zap.Logger
}

// NewVKConnectHandler wires the handler.
func NewVKConnectHandler(accounts *service.AccountService, rdb *redis.Client, apiVersion string, logger *zap.Logger) *VKConnectHandler {
	return &VKConnectHandler{
		accounts:   accounts,
		rdb:        rdb,
		apiVersion: apiVersion,
		logger:     logger,
	}
}

type vkConnectRequest struct {
	GroupID        string `json:"group_id"        binding:"required"`
	CommunityToken string `json:"community_token" binding:"required"`
}

// Connect handles POST /api/v1/accounts/vk/connect.
//
// Flow:
//  1. Validate group_id is a positive integer.
//  2. Build a VK client + call groups.getById to verify the token belongs to the claimed group.
//  3. Persist via AccountService.CreateConnected (which encrypts the token and enforces plan limits).
//  4. Publish "worker:start:<account_id>" so the worker process picks it up immediately.
func (h *VKConnectHandler) Connect(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	if userID == "" {
		respond401(c)
		return
	}

	var req vkConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}

	groupIDInt, err := strconv.Atoi(req.GroupID)
	if err != nil || groupIDInt <= 0 {
		RespondError(c, http.StatusBadRequest, "validation_error", "group_id must be a positive integer")
		return
	}

	client := vk.NewClient(req.CommunityToken, groupIDInt, "pending", h.apiVersion, h.rdb)
	info, err := client.VerifyToken(c.Request.Context())
	if err != nil {
		h.logger.Warn("vk connect: verify failed", zap.Error(err), zap.String("user_id", userID))
		RespondError(c, http.StatusBadRequest, "validation_error", "VK rejected the token / group_id pair")
		return
	}

	displayName := info.Name
	extra, _ := json.Marshal(map[string]any{
		"group_id":   groupIDInt,
		"group_name": info.Name,
	})

	account, err := h.accounts.CreateConnected(c.Request.Context(), repository.CreateAccountParams{
		UserID:      userID,
		Platform:    domain.PlatformVK,
		PlatformID:  req.GroupID,
		DisplayName: &displayName,
		AccessToken: req.CommunityToken,
		Extra:       extra,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrConflict):
			RespondError(c, http.StatusConflict, "conflict", "this VK community is already connected")
		case errors.Is(err, service.ErrAccountLimitExceeded):
			RespondError(c, http.StatusBadRequest, "limit_exceeded", "your plan's account cap has been reached")
		case errors.Is(err, service.ErrAccountPlatformNotAllowed):
			RespondError(c, http.StatusBadRequest, "platform_not_allowed", "your plan does not allow VK alongside an existing platform")
		default:
			h.logger.Error("vk connect: persist", zap.Error(err))
			RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
		return
	}

	// Tell the worker process to spawn a Long Poll goroutine right now.
	if h.rdb != nil {
		if err := h.rdb.Publish(c.Request.Context(), vk.ChannelWorkerStart+":"+account.ID, "").Err(); err != nil {
			h.logger.Warn("vk connect: pubsub publish", zap.Error(err))
		}
	}

	RespondData(c, http.StatusCreated, account)
}
