package router

import (
	"go-mall/internal/handler"

	"github.com/gin-gonic/gin"
)

func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
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
	}

	return r
}
