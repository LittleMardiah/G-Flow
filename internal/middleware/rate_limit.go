// Package middleware — Redis-backed rate limiter (fixed window).
//
// Token bucket sederhana: INCR counter per (keyPrefix, client IP) dengan
// TTL = window. Fixed window (bukan sliding) — sesuai API_CONTRACT §5.
//
// Fail-open: kalau Redis nil atau error, request di-allow (availability >
// security). Alert diserahkan ke upstream (log/metrics).

package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit returns Gin middleware yang membatasi request per client-IP
// dalam fixed time window.
//
// rdb: Redis client (nil = fail-open, tidak block).
// limit: max request per window.
// window: durasi window (mis. time.Minute).
// keyPrefix: namespace key (mis. "ratelimit:admin-login").
func RateLimit(rdb *redis.Client, limit int, window time.Duration, keyPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}

		key := fmt.Sprintf("%s:%s", keyPrefix, c.ClientIP())
		ctx := c.Request.Context()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Fail-open: Redis error tidak block.
			c.Next()
			return
		}
		if count == 1 {
			_ = rdb.Expire(ctx, key, window).Err()
		}

		remaining := limit - int(count)
		if remaining < 0 {
			remaining = 0
		}
		ttl, _ := rdb.TTL(ctx, key).Result()
		if ttl <= 0 {
			ttl = window
		}
		resetAt := time.Now().Add(ttl).Unix()

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if int(count) > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": fmt.Sprintf("rate limit exceeded. Try again in %d seconds.", int(ttl.Seconds())),
					"details": gin.H{
						"limit":          limit,
						"window_seconds": int(window.Seconds()),
						"reset_at":       time.Unix(resetAt, 0).UTC().Format(time.RFC3339),
					},
				},
			})
			return
		}
		c.Next()
	}
}
