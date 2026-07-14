package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-mall/internal/authorization"
	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/pkg/jwt"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type fakeMerchantAccountLoader struct {
	account  model.MerchantAccount
	merchant model.Merchant
}

func (l *fakeMerchantAccountLoader) FindByIDAndMerchantID(_ context.Context, accountID int64, merchantID int64) (*model.MerchantAccount, error) {
	if l.account.ID != accountID || l.account.MerchantID != merchantID {
		return nil, fmt.Errorf("account not found")
	}
	account := l.account
	return &account, nil
}

func (l *fakeMerchantAccountLoader) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if l.merchant.ID != merchantID {
		return nil, fmt.Errorf("merchant not found")
	}
	merchant := l.merchant
	return &merchant, nil
}

func merchantAuthTestDependencies(t *testing.T) (*fakeMerchantAccountLoader, *redis.Client) {
	t.Helper()
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	return &fakeMerchantAccountLoader{
		account:  model.MerchantAccount{ID: 11, MerchantID: 3, Role: model.MerchantRoleOwner, Status: model.StatusEnabled},
		merchant: model.Merchant{ID: 3, Status: model.StatusEnabled},
	}, redisClient
}

func TestMerchantAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.JWTConfig{MerchantAccessSecret: "merchant-secret"}
	token, err := jwt.GenerateMerchantAccessToken(11, 3, "owner", 0, cfg.MerchantAccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	loader, redisClient := merchantAuthTestDependencies(t)
	router.GET("/merchant", MerchantAuth(cfg, loader, redisClient), func(c *gin.Context) {
		identity, ok := CurrentMerchant(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"account_id":  identity.AccountID,
			"merchant_id": identity.MerchantID,
			"role":        identity.Role,
		})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/merchant", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestMerchantAuthRejectsBuyerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.JWTConfig{MerchantAccessSecret: "shared-secret"}
	buyerToken, err := jwt.GenerateAccessToken(7, cfg.MerchantAccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate buyer token: %v", err)
	}

	router := gin.New()
	loader, redisClient := merchantAuthTestDependencies(t)
	router.GET("/merchant", MerchantAuth(cfg, loader, redisClient), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/merchant", nil)
	request.Header.Set("Authorization", "Bearer "+buyerToken)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestMerchantAuthRejectsRevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.JWTConfig{MerchantAccessSecret: "merchant-secret"}
	token, err := jwt.GenerateMerchantAccessToken(11, 3, "sales", 0, cfg.MerchantAccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	loader, redisClient := merchantAuthTestDependencies(t)
	if err := redisClient.Set(context.Background(), authorization.MerchantAccountSessionVersionKey(11), 1, 0).Err(); err != nil {
		t.Fatalf("set session version: %v", err)
	}

	router := gin.New()
	router.GET("/merchant", MerchantAuth(cfg, loader, redisClient), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/merchant", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestMerchantAuthRejectsDisabledMerchant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.JWTConfig{MerchantAccessSecret: "merchant-secret"}
	token, err := jwt.GenerateMerchantAccessToken(11, 3, model.MerchantRoleOwner, 0, cfg.MerchantAccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	loader, redisClient := merchantAuthTestDependencies(t)
	loader.merchant.Status = model.StatusDisabled

	router := gin.New()
	router.GET("/merchant", MerchantAuth(cfg, loader, redisClient), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/merchant", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestMerchantRolePermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission MerchantPermission
		allowed    bool
	}{
		{role: "owner", permission: MerchantPermissionCatalogWrite, allowed: true},
		{role: "owner", permission: MerchantPermissionAccountWrite, allowed: true},
		{role: "operator", permission: MerchantPermissionAccountRead, allowed: false},
		{role: "sales", permission: MerchantPermissionDashboardRead, allowed: true},
		{role: "sales", permission: MerchantPermissionOrderShip, allowed: false},
		{role: "sales", permission: MerchantPermissionCustomerRead, allowed: true},
		{role: "warehouse", permission: MerchantPermissionOrderShip, allowed: true},
		{role: "warehouse", permission: MerchantPermissionInventoryWrite, allowed: true},
		{role: "warehouse", permission: MerchantPermissionDashboardRead, allowed: false},
		{role: "warehouse", permission: MerchantPermissionCustomerRead, allowed: false},
	}
	for _, test := range tests {
		if got := MerchantRoleHasPermission(test.role, test.permission); got != test.allowed {
			t.Errorf("role=%s permission=%s got=%v want=%v", test.role, test.permission, got, test.allowed)
		}
	}
}

func TestRequireMerchantPermissionReturnsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ship", func(c *gin.Context) {
		c.Set(ContextMerchantAccountIDKey, int64(11))
		c.Set(ContextMerchantIDKey, int64(3))
		c.Set(ContextMerchantRoleKey, "sales")
		c.Next()
	}, RequireMerchantPermission(MerchantPermissionOrderShip), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ship", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
