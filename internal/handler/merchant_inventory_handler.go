package handler

import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantInventoryHandler struct {
	service service.MerchantInventoryService
}

func NewMerchantInventoryHandler(inventoryService service.MerchantInventoryService) *MerchantInventoryHandler {
	return &MerchantInventoryHandler{service: inventoryService}
}

func parseOptionalPositiveQueryID(c *gin.Context, name string) (int64, bool) {
	value := c.Query(name)
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, name+" 参数不合法")
		return 0, false
	}
	return id, true
}

func (h *MerchantInventoryHandler) List(c *gin.Context) {
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
	productID, ok := parseOptionalPositiveQueryID(c, "product_id")
	if !ok {
		return
	}
	skuID, ok := parseOptionalPositiveQueryID(c, "sku_id")
	if !ok {
		return
	}

	result, err := h.service.List(c.Request.Context(), merchantID, dto.MerchantInventoryLogListRequest{
		Page: page, PageSize: pageSize, ProductID: productID, SKUID: skuID, ChangeType: c.Query("change_type"),
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantInventoryHandler) ListAlerts(c *gin.Context) {
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
	productID, ok := parseOptionalPositiveQueryID(c, "product_id")
	if !ok {
		return
	}
	skuID, ok := parseOptionalPositiveQueryID(c, "sku_id")
	if !ok {
		return
	}

	result, err := h.service.ListAlerts(c.Request.Context(), merchantID, dto.MerchantInventoryAlertListRequest{
		Page: page, PageSize: pageSize, ProductID: productID, SKUID: skuID, Keyword: c.Query("keyword"),
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantInventoryHandler) AdjustStock(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	skuID, ok := parsePositivePathID(c, "sku_id", "SKU ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantStockAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.AdjustStock(c.Request.Context(), identity.MerchantID, identity.AccountID, skuID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
