package router

import (
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
	authHandler *handler.AuthHandler,
	addressHandler *handler.AddressHandler,
	cartHandler *handler.CartHandler,
	orderHandler *handler.OrderHandler,
	paymentHandler *handler.PaymentHandler,
	uploadHandler *handler.UploadHandler,
	afterSaleHandler *handler.AfterSaleHandler,
	couponHandler *handler.CouponHandler,
	merchantAuthHandler *handler.MerchantAuthHandler,
	merchantAccountHandler *handler.MerchantAccountHandler,
	merchantOrderHandler *handler.MerchantOrderHandler,
	merchantCatalogHandler *handler.MerchantCatalogHandler,
	merchantInventoryHandler *handler.MerchantInventoryHandler,
	merchantDashboardHandler *handler.MerchantDashboardHandler,
	merchantCustomerHandler *handler.MerchantCustomerHandler,
	merchantAccountLoader middleware.MerchantAccountLoader,
	redisClient *redis.Client,
	jwtCfg config.JWTConfig,
) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.GET("/health", healthHandler.Health)
	r.StaticFile("/docs/openapi.yaml", "docs/openapi.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/openapi.yaml")))

	api := r.Group("/api/v1")
	{
		api.GET("/categories", categoryHandler.List)
		api.GET("/products", productHandler.List)
		api.GET("/products/:id", productHandler.Detail)
		api.GET("/products/:id/skus", productHandler.SKUs)
		api.POST("/payments/alipay/notify", paymentHandler.AlipayNotify)

		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login/password", authHandler.PasswordLogin)
			auth.POST("/login/wechat", authHandler.WechatMiniProgramLogin)
			auth.POST("/refresh", authHandler.Refresh)
		}

		merchantAuth := api.Group("/merchant/auth")
		{
			merchantAuth.POST("/login", merchantAuthHandler.Login)
			merchantAuth.POST("/refresh", merchantAuthHandler.Refresh)
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
			merchantProtected.POST("/after-sales/:id/reject", middleware.RequireMerchantPermission(middleware.MerchantPermissionAfterSaleWrite), afterSaleHandler.MerchantReject)
			merchantProtected.GET("/coupons", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingRead), couponHandler.MerchantList)
			merchantProtected.POST("/coupons", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingWrite), couponHandler.MerchantCreate)
			merchantProtected.PUT("/coupons/:id", middleware.RequireMerchantPermission(middleware.MerchantPermissionMarketingWrite), couponHandler.MerchantUpdate)
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
			protected.POST("/orders/:id/pay", orderHandler.Pay)
			protected.POST("/orders/:id/confirm", orderHandler.Confirm)
			protected.GET("/orders/:id/logistics", orderHandler.Logistics)

			protected.POST("/payments", paymentHandler.Create)
			protected.GET("/payments/:payment_no", paymentHandler.Detail)
			protected.POST("/payments/:payment_no/mock-complete", paymentHandler.MockComplete)
			protected.POST("/payments/:payment_no/sync", paymentHandler.Sync)

			protected.POST("/uploads", uploadHandler.Image)
			protected.GET("/after-sales", afterSaleHandler.List)
			protected.POST("/after-sales", afterSaleHandler.Create)
			protected.GET("/after-sales/:id", afterSaleHandler.Detail)
			protected.POST("/after-sales/:id/cancel", afterSaleHandler.Cancel)
			protected.GET("/coupons", couponHandler.Available)
			protected.POST("/coupons/:id/claim", couponHandler.Claim)
			protected.GET("/me/coupons", couponHandler.Mine)
		}
	}

	return r
}
