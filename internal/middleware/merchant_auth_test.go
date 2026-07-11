package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/pkg/jwt"

	"github.com/gin-gonic/gin"
)

func TestMerchantAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.JWTConfig{MerchantAccessSecret: "merchant-secret"}
	token, err := jwt.GenerateMerchantAccessToken(11, 3, "owner", cfg.MerchantAccessSecret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.GET("/merchant", MerchantAuth(cfg), func(c *gin.Context) {
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
	router.GET("/merchant", MerchantAuth(cfg), func(c *gin.Context) {
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

func TestMerchantRolePermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission MerchantPermission
		allowed    bool
	}{
		{role: "owner", permission: MerchantPermissionCatalogWrite, allowed: true},
		{role: "sales", permission: MerchantPermissionDashboardRead, allowed: true},
		{role: "sales", permission: MerchantPermissionOrderShip, allowed: false},
		{role: "warehouse", permission: MerchantPermissionOrderShip, allowed: true},
		{role: "warehouse", permission: MerchantPermissionInventoryWrite, allowed: true},
		{role: "warehouse", permission: MerchantPermissionDashboardRead, allowed: false},
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
