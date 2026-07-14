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

type AfterSaleHandler struct{ service service.AfterSaleService }

func NewAfterSaleHandler(value service.AfterSaleService) *AfterSaleHandler {
	return &AfterSaleHandler{service: value}
}

func parseAfterSaleID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "售后 ID 不合法")
		return 0, false
	}
	return id, true
}

func afterSaleListRequest(c *gin.Context) (dto.AfterSaleListRequest, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page 参数不合法")
		return dto.AfterSaleListRequest{}, false
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page_size 参数不合法")
		return dto.AfterSaleListRequest{}, false
	}
	status, err := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
		return dto.AfterSaleListRequest{}, false
	}
	return dto.AfterSaleListRequest{Page: page, PageSize: pageSize, Status: status}, true
}

func (h *AfterSaleHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	var req dto.CreateAfterSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AfterSaleHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	req, ok := afterSaleListRequest(c)
	if !ok {
		return
	}
	result, err := h.service.List(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AfterSaleHandler) Detail(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	id, ok := parseAfterSaleID(c)
	if !ok {
		return
	}
	result, err := h.service.Detail(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AfterSaleHandler) Cancel(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	id, ok := parseAfterSaleID(c)
	if !ok {
		return
	}
	if err := h.service.Cancel(c.Request.Context(), userID, id); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AfterSaleHandler) MerchantList(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	req, ok := afterSaleListRequest(c)
	if !ok {
		return
	}
	result, err := h.service.MerchantList(c.Request.Context(), identity.MerchantID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AfterSaleHandler) MerchantApprove(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	id, ok := parseAfterSaleID(c)
	if !ok {
		return
	}
	result, err := h.service.MerchantApprove(c.Request.Context(), identity.MerchantID, identity.AccountID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AfterSaleHandler) MerchantReject(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	id, ok := parseAfterSaleID(c)
	if !ok {
		return
	}
	var req dto.RejectAfterSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.MerchantReject(c.Request.Context(), identity.MerchantID, identity.AccountID, id, req.Reason)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
