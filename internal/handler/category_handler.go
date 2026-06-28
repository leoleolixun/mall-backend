package handler

import (
	"go-mall/internal/service"
	"go-mall/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CategoryHandler 处理商品分类相关的请求
type CategoryHandler struct {
	categoryService service.CategoryService
}

func NewCategoryHandler(categoryService service.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

// List 获取商品分类列表
func (h *CategoryHandler) List(c *gin.Context) {
	categories, err := h.categoryService.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品分类失败")
		return
	}
	response.Success(c, categories)
}
