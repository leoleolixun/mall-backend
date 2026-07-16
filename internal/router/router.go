package router

import (
	"fmt"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/middleware"
	"go-mall/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
	merchantHandler *handler.MerchantHandler,
	authHandler *handler.AuthHandler,
	addressHandler *handler.AddressHandler,
	cartHandler *handler.CartHandler,
	orderHandler *handler.OrderHandler,
	tradeHandler *handler.TradeHandler,
	paymentHandler *handler.PaymentHandler,
	uploadHandler *handler.UploadHandler,
	afterSaleHandler *handler.AfterSaleHandler,
	couponHandler *handler.CouponHandler,
	favoriteHandler *handler.FavoriteHandler,
	merchantAuthHandler *handler.MerchantAuthHandler,
	merchantAccountHandler *handler.MerchantAccountHandler,
	merchantOrderHandler *handler.MerchantOrderHandler,
	merchantCatalogHandler *handler.MerchantCatalogHandler,
	merchantInventoryHandler *handler.MerchantInventoryHandler,
	merchantDashboardHandler *handler.MerchantDashboardHandler,
	merchantCustomerHandler *handler.MerchantCustomerHandler,
	merchantSettlementHandler *handler.MerchantSettlementHandler,
	merchantAccountLoader middleware.MerchantAccountLoader,
	redisClient *redis.Client,
	jwtCfg config.JWTConfig,
	serverCfg config.ServerConfig,
	authCfg config.AuthConfig,
	paymentCfg config.PaymentConfig,
	observabilityCfg config.ObservabilityConfig,
	log *zap.Logger,
	metrics *observability.Metrics,
) (*gin.Engine, error) {
	r := gin.New()
	if metrics == nil {
		metrics = observability.NewMetrics(nil)
	}
	trustedProxies := serverCfg.TrustedProxies
	if len(trustedProxies) == 0 {
		trustedProxies = nil
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("配置可信代理失败: %w", err)
	}
	r.Use(
		middleware.RequestID(),
		middleware.RequestLogger(log),
		metrics.Middleware(),
		middleware.Recovery(log),
		middleware.SecurityHeaders(),
		middleware.CORS(serverCfg.CORSAllowedOrigins),
	)

	r.GET("/health", healthHandler.Health)
	if observabilityCfg.MetricsEnabled {
		// Nginx 不代理该路径；仅供同机 Prometheus 或运维检查使用。
		r.GET("/metrics", gin.WrapH(metrics.Handler()))
	}
	r.StaticFile("/docs/openapi.yaml", "docs/openapi.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/openapi.yaml")))

	api := r.Group("/api/v1")
	{
		isReleaseMode := gin.Mode() == gin.ReleaseMode || strings.EqualFold(serverCfg.Mode, gin.ReleaseMode)
		developmentFeaturesEnabled := !isReleaseMode

		api.GET("/categories", categoryHandler.List)
		api.GET("/products", productHandler.List)
		api.GET("/products/:id", productHandler.Detail)
		api.GET("/products/:id/skus", productHandler.SKUs)
		api.GET("/merchants", merchantHandler.List)
		api.GET("/merchants/:id", merchantHandler.Detail)
		api.GET("/merchants/:id/categories", merchantHandler.Categories)
		api.GET("/merchants/:id/products", merchantHandler.Products)
		api.POST("/payments/alipay/notify", paymentHandler.AlipayNotify)

		auth := api.Group("/auth")
		{
			auth.POST("/register", middleware.IPRateLimit(redisClient, "buyer_register", authCfg.LoginRateLimitPerMinute, time.Minute), authHandler.Register)
			auth.POST("/login/password", middleware.IPRateLimit(redisClient, "buyer_login", authCfg.LoginRateLimitPerMinute, time.Minute), authHandler.PasswordLogin)
			if authCfg.UnsafeWechatOpenIDLoginEnabled && developmentFeaturesEnabled {
				auth.POST("/login/wechat", middleware.IPRateLimit(redisClient, "buyer_wechat_login", authCfg.LoginRateLimitPerMinute, time.Minute), authHandler.WechatMiniProgramLogin)
			}
			auth.POST("/refresh", middleware.IPRateLimit(redisClient, "buyer_refresh", authCfg.RefreshRateLimitPerMinute, time.Minute), authHandler.Refresh)
		}

		merchantAuth := api.Group("/merchant/auth")
		{
			merchantAuth.POST("/login", middleware.IPRateLimit(redisClient, "merchant_login", authCfg.LoginRateLimitPerMinute, time.Minute), merchantAuthHandler.Login)
			merchantAuth.POST("/refresh", middleware.IPRateLimit(redisClient, "merchant_refresh", authCfg.RefreshRateLimitPerMinute, time.Minute), merchantAuthHandler.Refresh)
		}

		merchantProtected := api.Group("/merchant")
		merchantProtected.Use(middleware.MerchantAuth(jwtCfg, merchantAccountLoader, redisClient))
		{
			merchantProtected.GET("/me", merchantAuthHandler.Me)
			merchantProtected.POST("/auth/logout", merchantAuthHandler.Logout)
			merchantProtected.GET("/accounts", middleware.RequireMerchantPermission(middleware.MerchantPermissionAccountRead), merchantAccountHandler.List)
			merchantProtected.POST("/accounts", middleware.RequireMerchantPermission(middleware.MerchantPermissionAccountWrite), merchantAccountHandler.Create)
			merchantProtected.PUT("/accounts/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionAccountWrite), merchantAccountHandler.Update)
			merchantProtected.PUT("/accounts/:id/password", middleware.RequireMerchantPermission(middleware.MerchantPermissionAccountWrite), merchantAccountHandler.ResetPassword)
			merchantProtected.GET("/roles", middleware.RequireMerchantPermission(middleware.MerchantPermissionAccountRead), merchantAccountHandler.Roles)
			merchantProtected.POST("/uploads", middleware.RequireMerchantPermission(middleware.MerchantPermissionUpload), uploadHandler.Image)
			merchantProtected.GET("/orders", middleware.RequireMerchantPermission(middleware.MerchantPermissionOrderRead), merchantOrderHandler.List)
			merchantProtected.GET("/orders/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionOrderRead), merchantOrderHandler.Detail)
			merchantProtected.POST("/orders/:id/ship", middleware.RequireMerchantPermission(middleware.MerchantPermissionOrderShip), merchantOrderHandler.Ship)
			merchantProtected.GET("/categories", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogRead), merchantCatalogHandler.ListCategories)
			merchantProtected.POST("/categories", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.CreateCategory)
			merchantProtected.PUT("/categories/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.UpdateCategory)
			merchantProtected.DELETE("/categories/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.DeleteCategory)
			merchantProtected.GET("/products", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogRead), merchantCatalogHandler.ListProducts)
			merchantProtected.POST("/products", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.CreateProduct)
			merchantProtected.GET("/products/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogRead), merchantCatalogHandler.ProductDetail)
			merchantProtected.PUT("/products/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.UpdateProduct)
			merchantProtected.DELETE("/products/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.DeleteProduct)
			merchantProtected.POST("/products/:id/on-sale", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.OnSaleProduct)
			merchantProtected.POST("/products/:id/off-sale", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.OffSaleProduct)
			merchantProtected.POST("/products/:id/skus", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.CreateSKU)
			merchantProtected.PUT("/products/:id/skus/:sku_id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.UpdateSKU)
			merchantProtected.DELETE("/products/:id/skus/:sku_id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCatalogWrite), merchantCatalogHandler.DeleteSKU)
			merchantProtected.GET("/inventory-logs", middleware.RequireMerchantPermission(middleware.MerchantPermissionInventoryRead), merchantInventoryHandler.List)
			merchantProtected.GET("/inventory-alerts", middleware.RequireMerchantPermission(middleware.MerchantPermissionInventoryRead), merchantInventoryHandler.ListAlerts)
			merchantProtected.PUT("/inventory/skus/:sku_id/stock", middleware.RequireMerchantPermission(middleware.MerchantPermissionInventoryWrite), merchantInventoryHandler.AdjustStock)
			merchantProtected.GET("/dashboard/overview", middleware.RequireMerchantPermission(middleware.MerchantPermissionDashboardRead), merchantDashboardHandler.Overview)
			merchantProtected.GET("/dashboard/analytics", middleware.RequireMerchantPermission(middleware.MerchantPermissionDashboardRead), merchantDashboardHandler.Analytics)
			merchantProtected.GET("/customers/overview", middleware.RequireMerchantPermission(middleware.MerchantPermissionCustomerRead), merchantCustomerHandler.Overview)
			merchantProtected.GET("/customers", middleware.RequireMerchantPermission(middleware.MerchantPermissionCustomerRead), merchantCustomerHandler.List)
			merchantProtected.GET("/customers/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionCustomerRead), merchantCustomerHandler.Detail)
			merchantProtected.GET("/after-sales", middleware.RequireMerchantPermission(middleware.MerchantPermissionAfterSaleRead), afterSaleHandler.MerchantList)
			merchantProtected.POST("/after-sales/:id/approve", middleware.RequireMerchantPermission(middleware.MerchantPermissionAfterSaleWrite), afterSaleHandler.MerchantApprove)
			merchantProtected.POST("/after-sales/:id/refund/sync", middleware.RequireMerchantPermission(middleware.MerchantPermissionAfterSaleWrite), afterSaleHandler.MerchantSyncRefund)
			merchantProtected.POST("/after-sales/:id/reject", middleware.RequireMerchantPermission(middleware.MerchantPermissionAfterSaleWrite), afterSaleHandler.MerchantReject)
			merchantProtected.GET("/coupons", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingRead), couponHandler.MerchantList)
			merchantProtected.POST("/coupons", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingWrite), couponHandler.MerchantCreate)
			merchantProtected.PUT("/coupons/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingWrite), couponHandler.MerchantUpdate)
			merchantProtected.GET("/settlement-entries", middleware.RequireMerchantPermission(middleware.MerchantPermissionSettlementRead), merchantSettlementHandler.ListEntries)
			merchantProtected.GET("/settlements", middleware.RequireMerchantPermission(middleware.MerchantPermissionSettlementRead), merchantSettlementHandler.List)
			merchantProtected.GET("/settlements/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionSettlementRead), merchantSettlementHandler.Detail)
		}

		protected := api.Group("")
		protected.Use(middleware.Auth(jwtCfg))
		{
			protected.GET("/me", authHandler.Me)
			protected.PUT("/me", authHandler.UpdateProfile)
			protected.POST("/auth/logout", authHandler.Logout)

			protected.GET("/addresses", addressHandler.List)
			protected.POST("/addresses", addressHandler.Create)
			protected.PUT("/addresses/:id", addressHandler.Update)
			protected.DELETE("/addresses/:id", addressHandler.Delete)
			protected.PUT("/addresses/:id/default", addressHandler.SetDefault)

			protected.GET("/cart/items", cartHandler.List)
			protected.POST("/cart/items", cartHandler.Add)
			protected.PUT("/cart/items/:sku_id", cartHandler.Update)
			protected.DELETE("/cart/items/:sku_id", cartHandler.Delete)
			protected.DELETE("/cart/items", cartHandler.Clear)

			protected.POST("/orders/preview", orderHandler.Preview)
			protected.GET("/orders", orderHandler.List)
			protected.POST("/orders", orderHandler.Create)
			protected.GET("/orders/:id", orderHandler.Detail)
			protected.POST("/orders/:id/cancel", orderHandler.Cancel)
			if paymentCfg.MockEnabled && developmentFeaturesEnabled {
				protected.POST("/orders/:id/pay", orderHandler.Pay)
			}
			protected.POST("/orders/:id/confirm", orderHandler.Confirm)
			protected.GET("/orders/:id/logistics", orderHandler.Logistics)

			protected.POST("/trades/preview", tradeHandler.Preview)
			protected.POST("/trades", tradeHandler.Create)
			protected.GET("/trades", tradeHandler.List)
			protected.GET("/trades/:id", tradeHandler.Detail)
			protected.POST("/trades/:id/cancel", tradeHandler.Cancel)

			protected.POST("/payments", paymentHandler.Create)
			protected.GET("/payments/:payment_no", paymentHandler.Detail)
			if paymentCfg.MockEnabled && developmentFeaturesEnabled {
				protected.POST("/payments/:payment_no/mock-complete", paymentHandler.MockComplete)
			}
			protected.POST("/payments/:payment_no/sync", paymentHandler.Sync)

			protected.POST("/uploads", uploadHandler.Image)
			protected.GET("/after-sales", afterSaleHandler.List)
			protected.POST("/after-sales", afterSaleHandler.Create)
			protected.GET("/after-sales/:id", afterSaleHandler.Detail)
			protected.POST("/after-sales/:id/cancel", afterSaleHandler.Cancel)
			protected.GET("/coupons", couponHandler.Available)
			protected.POST("/coupons/:id/claim", couponHandler.Claim)
			protected.GET("/me/coupons", couponHandler.Mine)
			protected.GET("/favorites/products", favoriteHandler.List)
			protected.POST("/favorites/products", favoriteHandler.Add)
			protected.DELETE("/favorites/products/:product_id", favoriteHandler.Delete)
		}
	}

	return r, nil
}
