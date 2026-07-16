package middleware

import (
	"context"
	"net/http"
	"strings"

	"go-mall/internal/authorization"
	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/pkg/jwt"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	MerchantPermissionAccountRead    = authorization.MerchantPermissionAccountRead
	MerchantPermissionAccountWrite   = authorization.MerchantPermissionAccountWrite
	MerchantPermissionCustomerRead   = authorization.MerchantPermissionCustomerRead
	MerchantPermissionAfterSaleRead  = authorization.MerchantPermissionAfterSaleRead
	MerchantPermissionAfterSaleWrite = authorization.MerchantPermissionAfterSaleWrite
	MerchantPermissionMarketingRead  = authorization.MerchantPermissionMarketingRead
	MerchantPermissionMarketingWrite = authorization.MerchantPermissionMarketingWrite
	MerchantPermissionSettlementRead = authorization.MerchantPermissionSettlementRead
)

type MerchantAccountLoader interface {
	FindByIDAndMerchantID(ctx context.Context, accountID int64, merchantID int64) (*model.MerchantAccount, error)
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
}

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

func MerchantAuth(jwtCfg config.JWTConfig, accountLoader MerchantAccountLoader, redisClient *redis.Client) gin.HandlerFunc {
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
		account, err := accountLoader.FindByIDAndMerchantID(c.Request.Context(), claims.AccountID, claims.MerchantID)
		if err != nil || account.Status != model.StatusEnabled || !model.IsValidMerchantRole(account.Role) {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家账号不可用")
			c.Abort()
			return
		}
		merchant, err := accountLoader.FindMerchantByID(c.Request.Context(), account.MerchantID)
		if err != nil || merchant.Status != model.StatusEnabled {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商户不可用")
			c.Abort()
			return
		}
		currentVersion, err := redisClient.Get(c.Request.Context(), authorization.MerchantAccountSessionVersionKey(account.ID)).Int64()
		if err == redis.Nil {
			currentVersion = 0
		} else if err != nil {
			response.Error(c, http.StatusServiceUnavailable, response.CodeInternalError, "商家鉴权服务暂不可用")
			c.Abort()
			return
		}
		if claims.SessionVersion != currentVersion {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}

		c.Set(ContextMerchantAccountIDKey, account.ID)
		c.Set(ContextMerchantIDKey, account.MerchantID)
		c.Set(ContextMerchantRoleKey, account.Role)
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
