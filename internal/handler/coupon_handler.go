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

type CouponHandler struct{ service service.CouponService }

func NewCouponHandler(value service.CouponService) *CouponHandler {
	return &CouponHandler{service: value}
}
func parseCouponID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "优惠券 ID 不合法")
		return 0, false
	}
	return id, true
}
func (h *CouponHandler) Available(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	result, err := h.service.Available(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CouponHandler) Claim(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	id, ok := parseCouponID(c)
	if !ok {
		return
	}
	result, err := h.service.Claim(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CouponHandler) Mine(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}
	status, err := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
		return
	}
	result, err := h.service.Mine(c.Request.Context(), userID, status)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CouponHandler) MerchantList(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, "page 参数不合法")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, "page_size 参数不合法")
		return
	}
	status, err := strconv.Atoi(c.DefaultQuery("status", "-1"))
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, "status 参数不合法")
		return
	}
	result, err := h.service.MerchantList(c.Request.Context(), identity.MerchantID, dto.CouponListRequest{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CouponHandler) MerchantCreate(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, 401, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	var req dto.CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.MerchantCreate(c.Request.Context(), identity.MerchantID, req)
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CouponHandler) MerchantUpdate(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, 401, response.CodeUnauthorized, "商家后台未登录")
		return
	}
	id, ok := parseCouponID(c)
	if !ok {
		return
	}
	var req dto.CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.MerchantUpdate(c.Request.Context(), identity.MerchantID, id, req)
	if err != nil {
		response.Error(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
