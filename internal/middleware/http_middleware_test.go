package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-mall/pkg/response"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestIDAndStructuredRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("disable trusted proxies: %v", err)
	}
	engine.Use(RequestID(), RequestLogger(zap.New(core)))
	engine.GET("/orders/:id", func(c *gin.Context) {
		c.Set(ContextUserIDKey, int64(7))
		SetAuditLogField(c, "order_no", "O-TEST-1")
		SetAuditLogField(c, "access_token", "must-not-be-logged")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	requestID := uuid.NewString()
	request := httptest.NewRequest(http.MethodGet, "/orders/12", nil)
	request.Header.Set(RequestIDHeaderKey, requestID)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Header().Get(RequestIDHeaderKey) != requestID {
		t.Fatalf("unexpected response: status=%d request_id=%q", recorder.Code, recorder.Header().Get(RequestIDHeaderKey))
	}
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected one request log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != requestID || fields["route"] != "/orders/:id" || fields["user_id"] != int64(7) || fields["order_no"] != "O-TEST-1" {
		t.Fatalf("unexpected structured fields: %+v", fields)
	}
	if _, exists := fields["access_token"]; exists {
		t.Fatal("sensitive field was accepted by the audit log whitelist")
	}
}

func TestRequestIDReplacesInvalidInboundValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(RequestIDHeaderKey, "contains-newline-not-a-uuid")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if _, err := uuid.Parse(recorder.Header().Get(RequestIDHeaderKey)); err != nil {
		t.Fatalf("expected generated UUID request id: %v", err)
	}
}

func TestCORSAllowsConfiguredAndSameOriginRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"https://admin.example.com"}))
	engine.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowedRequest := httptest.NewRequest(http.MethodGet, "/resource", nil)
	allowedRequest.Header.Set("Origin", "https://admin.example.com")
	allowedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(allowedRecorder, allowedRequest)
	if allowedRecorder.Code != http.StatusNoContent || allowedRecorder.Header().Get("Access-Control-Allow-Origin") != "https://admin.example.com" {
		t.Fatalf("configured origin was not allowed: status=%d headers=%v", allowedRecorder.Code, allowedRecorder.Header())
	}

	sameOriginRequest := httptest.NewRequest(http.MethodGet, "https://mall.example.com/resource", nil)
	sameOriginRequest.Host = "mall.example.com"
	sameOriginRequest.Header.Set("Origin", "https://mall.example.com")
	sameOriginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(sameOriginRecorder, sameOriginRequest)
	if sameOriginRecorder.Code != http.StatusNoContent {
		t.Fatalf("same-origin request was rejected: %d", sameOriginRecorder.Code)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS([]string{"https://mall.example.com"}))
	engine.POST("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/resource", nil)
	request.Header.Set("Origin", "https://evil.example.com")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden CORS response, got %d", recorder.Code)
	}
}

func TestIPRateLimitRejectsExcessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("disable trusted proxies: %v", err)
	}
	engine.POST("/login", IPRateLimit(redisClient, "test_login", 2, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		request.RemoteAddr = "192.0.2.10:12345"
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if attempt <= 2 && recorder.Code != http.StatusNoContent {
			t.Fatalf("attempt %d unexpectedly rejected: %d", attempt, recorder.Code)
		}
		if attempt == 3 {
			if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "" {
				t.Fatalf("rate limit response missing: status=%d headers=%v", recorder.Code, recorder.Header())
			}
			var payload response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || payload.Code != response.CodeTooManyRequests {
				t.Fatalf("unexpected rate limit payload: %s", recorder.Body.String())
			}
		}
	}
}

func TestRecoveryHidesPanicDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.ErrorLevel)
	engine := gin.New()
	engine.Use(RequestID(), RequestLogger(zap.New(core)), Recovery(zap.New(core)))
	engine.GET("/panic", func(c *gin.Context) { panic("database-password-should-not-reach-client") })
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError || recorder.Body.String() == "" {
		t.Fatalf("unexpected recovery response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body == "database-password-should-not-reach-client" {
		t.Fatal("panic details leaked to the response")
	}
	if logs.Len() < 1 {
		t.Fatal("panic was not logged")
	}
}
