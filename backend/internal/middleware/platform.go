package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PlatformEnabledChecker reports whether a platform is globally enabled.
// Implemented by service.SettingsService. Declared here to avoid an import cycle.
type PlatformEnabledChecker interface {
	IsEnabled(ctx context.Context, platform string) (bool, error)
}

// RequirePlatformEnabled rejects requests for a globally-disabled platform with 403.
// Used to gate the account connect endpoints so no new accounts of a disabled platform
// can be linked. Fail-open on checker errors — a transient settings read failure should
// not block legitimate connects.
func RequirePlatformEnabled(checker PlatformEnabledChecker, platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		enabled, err := checker.IsEnabled(c.Request.Context(), platform)
		if err == nil && !enabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "platform_disabled",
				"message":  "This platform is currently disabled by the administrator",
				"platform": platform,
			})
			return
		}
		c.Next()
	}
}
