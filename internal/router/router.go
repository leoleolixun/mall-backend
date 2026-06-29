package router

import (
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/middleware"

	"github.com/gin-gonic/gin"
)

func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
	authHandler *handler.AuthHandler,
	jwtCfg config.JWTConfig,
) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.GET("/health", healthHandler.Health)

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
			protected.POST("/auth/logout", authHandler.Logout)
		}
	}

	return r
}
