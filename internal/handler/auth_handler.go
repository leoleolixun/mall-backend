package handler

import (
	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) PasswordLogin(c *gin.Context) {
	var req dto.PasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.PasswordLogin(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) WechatMiniProgramLogin(c *gin.Context) {
	var req dto.WechatMiniProgramLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.WechatMiniProgramLogin(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	user, err := h.authService.Me(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询用户失败")
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if _, ok := middleware.CurrentUserID(c); !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	var req dto.RefreshTokenRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "退出登录失败")
		return
	}
	response.Success(c, nil)
}
