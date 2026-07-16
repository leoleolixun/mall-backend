package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	RequestIDHeaderKey       = "X-Request-ID"
	ContextRequestIDKey      = "request_id"
	contextAuditLogFieldsKey = "audit_log_fields"
)

var allowedAuditLogFields = map[string]struct{}{
	"order_id":      {},
	"order_no":      {},
	"payment_no":    {},
	"after_sale_id": {},
	"after_sale_no": {},
	"refund_no":     {},
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeaderKey))
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}
		c.Set(ContextRequestIDKey, requestID)
		c.Header(RequestIDHeaderKey, requestID)
		c.Next()
	}
}

func CurrentRequestID(c *gin.Context) string {
	requestID, _ := c.Get(ContextRequestIDKey)
	value, _ := requestID.(string)
	return value
}

func SetAuditLogField(c *gin.Context, key string, value any) {
	if _, ok := allowedAuditLogFields[key]; !ok {
		return
	}
	fields, _ := c.Get(contextAuditLogFieldsKey)
	values, _ := fields.(map[string]any)
	if values == nil {
		values = make(map[string]any)
	}
	values[key] = value
	c.Set(contextAuditLogFieldsKey, values)
}

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		fields := []zap.Field{
			zap.String("request_id", CurrentRequestID(c)),
			zap.String("method", c.Request.Method),
			zap.String("route", route),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(startedAt)),
			zap.String("client_ip", c.ClientIP()),
			zap.Int64("request_bytes", c.Request.ContentLength),
			zap.Int("response_bytes", c.Writer.Size()),
		}
		if userID, ok := CurrentUserID(c); ok {
			fields = append(fields, zap.Int64("user_id", userID))
		}
		if identity, ok := CurrentMerchant(c); ok {
			fields = append(fields,
				zap.Int64("merchant_account_id", identity.AccountID),
				zap.Int64("merchant_id", identity.MerchantID),
				zap.String("merchant_role", identity.Role),
			)
		}
		if rawFields, ok := c.Get(contextAuditLogFieldsKey); ok {
			if values, typeOK := rawFields.(map[string]any); typeOK {
				for key, value := range values {
					fields = append(fields, zap.Any(key, value))
				}
			}
		}

		if c.Writer.Status() >= http.StatusInternalServerError {
			log.Error("http request completed", fields...)
			return
		}
		log.Info("http request completed", fields...)
	}
}

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error("http request panic recovered",
			zap.String("request_id", CurrentRequestID(c)),
			zap.Any("panic", recovered),
			zap.ByteString("stack", debug.Stack()),
		)
		if !c.Writer.Written() {
			response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "服务内部错误")
		}
		c.Abort()
	})
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
