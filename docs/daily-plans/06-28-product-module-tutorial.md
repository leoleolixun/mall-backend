# 2026-06-28 Product 模块完成教程

## 目标

这份教程只讲怎么把 Product 模块从当前状态补完整。

完成后需要跑通：

```http
GET /api/v1/categories
GET /api/v1/products
GET /api/v1/products/:id
GET /api/v1/products/:id/skus
```

当前你已经有：

```text
model
repository
dto
migrate
seed
部分 handler
空的 product service
```

接下来按顺序完成：

```text
CategoryService 实现
ProductService 实现
ProductHandler 补全
Router 接入商品路由
main.go 组装依赖
curl 验收
```

## 第 1 步：补 CategoryService

打开：

```text
internal/service/category_service.go
```

现在它只有接口。你需要补完整实现。

目标结构：

```go
package service

import (
	"context"

	"go-mall/internal/dto"
	"go-mall/internal/repository"
)

const defaultMerchantID int64 = 1

type CategoryService interface {
	List(ctx context.Context) ([]dto.CategoryResponse, error)
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository) CategoryService {
	return &categoryService{
		categoryRepo: categoryRepo,
	}
}

func (s *categoryService) List(ctx context.Context) ([]dto.CategoryResponse, error) {
	categories, err := s.categoryRepo.ListEnabled(ctx, defaultMerchantID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.CategoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, dto.CategoryResponse{
			ID:   category.ID,
			Name: category.Name,
			Sort: category.Sort,
		})
	}

	return result, nil
}
```

说明：

- `defaultMerchantID = 1` 是第一版单商户默认值
- Service 调用 Repository
- Service 把数据库 model 转成响应 DTO
- Service 不 import `gin`
- Service 不返回 `response.Success`

## 第 2 步：补 ProductService 基础结构

打开：

```text
internal/service/product_service.go
```

你现在已经有接口和空方法。先确认 import 至少包含：

```go
import (
	"context"

	"go-mall/internal/dto"
	"go-mall/internal/repository"

	"github.com/redis/go-redis/v9"
)
```

基础结构应该是：

```go
type ProductService interface {
	List(ctx context.Context, req dto.ProductListRequest) (*dto.PageResponse[dto.ProductListItem], error)
	Detail(ctx context.Context, id int64) (*dto.ProductDetailResponse, error)
	SKUs(ctx context.Context, productID int64) ([]dto.SKUResponse, error)
}

type productService struct {
	productRepo repository.ProductRepository
	redis       *redis.Client
}

func NewProductService(productRepo repository.ProductRepository, redis *redis.Client) ProductService {
	return &productService{
		productRepo: productRepo,
		redis:       redis,
	}
}
```

如果 `defaultMerchantID` 已经在 `category_service.go` 定义了，同一个 `service` package 内可以直接复用，不要在 `product_service.go` 重复定义。

## 第 3 步：实现 ProductService.List

替换空实现：

```go
func (s *productService) List(ctx context.Context, req dto.ProductListRequest) (*dto.PageResponse[dto.ProductListItem], error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 50 {
		req.PageSize = 50
	}

	offset := (req.Page - 1) * req.PageSize

	products, total, err := s.productRepo.ListOnSale(
		ctx,
		defaultMerchantID,
		req.CategoryID,
		req.Keyword,
		offset,
		req.PageSize,
	)
	if err != nil {
		return nil, err
	}

	productIDs := make([]int64, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}

	minPrices, err := s.productRepo.FindMinPrices(ctx, defaultMerchantID, productIDs)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ProductListItem, 0, len(products))
	for _, product := range products {
		items = append(items, dto.ProductListItem{
			ID:         product.ID,
			MerchantID: product.MerchantID,
			CategoryID: product.CategoryID,
			Name:       product.Name,
			Cover:      product.Cover,
			MinPrice:   minPrices[product.ID],
		})
	}

	return &dto.PageResponse[dto.ProductListItem]{
		List:     items,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}
```

这段逻辑做了：

```text
修正分页参数
计算 offset
查商品列表
查每个商品最低 SKU 价
组装前端响应
```

## 第 4 步：实现 ProductService.SKUs

替换空实现：

```go
func (s *productService) SKUs(ctx context.Context, productID int64) ([]dto.SKUResponse, error) {
	skus, err := s.productRepo.ListEnabledSKUs(ctx, defaultMerchantID, productID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.SKUResponse, 0, len(skus))
	for _, sku := range skus {
		result = append(result, dto.SKUResponse{
			ID:    sku.ID,
			Name:  sku.Name,
			Image: sku.Image,
			Price: sku.Price,
			Stock: sku.Stock,
		})
	}

	return result, nil
}
```

这里只查启用 SKU，不做库存扣减。

## 第 5 步：实现 ProductService.Detail

先实现 MySQL 版，不急着上 Redis 缓存。

替换空实现：

```go
func (s *productService) Detail(ctx context.Context, id int64) (*dto.ProductDetailResponse, error) {
	product, err := s.productRepo.FindOnSaleByID(ctx, defaultMerchantID, id)
	if err != nil {
		return nil, err
	}

	skus, err := s.SKUs(ctx, product.ID)
	if err != nil {
		return nil, err
	}

	return &dto.ProductDetailResponse{
		ID:          product.ID,
		MerchantID:  product.MerchantID,
		CategoryID:  product.CategoryID,
		Name:        product.Name,
		Cover:       product.Cover,
		Description: product.Description,
		SKUs:        skus,
	}, nil
}
```

