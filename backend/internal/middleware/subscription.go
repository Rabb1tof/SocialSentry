package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// RequireActiveSubscription blocks requests from users without an active subscription.
// Must be placed after RequireAuth so user_id is already set in the Gin context.
// On success it stores the subscription in the context under key "subscription".
func RequireActiveSubscription(subSvc *service.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(ContextKeyUserID)
		if userID == "" {
			respond401(c)
			return
		}

		sub, err := subSvc.GetActive(c.Request.Context(), userID)
		if err != nil {
			if errors.Is(err, service.ErrSubscriptionNotFound) ||
				errors.Is(err, repository.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":               "subscription_required",
					"message":             "An active subscription is required",
					"subscription_status": "none",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "Internal server error",
			})
			return
		}

		c.Set("subscription", sub)
		c.Next()
	}
}

// RequireAdmin rejects non-admin users with 403.
// Must be placed after RequireAuth.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(ContextKeyRole)
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Admin access required",
			})
			return
		}
		c.Next()
	}
}
