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

type MerchantCatalogHandler struct {
	service service.MerchantCatalogService
}

func NewMerchantCatalogHandler(catalogService service.MerchantCatalogService) *MerchantCatalogHandler {
	return &MerchantCatalogHandler{service: catalogService}
}

func merchantIdentityFromContext(c *gin.Context) (middleware.MerchantIdentity, bool) {
	identity, ok := middleware.CurrentMerchant(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "商家后台未登录")
		return middleware.MerchantIdentity{}, false
	}
	return identity, true
}

func merchantIDFromContext(c *gin.Context) (int64, bool) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return 0, false
	}
	return identity.MerchantID, true
}

func parsePositivePathID(c *gin.Context, name string, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, message)
		return 0, false
	}
	return id, true
}

func (h *MerchantCatalogHandler) ListCategories(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	result, err := h.service.ListCategories(c.Request.Context(), merchantID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) CreateCategory(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	var req dto.MerchantCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.CreateCategory(c.Request.Context(), merchantID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) UpdateCategory(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	categoryID, ok := parsePositivePathID(c, "id", "分类 ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.UpdateCategory(c.Request.Context(), merchantID, categoryID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) DeleteCategory(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	categoryID, ok := parsePositivePathID(c, "id", "分类 ID 不合法")
	if !ok {
		return
	}
	if err := h.service.DeleteCategory(c.Request.Context(), merchantID, categoryID); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MerchantCatalogHandler) ListProducts(c *gin.Context) {
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
	var status *int
	if value := c.Query("status"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "status 参数不合法")
			return
		}
		status = &parsed
	}
	result, err := h.service.ListProducts(c.Request.Context(), merchantID, dto.MerchantProductListRequest{
		Page: page, PageSize: pageSize, Status: status, Keyword: c.Query("keyword"),
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) ProductDetail(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	result, err := h.service.ProductDetail(c.Request.Context(), merchantID, productID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) CreateProduct(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	var req dto.MerchantProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.CreateProduct(c.Request.Context(), merchantID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) UpdateProduct(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.UpdateProduct(c.Request.Context(), merchantID, productID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) DeleteProduct(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	if err := h.service.DeleteProduct(c.Request.Context(), merchantID, productID); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MerchantCatalogHandler) OnSaleProduct(c *gin.Context) {
	h.updateProductSaleStatus(c, true)
}

func (h *MerchantCatalogHandler) OffSaleProduct(c *gin.Context) {
	h.updateProductSaleStatus(c, false)
}

func (h *MerchantCatalogHandler) updateProductSaleStatus(c *gin.Context, onSale bool) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	var err error
	if onSale {
		err = h.service.OnSaleProduct(c.Request.Context(), merchantID, productID)
	} else {
		err = h.service.OffSaleProduct(c.Request.Context(), merchantID, productID)
	}
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MerchantCatalogHandler) CreateSKU(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.CreateSKU(c.Request.Context(), identity.MerchantID, identity.AccountID, productID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) UpdateSKU(c *gin.Context) {
	identity, ok := merchantIdentityFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	skuID, ok := parsePositivePathID(c, "sku_id", "SKU ID 不合法")
	if !ok {
		return
	}
	var req dto.MerchantSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不合法")
		return
	}
	result, err := h.service.UpdateSKU(c.Request.Context(), identity.MerchantID, identity.AccountID, productID, skuID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *MerchantCatalogHandler) DeleteSKU(c *gin.Context) {
	merchantID, ok := merchantIDFromContext(c)
	if !ok {
		return
	}
	productID, ok := parsePositivePathID(c, "id", "商品 ID 不合法")
	if !ok {
		return
	}
	skuID, ok := parsePositivePathID(c, "sku_id", "SKU ID 不合法")
	if !ok {
		return
	}
	if err := h.service.DeleteSKU(c.Request.Context(), merchantID, productID, skuID); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}