等商品接口全跑通后，再加 Redis 缓存。不要一边写缓存一边排查基础接口问题。

## 第 6 步：补 ProductHandler imports

打开：

```text
internal/handler/product_handler.go
```

需要 import：

```go
import (
	"net/http"
	"strconv"

	"go-mall/internal/dto"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)
```

## 第 7 步：实现 ProductHandler.List

替换现在的 `List`：

```go
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
```

Handler 负责解析 HTTP 参数，分页修正放在 Service。

## 第 8 步：实现 ProductHandler.Detail

新增：

```go
func (h *ProductHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商品 ID 不合法")
		return
	}

	product, err := h.productService.Detail(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品详情失败")
		return
	}

	response.Success(c, product)
}
```

今天先不区分 404 和 500，后面再完善业务错误类型。

## 第 9 步：实现 ProductHandler.SKUs

新增：

```go
func (h *ProductHandler) SKUs(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "商品 ID 不合法")
		return
	}

	skus, err := h.productService.SKUs(c.Request.Context(), productID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询商品 SKU 失败")
		return
	}

	response.Success(c, skus)
}
```

## 第 10 步：更新 Router

打开：

```text
internal/router/router.go
```

把函数签名改成：

```go
func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
) *gin.Engine {
```

完整路由结构：

```go
func NewRouter(
	healthHandler *handler.HealthHandler,
	categoryHandler *handler.CategoryHandler,
	productHandler *handler.ProductHandler,
) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/health", healthHandler.Health)

	api := r.Group("/api/v1")
	{
		api.GET("/categories", categoryHandler.List)
		api.GET("/products", productHandler.List)
		api.GET("/products/:id", productHandler.Detail)
		api.GET("/products/:id/skus", productHandler.SKUs)
	}

	return r
}
```

## 第 11 步：更新 main.go 组装依赖

打开：

```text
cmd/server/main.go
```

补 import：

```go
import (
	"go-mall/internal/repository"
	"go-mall/internal/service"
)
```

在 Redis 初始化成功后，创建 handler 前，增加：

```go
categoryRepo := repository.NewCategoryRepository(db)
productRepo := repository.NewProductRepository(db)

categoryService := service.NewCategoryService(categoryRepo)
productService := service.NewProductService(productRepo, rdb)
```

然后 handler 改成：

```go
healthHandler := handler.NewHealthHandler(db, rdb)
categoryHandler := handler.NewCategoryHandler(categoryService)
productHandler := handler.NewProductHandler(productService)
```

router 改成：

```go
r := router.NewRouter(healthHandler, categoryHandler, productHandler)
```

最终依赖方向是：

```text
db/rdb
-> repository
-> service
-> handler
-> router
```

## 第 12 步：格式化和编译

执行：

```bash
gofmt -w internal/service/category_service.go \
  internal/service/product_service.go \
  internal/handler/product_handler.go \
  internal/router/router.go \
  cmd/server/main.go
```

然后：

```bash
go test ./...
```

如果这里不通过，先不要启动服务，按错误提示修。

## 第 13 步：启动服务

执行：

```bash
go run ./cmd/server
```

如果 seed 正常，启动时会自动建表并插入默认数据。

如果商品列表为空，检查：

- 数据库是否已有 `merchants.id = 1`
- `SeedDefaultData` 是否因为 merchant 已存在而跳过
- `products.status` 是否为 `1`
- `product_skus.status` 是否为 `1`
- 当前连接的数据库是否正确

## 第 14 步：curl 验收

分类：

```bash
curl "http://127.0.0.1:8080/api/v1/categories"
```

商品列表：

```bash
curl "http://127.0.0.1:8080/api/v1/products?page=1&page_size=10"
```

按分类查询：

```bash
curl "http://127.0.0.1:8080/api/v1/products?page=1&page_size=10&category_id=1"
```

关键词查询：

```bash
curl "http://127.0.0.1:8080/api/v1/products?keyword=键盘"
```

商品详情：

```bash
curl "http://127.0.0.1:8080/api/v1/products/1"
```

商品 SKU：

```bash
curl "http://127.0.0.1:8080/api/v1/products/1/skus"
```

## 第 15 步：再加 Redis 缓存

等上面接口都跑通后，再给 `ProductService.Detail` 加缓存。

需要 import：

```go
import (
	"encoding/json"
	"fmt"
)
```

缓存 key：

```go
key := fmt.Sprintf("mall:product:detail:%d", id)
```

流程：

```go
cached, err := s.redis.Get(ctx, key).Result()
if err == nil {
	var resp dto.ProductDetailResponse
	if json.Unmarshal([]byte(cached), &resp) == nil {
		return &resp, nil
	}
}
```

查 MySQL 并组装出 `resp` 后：

```go
payload, err := json.Marshal(resp)
if err == nil {
	_ = s.redis.Set(ctx, key, payload, productDetailCacheTTL).Err()
}
```

注意：

- 缓存失败不应该影响接口返回
- MySQL 查询成功才写缓存
- 今天可以先不做空值缓存

## 第 16 步：最终验收

完成后必须满足：

- [ ] `go test ./...` 通过
- [ ] `/health` 正常
- [ ] `/api/v1/categories` 返回分类
- [ ] `/api/v1/products` 返回分页商品
- [ ] `/api/v1/products/:id` 返回商品详情和 SKU
- [ ] `/api/v1/products/:id/skus` 返回 SKU
- [ ] 商品列表有 `min_price`
- [ ] 商品接口默认只查 `merchant_id = 1`
