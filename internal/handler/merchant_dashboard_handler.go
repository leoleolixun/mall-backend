package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantDashboardHandler struct {
	service service.MerchantDashboardService
}

func (h *MerchantDashboardHandler) Analytics(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	days, err := strconv.Atoi(c.DefaultQuery("days", "7"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "days 参数不合法")
		return
	}
	topLimit, err := strconv.Atoi(c.DefaultQuery("top_limit", "10"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "top_limit 参数不合法")
		return
	}
	analytics, err := h.service.Analytics(c.Request.Context(), merchantID, dto.MerchantDashboardAnalyticsRequest{
		Days: days, TopLimit: topLimit,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, analytics)
}

func NewMerchantDashboardHandler(dashboardService service.MerchantDashboardService) *MerchantDashboardHandler {
	return &MerchantDashboardHandler{service: dashboardService}
}

func (h *MerchantDashboardHandler) Overview(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	overview, err := h.service.Overview(c.Request.Context(), merchantID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, overview)
}
