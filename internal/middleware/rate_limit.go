package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func IPRateLimit(redisClient *redis.Client, namespace string, limit int, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		panic("rate limit must be positive")
	}
	if window <= 0 {
		panic("rate limit window must be positive")
	}

	return func(c *gin.Context) {
		digest := sha256.Sum256([]byte(c.ClientIP()))
		key := fmt.Sprintf("mall:rate_limit:%s:%x", namespace, digest[:12])
		count, err := rateLimitScript.Run(c.Request.Context(), redisClient, []string{key}, window.Milliseconds()).Int()
		if err != nil {
			response.Error(c, http.StatusServiceUnavailable, response.CodeInternalError, "认证限流服务暂不可用")
			c.Abort()
			return
		}

		remaining := limit - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if count > limit {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			response.Error(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}
