// Package middleware contains Gin middleware shared across handlers.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	jwtpkg "github.com/rabb1tof/socialsentry/backend/pkg/jwt"
)

// Gin context keys populated by RequireAuth.
const (
	ContextKeyUserID = "user_id"
	ContextKeyRole   = "user_role"
)

// RequireAuth validates the Authorization: Bearer JWT.
// On success it sets user_id and user_role in the Gin context.
func RequireAuth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, prefix) {
			respond401(c)
			return
		}
		token := strings.TrimPrefix(header, prefix)
		claims, err := jwtpkg.Parse(token, secret)
		if err != nil {
			respond401(c)
			return
		}
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyRole, claims.Role)
		c.Next()
	}
}

func respond401(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": "Missing or invalid access token",
	})
}
