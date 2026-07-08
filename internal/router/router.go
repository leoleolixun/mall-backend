package router

import (
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/middleware"

	"github.com/gin-gonic/gin"
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

		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login/password", authHandler.PasswordLogin)
			auth.POST("/login/wechat", authHandler.WechatMiniProgramLogin)
			auth.POST("/refresh", authHandler.Refresh)
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
		}
	}

	return r
}
