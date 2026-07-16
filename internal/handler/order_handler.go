package handler

import (
	"fmt"
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
	tradeService   service.TradeService
	paymentService service.PaymentService
}

func NewOrderHandler(orderService service.OrderService, tradeService service.TradeService, paymentService service.PaymentService) *OrderHandler {
	return &OrderHandler{
		orderService:   orderService,
		tradeService:   tradeService,
		paymentService: paymentService,
	}
}

func parseOrderID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "订单 ID 不合法")
		return 0, false
	}
	middleware.SetAuditLogField(c, "order_id", id)

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

	merchantCoupons := make([]dto.MerchantCouponSelection, 0, 1)
	if req.UserCouponID > 0 {
		merchantCoupons = append(merchantCoupons, dto.MerchantCouponSelection{MerchantID: defaultLegacyMerchantID, UserCouponID: req.UserCouponID})
	}
	tradePreview, err := h.tradeService.Preview(c.Request.Context(), userID, dto.TradePreviewRequest{
		AddressID: req.AddressID, MerchantCoupons: merchantCoupons, Items: req.Items,
	})
	if err != nil {
		writeTradeError(c, err)
		return
	}
	if len(tradePreview.MerchantGroups) != 1 || tradePreview.MerchantGroups[0].MerchantID != defaultLegacyMerchantID {
		response.Error(c, http.StatusConflict, response.CodeConflict, "旧订单接口只支持默认商户，请使用交易接口结算")
		return
	}
	group := tradePreview.MerchantGroups[0]
	response.Success(c, &dto.OrderPreviewResponse{
		IdempotencyToken: tradePreview.IdempotencyToken,
		MerchantID:       group.MerchantID,
		MerchantName:     group.MerchantName,
		Address:          tradePreview.Address,
		Items:            group.Items,
		GoodsAmount:      group.GoodsAmount,
		FreightAmount:    group.FreightAmount,
		DiscountAmount:   group.DiscountAmount,
		PayableAmount:    group.PayableAmount,
		UserCouponID:     group.UserCouponID,
	})
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
	middleware.SetAuditLogField(c, "order_no", result.OrderNo)

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
	tradeID, childCount, linked, err := h.tradeService.ResolveOrderTrade(c.Request.Context(), userID, id)
	if err != nil {
		writeTradeError(c, err)
		return
	}
	if linked {
		if childCount > 1 {
			writeTradeError(c, fmt.Errorf("%w: 多商户交易必须通过 /trades/%d/cancel 整笔取消", service.ErrTradeConflict, tradeID))
			return
		}
		if _, err := h.tradeService.Cancel(c.Request.Context(), userID, tradeID); err != nil {
			writeTradeError(c, err)
			return
		}
		response.Success(c, nil)
		return
	}
	if err := h.paymentService.PrepareUserOrderForCancel(c.Request.Context(), userID, id); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	if err := h.orderService.Cancel(c.Request.Context(), userID, id); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.Success(c, nil)
}

func (h *OrderHandler) Logistics(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	id, ok := parseOrderID(c)
	if !ok {
		return
	}

	result, err := h.orderService.Logistics(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *OrderHandler) Confirm(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	id, ok := parseOrderID(c)
	if !ok {
		return
	}

	result, err := h.orderService.Confirm(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	middleware.SetAuditLogField(c, "order_no", result.OrderNo)
	response.Success(c, result)
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
	middleware.SetAuditLogField(c, "order_no", result.OrderNo)

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

	merchantCoupons := make([]dto.MerchantCouponSelection, 0, 1)
	if req.UserCouponID > 0 {
		merchantCoupons = append(merchantCoupons, dto.MerchantCouponSelection{MerchantID: defaultLegacyMerchantID, UserCouponID: req.UserCouponID})
	}
	trade, err := h.tradeService.Create(c.Request.Context(), userID, dto.CreateTradeRequest{
		AddressID: req.AddressID, MerchantCoupons: merchantCoupons, Remark: req.Remark,
		IdempotencyToken: req.IdempotencyToken, Items: req.Items,
	})
	if err != nil {
		writeTradeError(c, err)
		return
	}
	if len(trade.Orders) != 1 || trade.Orders[0].MerchantID != defaultLegacyMerchantID {
		writeTradeError(c, fmt.Errorf("%w: 旧订单接口只能创建默认商户单组交易", service.ErrTradeConflict))
		return
	}
	result := &trade.Orders[0]
	middleware.SetAuditLogField(c, "order_id", result.ID)
	middleware.SetAuditLogField(c, "order_no", result.OrderNo)

	response.Success(c, result)
}

const defaultLegacyMerchantID int64 = 1
