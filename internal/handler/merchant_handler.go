package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type MerchantHandler struct {
	merchantService service.MerchantService
	categoryService service.CategoryService
	productService  service.ProductService
}

func NewMerchantHandler(merchantService service.MerchantService, categoryService service.CategoryService, productService service.ProductService) *MerchantHandler {
	return &MerchantHandler{merchantService: merchantService, categoryService: categoryService, productService: productService}
}

func (h *MerchantHandler) List(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page 参数不合法")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "page_size 参数不合法")
		return
	}
	result, err := h.merchantService.List(c.Request.Context(), dto.PublicMerchantListRequest{Page: page, PageSize: pageSize})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商户失败")
		return
	}
	response.Success(c, result)
}

func (h *MerchantHandler) Detail(c *gin.Context) {
	merchantID, ok := parseMerchantID(c)
	if !ok {
		return
	}
	result, err := h.merchantService.Detail(c.Request.Context(), merchantID)
	if errors.Is(err, service.ErrPublicMerchantNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商户失败")
		return
	}
	response.Success(c, result)
}

func (h *MerchantHandler) Categories(c *gin.Context) {
	merchantID, ok := parseMerchantID(c)
	if !ok {
		return
	}
	result, err := h.categoryService.ListByMerchant(c.Request.Context(), merchantID)
	if errors.Is(err, service.ErrPublicMerchantNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品分类失败")
		return
	}
	response.Success(c, result)
}

func (h *MerchantHandler) Products(c *gin.Context) {
	merchantID, ok := parseMerchantID(c)
	if !ok {
		return
	}
	if _, err := h.merchantService.Detail(c.Request.Context(), merchantID); err != nil {
		if errors.Is(err, service.ErrPublicMerchantNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商户失败")
		return
	}
	req, ok := parseProductListRequest(c)
	if !ok {
		return
	}
	req.MerchantID = merchantID
	result, err := h.productService.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品失败")
		return
	}
	response.Success(c, result)
}

func parseMerchantID(c *gin.Context) (int64, bool) {
	merchantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || merchantID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商户 ID 不合法")
		return 0, false
	}
	return merchantID, true
}
