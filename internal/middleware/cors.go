package middleware

import (
	"net/http"
	"strings"

	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSuffix(strings.TrimSpace(origin), "/")
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSuffix(strings.TrimSpace(c.GetHeader("Origin")), "/")
		if origin == "" {
			c.Next()
			return
		}
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		} else if forwardedProto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
			scheme = strings.ToLower(forwardedProto)
		}
		requestOrigin := scheme + "://" + c.Request.Host
		if origin == strings.TrimSuffix(requestOrigin, "/") {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "跨域来源不允许访问")
			c.Abort()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", RequestIDHeaderKey)
		c.Header("Access-Control-Max-Age", "600")
		c.Header("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
