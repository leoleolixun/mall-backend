package handler

import (
	"context"
	"go-mall/pkg/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	result := gin.H{
		"status": "ok",
		"mysql":  "ok",
		"redis":  "ok",
	}

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "mysql unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.redis.Ping(ctx).Err(); err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "redis unavailable")
		return
	}

	response.Success(c, result)

}
