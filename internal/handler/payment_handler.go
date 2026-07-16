package handler

import (
	"net/http"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/observability"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService service.PaymentService
	metrics        *observability.Metrics
}

func NewPaymentHandler(paymentService service.PaymentService, metrics *observability.Metrics) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		metrics:        metrics,
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
	if result.TradeID != nil {
		middleware.SetAuditLogField(c, "trade_id", *result.TradeID)
		middleware.SetAuditLogField(c, "trade_no", result.TradeNo)
	}
	if result.OrderID != nil {
		middleware.SetAuditLogField(c, "order_id", *result.OrderID)
		middleware.SetAuditLogField(c, "order_no", result.OrderNo)
	}
	middleware.SetAuditLogField(c, "payment_no", result.PaymentNo)

	response.Success(c, result)
}

func (h *PaymentHandler) Detail(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	paymentNo := c.Param("payment_no")
	middleware.SetAuditLogField(c, "payment_no", paymentNo)
	result, err := h.paymentService.Detail(c.Request.Context(), userID, paymentNo)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *PaymentHandler) AlipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		if h.metrics != nil {
			h.metrics.RecordPaymentCallbackFailure()
		}
		c.String(http.StatusBadRequest, "fail")
		return
	}
	middleware.SetAuditLogField(c, "payment_no", c.Request.Form.Get("out_trade_no"))

	if err := h.paymentService.AlipayNotify(c.Request.Context(), c.Request.Form); err != nil {
		if h.metrics != nil {
			h.metrics.RecordPaymentCallbackFailure()
		}
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

func (h *PaymentHandler) MockComplete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	paymentNo := c.Param("payment_no")
	middleware.SetAuditLogField(c, "payment_no", paymentNo)
	result, err := h.paymentService.MockComplete(c.Request.Context(), userID, paymentNo)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

func (h *PaymentHandler) Sync(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	paymentNo := c.Param("payment_no")
	middleware.SetAuditLogField(c, "payment_no", paymentNo)
	result, err := h.paymentService.Sync(c.Request.Context(), userID, paymentNo)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
