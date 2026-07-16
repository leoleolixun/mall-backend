package handler

import (
	"errors"
	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProductHandler struct {
	productService service.ProductService
}

func NewProductHandler(productService service.ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) List(c *gin.Context) {
	req, ok := parseProductListRequest(c)
	if !ok {
		return
	}

	products, err := h.productService.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品失败")
		return
	}
	response.Success(c, products)
}

func parseProductListRequest(c *gin.Context) (dto.ProductListRequest, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page 参数不合法")
		return dto.ProductListRequest{}, false
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page_size 参数不合法")
		return dto.ProductListRequest{}, false
	}

	var merchantID int64
	merchantIDText := c.Query("merchant_id")
	if merchantIDText != "" {
		merchantID, err = strconv.ParseInt(merchantIDText, 10, 64)
		if err != nil || merchantID <= 0 {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "merchant_id 参数不合法")
			return dto.ProductListRequest{}, false
		}
	}

	var categoryID int64
	categoryIDText := c.Query("category_id")
	if categoryIDText != "" {
		categoryID, err = strconv.ParseInt(categoryIDText, 10, 64)
		if err != nil || categoryID < 0 {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "category_id 参数不合法")
			return dto.ProductListRequest{}, false
		}
	}

	return dto.ProductListRequest{
		Page:       page,
		PageSize:   pageSize,
		MerchantID: merchantID,
		CategoryID: categoryID,
		Keyword:    c.Query("keyword"),
	}, true
}

func (h *ProductHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商品ID不合法")
		return
	}

	product, err := h.productService.Detail(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "商品不存在或已下架")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品详情失败")
		return
	}
	response.Success(c, product)
}

func (h *ProductHandler) SKUs(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商品ID不合法")
		return
	}

	skus, err := h.productService.SKUs(c.Request.Context(), productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "商品不存在或已下架")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品SKU失败")
		return
	}
	response.Success(c, skus)
}
