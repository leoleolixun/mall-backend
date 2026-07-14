package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantAccountHandler struct {
	service service.MerchantAccountService
}

func NewMerchantAccountHandler(accountService service.MerchantAccountService) *MerchantAccountHandler {
	return &MerchantAccountHandler{service: accountService}
}

func (h *MerchantAccountHandler) List(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
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
	var status *int
	if value := c.Query("status"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
			return
		}
		status = &parsed
	}
	result, err := h.service.List(c.Request.Context(), identity.MerchantID, dto.MerchantAccountListRequest{
		Page: page, PageSize: pageSize, Role: c.Query("role"), Status: status, Keyword: c.Query("keyword"),
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantAccountHandler) Create(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	var req dto.MerchantAccountCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.Create(c.Request.Context(), identity.Role, identity.MerchantID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantAccountHandler) Update(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	accountID, ok := parsePositivePathID(c, "id", "员工账号 ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantAccountUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.Update(c.Request.Context(), identity.AccountID, identity.Role, identity.MerchantID, accountID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantAccountHandler) ResetPassword(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	accountID, ok := parsePositivePathID(c, "id", "员工账号 ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantAccountPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), identity.AccountID, identity.Role, identity.MerchantID, accountID, req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MerchantAccountHandler) Roles(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	result, err := h.service.Roles(c.Request.Context(), identity.MerchantID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
