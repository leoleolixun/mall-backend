package handler

import (
	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orderService   service.OrderService
	paymentService service.PaymentService
}

func NewOrderHandler(orderService service.OrderService, paymentService service.PaymentService) *OrderHandler {
	return &OrderHandler{
		orderService:   orderService,
		paymentService: paymentService,
	}
}

func parseOrderID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "订单 ID 不合法")
		return 0, false
	}

	return id, true
}

func (h *OrderHandler) Preview(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	var req dto.OrderPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.orderService.Preview(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *OrderHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
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

	status := 0
	if value := c.Query("status"); value != "" {
		status, err = strconv.Atoi(value)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
			return
		}
	}

	result, err := h.orderService.List(c.Request.Context(), userID, dto.OrderListRequest{
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

func (h *OrderHandler) Detail(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	id, ok := parseOrderID(c)
	if !ok {
		return
	}

	result, err := h.orderService.Detail(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	id, ok := parseOrderID(c)
	if !ok {
		return
	}

	if err := h.orderService.Cancel(c.Request.Context(), userID, id); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, nil)
}

func (h *OrderHandler) Pay(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	id, ok := parseOrderID(c)
	if !ok {
		return
	}

	if _, err := h.paymentService.MockPayOrder(c.Request.Context(), userID, id); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	result, err := h.orderService.Detail(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *OrderHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.orderService.Create(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}
