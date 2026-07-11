package handler

import (
	"net/http"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantAuthHandler struct {
	service service.MerchantAuthService
}

func NewMerchantAuthHandler(merchantAuthService service.MerchantAuthService) *MerchantAuthHandler {
	return &MerchantAuthHandler{service: merchantAuthService}
}

func (h *MerchantAuthHandler) Login(c *gin.Context) {
	var req dto.MerchantLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantAuthHandler) Refresh(c *gin.Context) {
	var req dto.MerchantRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantAuthHandler) Logout(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}

	var req dto.MerchantRefreshRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.service.Logout(c.Request.Context(), identity.AccountID, req.RefreshToken); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MerchantAuthHandler) Me(c *gin.Context) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return
	}

	result, err := h.service.Me(c.Request.Context(), identity.AccountID, identity.MerchantID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	response.Success(c, result)
}
