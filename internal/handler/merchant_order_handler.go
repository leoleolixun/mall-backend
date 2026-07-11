package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantOrderHandler struct {
	service service.MerchantOrderService
}

func NewMerchantOrderHandler(merchantOrderService service.MerchantOrderService) *MerchantOrderHandler {
	return &MerchantOrderHandler{service: merchantOrderService}
}

func (h *MerchantOrderHandler) List(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page 参数不合法")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page_size 参数不合法")
		return
	}
	status, err := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
		return
	}

	result, err := h.service.List(c.Request.Context(), identity.MerchantID, dto.MerchantOrderListRequest{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantOrderHandler) Detail(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "订单 ID 不合法")
		return
	}

	result, err := h.service.Detail(c.Request.Context(), identity.MerchantID, orderID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantOrderHandler) Ship(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "订单 ID 不合法")
		return
	}
	var req dto.ShipOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.service.Ship(c.Request.Context(), identity.MerchantID, orderID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
