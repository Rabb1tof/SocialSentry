package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit returns a Gin middleware that enforces a per-IP fixed-window
// counter via Redis INCR + EXPIRE. When the limit is exceeded, the handler
// returns 429 with a Retry-After header.
//
// On Redis errors the middleware fails open: better to let traffic through
// than to wedge auth endpoints during a Redis outage.
func RateLimit(rdb *redis.Client, name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("ratelimit:%s:%s", name, c.ClientIP())

		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			_ = rdb.Expire(ctx, key, window).Err()
		}
		if count > int64(limit) {
			ttl, err := rdb.TTL(ctx, key).Result()
			if err != nil || ttl <= 0 {
				ttl = window
			}
			c.Header("Retry-After", strconv.Itoa(int(ttl.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": fmt.Sprintf("Too many requests. Retry in %d seconds.", int(ttl.Seconds())),
			})
			return
		}
		c.Next()
	}
}
