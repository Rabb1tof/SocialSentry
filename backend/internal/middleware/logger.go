package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ContextKeyRequestID is the key under which the per-request UUID is stored.
const ContextKeyRequestID = "request_id"

// Logger returns a Gin middleware that writes one structured log line per request.
// A UUIDv4 request_id is assigned and echoed in the X-Request-ID response header.
// Authorization headers are never logged.
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.NewString()
		c.Set(ContextKeyRequestID, requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		}
		if uid := c.GetString(ContextKeyUserID); uid != "" {
			fields = append(fields, zap.String("user_id", uid))
		}

		switch status := c.Writer.Status(); {
		case status >= 500:
			logger.Error("request", fields...)
		case status >= 400:
			logger.Warn("request", fields...)
		default:
			logger.Info("request", fields...)
		}
	}
}
