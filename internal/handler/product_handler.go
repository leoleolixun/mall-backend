package handler

import (
	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

	var categoryID int64
	categoryIDText := c.Query("category_id")
	if categoryIDText != "" {
		categoryID, err = strconv.ParseInt(categoryIDText, 10, 64)
		if err != nil || categoryID < 0 {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "category_id 参数不合法")
			return
		}
	}

	req := dto.ProductListRequest{
		Page:       page,
		PageSize:   pageSize,
		CategoryID: categoryID,
		Keyword:    c.Query("keyword"),
	}

	products, err := h.productService.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品失败")
		return
	}
	response.Success(c, products)
}

func (h *ProductHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商品ID不合法")
		return
	}

	product, err := h.productService.Detail(c.Request.Context(), id)
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
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品SKU失败")
		return
	}
	response.Success(c, skus)
}
