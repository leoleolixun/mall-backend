package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type TradeHandler struct {
	service service.TradeService
}

func NewTradeHandler(tradeService service.TradeService) *TradeHandler {
	return &TradeHandler{service: tradeService}
}

func (h *TradeHandler) Preview(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	var req dto.TradePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.Preview(c.Request.Context(), userID, req)
	if err != nil {
		writeTradeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TradeHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	var req dto.CreateTradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		writeTradeError(c, err)
		return
	}
	middleware.SetAuditLogField(c, "trade_id", result.ID)
	middleware.SetAuditLogField(c, "trade_no", result.TradeNo)
	response.Success(c, result)
}

func (h *TradeHandler) List(c *gin.Context) {
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
	result, err := h.service.List(c.Request.Context(), userID, dto.TradeListRequest{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		writeTradeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TradeHandler) Detail(c *gin.Context) {
	userID, tradeID, ok := tradeIdentity(c)
	if !ok {
		return
	}
	result, err := h.service.Detail(c.Request.Context(), userID, tradeID)
	if err != nil {
		writeTradeError(c, err)
		return
	}
	middleware.SetAuditLogField(c, "trade_no", result.TradeNo)
	response.Success(c, result)
}

func (h *TradeHandler) Cancel(c *gin.Context) {
	userID, tradeID, ok := tradeIdentity(c)
	if !ok {
		return
	}
	result, err := h.service.Cancel(c.Request.Context(), userID, tradeID)
	if err != nil {
		writeTradeError(c, err)
		return
	}
	middleware.SetAuditLogField(c, "trade_no", result.TradeNo)
	response.Success(c, result)
}

func tradeIdentity(c *gin.Context) (int64, int64, bool) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return 0, 0, false
	}
	tradeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || tradeID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "交易 ID 不合法")
		return 0, 0, false
	}
	middleware.SetAuditLogField(c, "trade_id", tradeID)
	return userID, tradeID, true
}

func writeTradeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTradeNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, service.ErrTradeConflict), errors.Is(err, service.ErrTradePreviewRequired):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	default:
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	}
}
