package router

import (
	"go-mall/internal/handler"

	"github.com/gin-gonic/gin"
)

func NewRouter(healthHandler *handler.HealthHandler) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.GET("/health", healthHandler.Health)

	return r
}
