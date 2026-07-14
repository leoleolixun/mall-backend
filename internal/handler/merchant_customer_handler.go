package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantCustomerHandler struct {
	service service.MerchantCustomerService
}

func NewMerchantCustomerHandler(customerService service.MerchantCustomerService) *MerchantCustomerHandler {
	return &MerchantCustomerHandler{service: customerService}
}

func (h *MerchantCustomerHandler) Overview(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	result, err := h.service.Overview(c.Request.Context(), merchantID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCustomerHandler) List(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
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
	repeatOnly, err := strconv.ParseBool(c.DefaultQuery("repeat_only", "false"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "repeat_only 参数不合法")
		return
	}
	result, err := h.service.List(c.Request.Context(), merchantID, dto.MerchantCustomerListRequest{
		Page: page, PageSize: pageSize, Keyword: c.Query("keyword"), RepeatOnly: repeatOnly,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCustomerHandler) Detail(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	userID, ok := parsePositivePathID(c, "id", "顾客 ID 不合法")
	if !ok {
		return
	}
	result, err := h.service.Detail(c.Request.Context(), merchantID, userID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
