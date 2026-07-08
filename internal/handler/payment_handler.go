package handler

import (
	"net/http"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService service.PaymentService
}

func NewPaymentHandler(paymentService service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

func (h *PaymentHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.paymentService.Create(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *PaymentHandler) Detail(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	result, err := h.paymentService.Detail(c.Request.Context(), userID, c.Param("payment_no"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *PaymentHandler) MockComplete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	result, err := h.paymentService.MockComplete(c.Request.Context(), userID, c.Param("payment_no"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}
