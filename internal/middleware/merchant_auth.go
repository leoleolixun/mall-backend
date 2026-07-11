package middleware

import (
	"net/http"
	"strings"

	"go-mall/internal/authorization"
	"go-mall/internal/config"
	"go-mall/pkg/jwt"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	ContextMerchantAccountIDKey = "merchant_account_id"
	ContextMerchantIDKey        = "merchant_id"
	ContextMerchantRoleKey      = "merchant_role"
)

type MerchantIdentity struct {
	AccountID  int64
	MerchantID int64
	Role       string
}

type MerchantPermission = authorization.MerchantPermission

const (
	MerchantPermissionDashboardRead  = authorization.MerchantPermissionDashboardRead
	MerchantPermissionOrderRead      = authorization.MerchantPermissionOrderRead
	MerchantPermissionOrderShip      = authorization.MerchantPermissionOrderShip
	MerchantPermissionCatalogRead    = authorization.MerchantPermissionCatalogRead
	MerchantPermissionCatalogWrite   = authorization.MerchantPermissionCatalogWrite
	MerchantPermissionInventoryRead  = authorization.MerchantPermissionInventoryRead
	MerchantPermissionInventoryWrite = authorization.MerchantPermissionInventoryWrite
	MerchantPermissionUpload         = authorization.MerchantPermissionUpload
)

func MerchantRoleHasPermission(role string, permission MerchantPermission) bool {
	return authorization.MerchantRoleHasPermission(role, permission)
}

func RequireMerchantPermission(permission MerchantPermission) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := CurrentMerchant(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
			c.Abort()
			return
		}
		if !MerchantRoleHasPermission(identity.Role, permission) {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "当前角色无权执行此操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

func MerchantAuth(jwtCfg config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
			c.Abort()
			return
		}

		claims, err := jwt.ParseMerchantAccessToken(parts[1], jwtCfg.MerchantAccessSecret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "无效的商家 token")
			c.Abort()
			return
		}

		c.Set(ContextMerchantAccountIDKey, claims.AccountID)
		c.Set(ContextMerchantIDKey, claims.MerchantID)
		c.Set(ContextMerchantRoleKey, claims.Role)
		c.Next()
	}
}

func CurrentMerchant(c *gin.Context) (MerchantIdentity, bool) {
	accountID, accountOK := c.Get(ContextMerchantAccountIDKey)
	merchantID, merchantOK := c.Get(ContextMerchantIDKey)
	role, roleOK := c.Get(ContextMerchantRoleKey)
	if !accountOK || !merchantOK || !roleOK {
		return MerchantIdentity{}, false
	}

	accountIDValue, accountTypeOK := accountID.(int64)
	merchantIDValue, merchantTypeOK := merchantID.(int64)
	roleValue, roleTypeOK := role.(string)
	if !accountTypeOK || !merchantTypeOK || !roleTypeOK || accountIDValue <= 0 || merchantIDValue <= 0 {
		return MerchantIdentity{}, false
	}

	return MerchantIdentity{
		AccountID:  accountIDValue,
		MerchantID: merchantIDValue,
		Role:       roleValue,
	}, true
}
