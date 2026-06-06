package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// TriggerHandler wires trigger CRUD into HTTP.
type TriggerHandler struct {
	svc     *service.TriggerService
	accSvc  *service.AccountService
	logRepo repository.LogRepo
}

// NewTriggerHandler returns a TriggerHandler.
func NewTriggerHandler(svc *service.TriggerService, accSvc *service.AccountService, logRepo repository.LogRepo) *TriggerHandler {
	return &TriggerHandler{svc: svc, accSvc: accSvc, logRepo: logRepo}
}

type triggerBody struct {
	Name                string   `json:"name"`
	IsActive            bool     `json:"is_active"`
	EventType           string   `json:"event_type"`
	MatchMode           string   `json:"match_mode"`
	Keywords            []string `json:"keywords"`
	KeywordsMode        string   `json:"keywords_mode"`
	CaseSensitive       bool     `json:"case_sensitive"`
	ReplyToComment      bool     `json:"reply_to_comment"`
	ReplyCommentText    *string  `json:"reply_comment_text"`
	SendPrivateReply    bool     `json:"send_private_reply"`
	PrivateReplyText    *string  `json:"private_reply_text"`
	SendDM              bool     `json:"send_dm"`
	DMText              *string  `json:"dm_text"`
	CheckSubscription   bool     `json:"check_subscription"`
	ReplyIfSubscribed   *string  `json:"reply_if_subscribed"`
	ReplyIfUnsubscribed *string  `json:"reply_if_unsubscribed"`
	CooldownSeconds     int32    `json:"cooldown_seconds"`
	MaxRepliesPerUser   int32    `json:"max_replies_per_user"`
	Priority            int32    `json:"priority"`
	ReplyDelaySeconds   int32    `json:"reply_delay_seconds"`
}

func (b triggerBody) toParams(accountID string) repository.TriggerParams {
	return repository.TriggerParams{
		AccountID:           accountID,
		Name:                b.Name,
		IsActive:            b.IsActive,
		EventType:           b.EventType,
		MatchMode:           b.MatchMode,
		Keywords:            b.Keywords,
		KeywordsMode:        b.KeywordsMode,
		CaseSensitive:       b.CaseSensitive,
		ReplyToComment:      b.ReplyToComment,
		ReplyCommentText:    b.ReplyCommentText,
		SendPrivateReply:    b.SendPrivateReply,
		PrivateReplyText:    b.PrivateReplyText,
		SendDM:              b.SendDM,
		DMText:              b.DMText,
		CheckSubscription:   b.CheckSubscription,
		ReplyIfSubscribed:   b.ReplyIfSubscribed,
		ReplyIfUnsubscribed: b.ReplyIfUnsubscribed,
		CooldownSeconds:     b.CooldownSeconds,
		MaxRepliesPerUser:   b.MaxRepliesPerUser,
		Priority:            b.Priority,
		ReplyDelaySeconds:   b.ReplyDelaySeconds,
	}
}

// List handles GET /accounts/:id/triggers (open to anyone with the account).
func (h *TriggerHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	accountID := c.Param("id")
	triggers, err := h.svc.ListByAccount(c.Request.Context(), userID, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Account not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": triggers})
}

// Create handles POST /accounts/:id/triggers.
func (h *TriggerHandler) Create(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	accountID := c.Param("id")
	var body triggerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	t, err := h.svc.Create(c.Request.Context(), userID, accountID, body.toParams(accountID))
	if err != nil {
		writeTriggerError(c, err)
		return
	}
	RespondData(c, http.StatusCreated, t)
}

// Get handles GET /accounts/:id/triggers/:tid.
func (h *TriggerHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	t, err := h.svc.Get(c.Request.Context(), userID, c.Param("tid"))
	if err != nil {
		writeTriggerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, t)
}

// Update handles PUT /accounts/:id/triggers/:tid.
func (h *TriggerHandler) Update(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	accountID := c.Param("id")
	triggerID := c.Param("tid")
	var body triggerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	t, err := h.svc.Update(c.Request.Context(), userID, triggerID, body.toParams(accountID))
	if err != nil {
		writeTriggerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, t)
}

type toggleBody struct {
	IsActive bool `json:"is_active"`
}

// Toggle handles PATCH /accounts/:id/triggers/:tid/toggle.
func (h *TriggerHandler) Toggle(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	var body toggleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	if err := h.svc.Toggle(c.Request.Context(), userID, c.Param("tid"), body.IsActive); err != nil {
		writeTriggerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Delete handles DELETE /accounts/:id/triggers/:tid.
func (h *TriggerHandler) Delete(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	if err := h.svc.Delete(c.Request.Context(), userID, c.Param("tid")); err != nil {
		writeTriggerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type testTriggerBody struct {
	Text           string `json:"text"`
	SenderID       string `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	SenderUsername string `json:"sender_username"`
	Kind           string `json:"kind"` // "dm" or "comment"
}

// Test handles POST /accounts/:id/triggers/:tid/test.
// Runs the matcher pipeline against a fake event for a single trigger without
// touching Redis or writing any logs. Auth-only (no subscription required).
func (h *TriggerHandler) Test(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	var body testTriggerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	kind := body.Kind
	if kind == "" {
		kind = "dm"
	}
	result, err := h.svc.Test(c.Request.Context(), userID, c.Param("tid"), service.TestParams{
		Text:           body.Text,
		SenderID:       body.SenderID,
		SenderName:     body.SenderName,
		SenderUsername: body.SenderUsername,
		Kind:           service.TestEventKind(kind),
	})
	if err != nil {
		writeTriggerError(c, err)
		return
	}
	RespondData(c, http.StatusOK, result)
}

// Logs handles GET /accounts/:id/triggers/:tid/logs.
func (h *TriggerHandler) Logs(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	triggerID := c.Param("tid")
	// Verify ownership via the underlying trigger lookup.
	if _, err := h.svc.Get(c.Request.Context(), userID, triggerID); err != nil {
		writeTriggerError(c, err)
		return
	}
	limit, offset := parsePagination(c)
	logs, err := h.logRepo.ListByTrigger(c.Request.Context(), triggerID, limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": logs,
		"meta": gin.H{"limit": limit, "offset": offset, "count": len(logs)},
	})
}

// AccountLogs handles GET /accounts/:id/logs.
func (h *TriggerHandler) AccountLogs(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	accountID := c.Param("id")
	// Ownership check.
	if _, err := h.accSvc.Get(c.Request.Context(), userID, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "not_found", "Account not found")
			return
		}
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	limit, offset := parsePagination(c)
	logs, total, err := h.logRepo.ListByAccount(c.Request.Context(), accountID, limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": logs,
		"meta": gin.H{"limit": limit, "offset": offset, "total": total},
	})
}

// RecentLogs handles GET /logs/recent — recent trigger logs across all of the caller's
// accounts. Auth-only (no subscription gate) so the dashboard always shows activity.
func (h *TriggerHandler) RecentLogs(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	limit, offset := parsePagination(c)
	logs, err := h.logRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": logs,
		"meta": gin.H{"limit": limit, "offset": offset, "count": len(logs)},
	})
}

func writeTriggerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		RespondError(c, http.StatusNotFound, "not_found", "Trigger not found")
	case errors.Is(err, service.ErrTriggerLimitExceeded):
		RespondError(c, http.StatusBadRequest, "limit_exceeded", "Trigger limit for current plan reached")
	case errors.Is(err, service.ErrTriggerValidation):
		RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
	default:
		RespondError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
