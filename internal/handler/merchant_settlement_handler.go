package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantSettlementHandler struct {
	service service.SettlementService
}

func NewMerchantSettlementHandler(settlementService service.SettlementService) *MerchantSettlementHandler {
	return &MerchantSettlementHandler{service: settlementService}
}

func (h *MerchantSettlementHandler) ListEntries(c *gin.Context) {
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
	result, err := h.service.ListEntries(c.Request.Context(), merchantID, dto.SettlementEntryListRequest{
		Page: page, PageSize: pageSize, EntryType: c.Query("entry_type"),
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantSettlementHandler) List(c *gin.Context) {
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
	status, err := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
		return
	}
	result, err := h.service.List(c.Request.Context(), merchantID, dto.MerchantSettlementListRequest{
		Page: page, PageSize: pageSize, Status: status,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantSettlementHandler) Detail(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	id, ok := parsePositivePathID(c, "id", "结算单 ID 不合法")
	if !ok {
		return
	}
	result, err := h.service.Detail(c.Request.Context(), merchantID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		return
	}
	response.Success(c, result)
}
