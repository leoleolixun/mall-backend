package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type MerchantCatalogService interface {
	ListCategories(ctx context.Context, merchantID int64) ([]dto.MerchantCategoryResponse, error)
	CreateCategory(ctx context.Context, merchantID int64, req dto.MerchantCategoryRequest) (*dto.MerchantCategoryResponse, error)
	UpdateCategory(ctx context.Context, merchantID int64, categoryID int64, req dto.MerchantCategoryRequest) (*dto.MerchantCategoryResponse, error)
	DeleteCategory(ctx context.Context, merchantID int64, categoryID int64) error

	ListProducts(ctx context.Context, merchantID int64, req dto.MerchantProductListRequest) (*dto.PageResponse[dto.MerchantProductResponse], error)
	ProductDetail(ctx context.Context, merchantID int64, productID int64) (*dto.MerchantProductResponse, error)
	CreateProduct(ctx context.Context, merchantID int64, req dto.MerchantProductRequest) (*dto.MerchantProductResponse, error)
	UpdateProduct(ctx context.Context, merchantID int64, productID int64, req dto.MerchantProductRequest) (*dto.MerchantProductResponse, error)
	DeleteProduct(ctx context.Context, merchantID int64, productID int64) error
	OnSaleProduct(ctx context.Context, merchantID int64, productID int64) error
	OffSaleProduct(ctx context.Context, merchantID int64, productID int64) error

	CreateSKU(ctx context.Context, merchantID int64, operatorID int64, productID int64, req dto.MerchantSKURequest) (*dto.MerchantSKUResponse, error)
	UpdateSKU(ctx context.Context, merchantID int64, operatorID int64, productID int64, skuID int64, req dto.MerchantSKURequest) (*dto.MerchantSKUResponse, error)
	DeleteSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) error
}

type merchantCatalogService struct {
	repo repository.MerchantCatalogRepository
}

func NewMerchantCatalogService(repo repository.MerchantCatalogRepository) MerchantCatalogService {
	return &merchantCatalogService{repo: repo}
}

func (s *merchantCatalogService) validateMerchant(ctx context.Context, merchantID int64) error {
	if merchantID <= 0 {
		return fmt.Errorf("商户身份不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return fmt.Errorf("商户不可用")
	}
	return nil
}

func validEnabledStatus(status int) bool {
	return status == model.StatusDisabled || status == model.StatusEnabled
}

func validProductStatus(status int) bool {
	return status == model.ProductStatusDraft || status == model.ProductStatusOnSale || status == model.ProductStatusOffSale
}

func toMerchantCategoryResponse(category model.Category) dto.MerchantCategoryResponse {
	return dto.MerchantCategoryResponse{
		ID:         category.ID,
		MerchantID: category.MerchantID,
		ParentID:   category.ParentID,
		Name:       category.Name,
		Sort:       category.Sort,
		Status:     category.Status,
	}
}

func toMerchantSKUResponse(sku model.ProductSKU) dto.MerchantSKUResponse {
	return dto.MerchantSKUResponse{
		ID:                sku.ID,
		MerchantID:        sku.MerchantID,
		ProductID:         sku.ProductID,
		Name:              sku.Name,
		Image:             sku.Image,
		Price:             sku.Price,
		Stock:             sku.Stock,
		LowStockThreshold: sku.LowStockThreshold,
		Status:            sku.Status,
	}
}

func toMerchantProductResponse(product model.Product, skus []model.ProductSKU) dto.MerchantProductResponse {
	skuResponses := make([]dto.MerchantSKUResponse, 0, len(skus))
	for _, sku := range skus {
		skuResponses = append(skuResponses, toMerchantSKUResponse(sku))
	}
	return dto.MerchantProductResponse{
		ID:          product.ID,
		MerchantID:  product.MerchantID,
		CategoryID:  product.CategoryID,
		Name:        product.Name,
		Cover:       product.Cover,
		Description: product.Description,
		Status:      product.Status,
		SKUs:        skuResponses,
		CreatedAt:   product.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   product.UpdatedAt.Format(time.RFC3339),
	}
}

func validateCategoryRequest(req dto.MerchantCategoryRequest) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", fmt.Errorf("分类名称不能为空")
	}
	if len([]rune(name)) > 100 {
		return "", fmt.Errorf("分类名称不能超过 100 个字符")
	}
	if req.ParentID < 0 {
		return "", fmt.Errorf("父分类 ID 不合法")
	}
	if req.Status != nil && !validEnabledStatus(*req.Status) {
		return "", fmt.Errorf("分类状态不合法")
	}
	return name, nil
}

func (s *merchantCatalogService) validateCategoryParent(
	ctx context.Context,
	merchantID int64,
	categoryID int64,
	parentID int64,
) error {
	visited := map[int64]struct{}{}
	for parentID > 0 {
		if parentID == categoryID {
			return fmt.Errorf("父分类不能是当前分类或其子分类")
		}
		if _, exists := visited[parentID]; exists {
			return fmt.Errorf("分类层级存在循环")
		}
		visited[parentID] = struct{}{}
		parent, err := s.repo.FindCategory(ctx, merchantID, parentID)
		if err != nil {
			return fmt.Errorf("父分类不存在")
		}
		parentID = parent.ParentID
	}
	return nil
}

func (s *merchantCatalogService) ListCategories(ctx context.Context, merchantID int64) ([]dto.MerchantCategoryResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	categories, err := s.repo.ListCategories(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.MerchantCategoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, toMerchantCategoryResponse(category))
	}
	return result, nil
}

func (s *merchantCatalogService) CreateCategory(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantCategoryRequest,
) (*dto.MerchantCategoryResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, err := validateCategoryRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.validateCategoryParent(ctx, merchantID, 0, req.ParentID); err != nil {
		return nil, err
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	category := &model.Category{
		MerchantID: merchantID,
		ParentID:   req.ParentID,
		Name:       name,
		Sort:       req.Sort,
		Status:     status,
	}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	response := toMerchantCategoryResponse(*category)
	return &response, nil
}

func (s *merchantCatalogService) UpdateCategory(
	ctx context.Context,
	merchantID int64,
	categoryID int64,
	req dto.MerchantCategoryRequest,
) (*dto.MerchantCategoryResponse, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("分类 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, err := validateCategoryRequest(req)
	if err != nil {
		return nil, err
	}
	category, err := s.repo.FindCategory(ctx, merchantID, categoryID)
	if err != nil {
		return nil, fmt.Errorf("分类不存在")
	}
	if err := s.validateCategoryParent(ctx, merchantID, categoryID, req.ParentID); err != nil {
		return nil, err
	}
	category.ParentID = req.ParentID
	category.Name = name
	category.Sort = req.Sort
	if req.Status != nil {
		if *req.Status == model.StatusDisabled && category.Status != model.StatusDisabled {
			onSaleProducts, err := s.repo.CountProductsByCategoryAndStatus(
				ctx,
				merchantID,
				categoryID,
				model.ProductStatusOnSale,
			)
			if err != nil {
				return nil, err
			}
			if onSaleProducts > 0 {
				return nil, fmt.Errorf("分类下存在上架商品，请先下架商品")
			}
		}
		category.Status = *req.Status
	}
	if err := s.repo.UpdateCategory(ctx, category); err != nil {
		return nil, err
	}
	response := toMerchantCategoryResponse(*category)
	return &response, nil
}

func (s *merchantCatalogService) DeleteCategory(ctx context.Context, merchantID int64, categoryID int64) error {
	if categoryID <= 0 {
		return fmt.Errorf("分类 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return err
	}
	if _, err := s.repo.FindCategory(ctx, merchantID, categoryID); err != nil {
		return fmt.Errorf("分类不存在")
	}
	children, err := s.repo.CountChildCategories(ctx, merchantID, categoryID)
	if err != nil {
		return err
	}
	if children > 0 {
		return fmt.Errorf("请先删除子分类")
	}
	products, err := s.repo.CountProductsByCategory(ctx, merchantID, categoryID)
	if err != nil {
		return err
	}
	if products > 0 {
		return fmt.Errorf("分类下存在商品，不能删除")
	}
	return s.repo.DeleteCategory(ctx, merchantID, categoryID)
}

func validateMerchantProductRequest(req dto.MerchantProductRequest) (string, string, error) {
	name := strings.TrimSpace(req.Name)
	cover := strings.TrimSpace(req.Cover)
	if req.CategoryID <= 0 {
		return "", "", fmt.Errorf("商品分类不能为空")
	}
	if name == "" {
		return "", "", fmt.Errorf("商品名称不能为空")
	}
	if len([]rune(name)) > 200 {
		return "", "", fmt.Errorf("商品名称不能超过 200 个字符")
	}
	if len(cover) > 255 {
		return "", "", fmt.Errorf("商品封面地址过长")
	}
	if req.Status != nil && !validProductStatus(*req.Status) {
		return "", "", fmt.Errorf("商品状态不合法")
	}
	return name, cover, nil
}

func (s *merchantCatalogService) ensureCategory(ctx context.Context, merchantID int64, categoryID int64) error {
	category, err := s.repo.FindCategory(ctx, merchantID, categoryID)
	if err != nil {
		return fmt.Errorf("商品分类不存在")
	}
	if category.Status != model.StatusEnabled {
		return fmt.Errorf("商品分类已禁用")
	}
	return nil
}

func (s *merchantCatalogService) ensureProductCanBeOnSale(ctx context.Context, merchantID int64, productID int64) error {
	count, err := s.repo.CountEnabledSKUs(ctx, merchantID, productID, 0)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("商品至少需要一个价格大于 0 的启用 SKU 才能上架")
	}
	return nil
}

func normalizeMerchantProductPage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func (s *merchantCatalogService) ListProducts(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantProductListRequest,
) (*dto.PageResponse[dto.MerchantProductResponse], error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	if req.Status != nil && !validProductStatus(*req.Status) {
		return nil, fmt.Errorf("status 参数不合法")
	}
	page, pageSize := normalizeMerchantProductPage(req.Page, req.PageSize)
	products, total, err := s.repo.ListProducts(ctx, merchantID, (page-1)*pageSize, pageSize, req.Status, strings.TrimSpace(req.Keyword))
	if err != nil {
		return nil, err
	}
	productIDs := make([]int64, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}
	skus, err := s.repo.ListSKUsByProductIDs(ctx, merchantID, productIDs)
	if err != nil {
		return nil, err
	}
	skusByProductID := make(map[int64][]model.ProductSKU, len(products))
	for _, sku := range skus {
		skusByProductID[sku.ProductID] = append(skusByProductID[sku.ProductID], sku)
	}
	list := make([]dto.MerchantProductResponse, 0, len(products))
	for _, product := range products {
		list = append(list, toMerchantProductResponse(product, skusByProductID[product.ID]))
	}
	return &dto.PageResponse[dto.MerchantProductResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *merchantCatalogService) ProductDetail(ctx context.Context, merchantID int64, productID int64) (*dto.MerchantProductResponse, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	product, err := s.repo.FindProduct(ctx, merchantID, productID)
	if err != nil {
		return nil, fmt.Errorf("商品不存在")
	}
	skus, err := s.repo.ListSKUsByProductID(ctx, merchantID, productID)
	if err != nil {
		return nil, err
	}
	response := toMerchantProductResponse(*product, skus)
	return &response, nil
}

func (s *merchantCatalogService) CreateProduct(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantProductRequest,
) (*dto.MerchantProductResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, cover, err := validateMerchantProductRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCategory(ctx, merchantID, req.CategoryID); err != nil {
		return nil, err
	}
	status := model.ProductStatusDraft
	if req.Status != nil {
		status = *req.Status
	}
	if status == model.ProductStatusOnSale {
		return nil, fmt.Errorf("请先创建 SKU，再上架商品")
	}
	product := &model.Product{
		MerchantID:  merchantID,
		CategoryID:  req.CategoryID,
		Name:        name,
		Cover:       cover,
		Description: strings.TrimSpace(req.Description),
		Status:      status,
	}
	if err := s.repo.CreateProduct(ctx, product); err != nil {
		return nil, err
	}
	response := toMerchantProductResponse(*product, []model.ProductSKU{})
	return &response, nil
}

func (s *merchantCatalogService) UpdateProduct(
	ctx context.Context,
	merchantID int64,
	productID int64,
	req dto.MerchantProductRequest,
) (*dto.MerchantProductResponse, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, cover, err := validateMerchantProductRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCategory(ctx, merchantID, req.CategoryID); err != nil {
		return nil, err
	}
	product, err := s.repo.FindProduct(ctx, merchantID, productID)
	if err != nil {
		return nil, fmt.Errorf("商品不存在")
	}
	if req.Status != nil && *req.Status == model.ProductStatusOnSale {
		if err := s.ensureProductCanBeOnSale(ctx, merchantID, productID); err != nil {
			return nil, err
		}
	}
	product.CategoryID = req.CategoryID
	product.Name = name
	product.Cover = cover
	product.Description = strings.TrimSpace(req.Description)
	if req.Status != nil {
		product.Status = *req.Status
	}
	if err := s.repo.UpdateProduct(ctx, product); err != nil {
		return nil, err
	}
	return s.ProductDetail(ctx, merchantID, productID)
}

func (s *merchantCatalogService) OnSaleProduct(ctx context.Context, merchantID int64, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return err
	}
	product, err := s.repo.FindProduct(ctx, merchantID, productID)
	if err != nil {
		return fmt.Errorf("商品不存在")
	}
	if err := s.ensureCategory(ctx, merchantID, product.CategoryID); err != nil {
		return err
	}
	if err := s.ensureProductCanBeOnSale(ctx, merchantID, productID); err != nil {
		return err
	}
	return s.repo.UpdateProductStatus(ctx, merchantID, productID, model.ProductStatusOnSale)
}

func (s *merchantCatalogService) OffSaleProduct(ctx context.Context, merchantID int64, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return err
	}
	if _, err := s.repo.FindProduct(ctx, merchantID, productID); err != nil {
		return fmt.Errorf("商品不存在")
	}
	return s.repo.UpdateProductStatus(ctx, merchantID, productID, model.ProductStatusOffSale)
}

func (s *merchantCatalogService) DeleteProduct(ctx context.Context, merchantID int64, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return err
	}

	return s.repo.Transaction(ctx, func(repo repository.MerchantCatalogRepository) error {
		product, err := repo.FindProduct(ctx, merchantID, productID)
		if err != nil {
			return fmt.Errorf("商品不存在")
		}
		if product.Status == model.ProductStatusOnSale {
			return fmt.Errorf("上架商品不能删除，请先下架商品")
		}
		if err := repo.DeleteSKUsByProductID(ctx, merchantID, productID); err != nil {
			return err
		}
		return repo.DeleteProduct(ctx, merchantID, productID)
	})
}

func validateMerchantSKURequest(req dto.MerchantSKURequest) (string, string, error) {
	name := strings.TrimSpace(req.Name)
	image := strings.TrimSpace(req.Image)
	if name == "" {
		return "", "", fmt.Errorf("SKU 名称不能为空")
	}
	if len([]rune(name)) > 200 {
		return "", "", fmt.Errorf("SKU 名称不能超过 200 个字符")
	}
	if len(image) > 255 {
		return "", "", fmt.Errorf("SKU 图片地址过长")
	}
	if req.Price <= 0 {
		return "", "", fmt.Errorf("SKU 价格必须大于 0")
	}
	if req.Stock < 0 {
		return "", "", fmt.Errorf("SKU 库存不能小于 0")
	}
	if req.LowStockThreshold != nil && *req.LowStockThreshold < 0 {
		return "", "", fmt.Errorf("低库存预警阈值不能小于 0")
	}
	if req.Status != nil && !validEnabledStatus(*req.Status) {
		return "", "", fmt.Errorf("SKU 状态不合法")
	}
	return name, image, nil
}

func (s *merchantCatalogService) CreateSKU(
	ctx context.Context,
	merchantID int64,
	operatorID int64,
	productID int64,
	req dto.MerchantSKURequest,
) (*dto.MerchantSKUResponse, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("商品 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, image, err := validateMerchantSKURequest(req)
	if err != nil {
		return nil, err
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	lowStockThreshold := 0
	if req.LowStockThreshold != nil {
		lowStockThreshold = *req.LowStockThreshold
	}
	sku := &model.ProductSKU{
		MerchantID:        merchantID,
		ProductID:         productID,
		Name:              name,
		Image:             image,
		Price:             req.Price,
		Stock:             req.Stock,
		LowStockThreshold: lowStockThreshold,
		Status:            status,
	}
	if err := s.repo.Transaction(ctx, func(repo repository.MerchantCatalogRepository) error {
		product, err := repo.FindProduct(ctx, merchantID, productID)
		if err != nil {
			return fmt.Errorf("商品不存在")
		}
		if err := repo.CreateSKU(ctx, sku); err != nil {
			return err
		}
		if sku.Stock == 0 {
			return nil
		}
		return repo.CreateInventoryLog(ctx, &model.InventoryLog{
			MerchantID:    merchantID,
			ProductID:     productID,
			SKUID:         sku.ID,
			ProductName:   product.Name,
			SKUName:       sku.Name,
			ChangeType:    model.InventoryChangeMerchantInit,
			Quantity:      sku.Stock,
			BeforeStock:   0,
			AfterStock:    sku.Stock,
			ReferenceType: model.InventoryReferenceSKU,
			ReferenceID:   sku.ID,
			OperatorType:  model.InventoryOperatorMerchant,
			OperatorID:    operatorID,
			Remark:        "商家创建 SKU 初始化库存",
		})
	}); err != nil {
		return nil, err
	}
	response := toMerchantSKUResponse(*sku)
	return &response, nil
}

func (s *merchantCatalogService) UpdateSKU(
	ctx context.Context,
	merchantID int64,
	operatorID int64,
	productID int64,
	skuID int64,
	req dto.MerchantSKURequest,
) (*dto.MerchantSKUResponse, error) {
	if productID <= 0 || skuID <= 0 {
		return nil, fmt.Errorf("商品 ID 或 SKU ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	name, image, err := validateMerchantSKURequest(req)
	if err != nil {
		return nil, err
	}
	product, err := s.repo.FindProduct(ctx, merchantID, productID)
	if err != nil {
		return nil, fmt.Errorf("商品不存在")
	}
	var updatedSKU *model.ProductSKU
	err = s.repo.Transaction(ctx, func(repo repository.MerchantCatalogRepository) error {
		sku, err := repo.FindSKUForUpdate(ctx, merchantID, productID, skuID)
		if err != nil {
			return fmt.Errorf("SKU 不存在")
		}
		nextStatus := sku.Status
		if req.Status != nil {
			nextStatus = *req.Status
		}
		if product.Status == model.ProductStatusOnSale && sku.Status == model.StatusEnabled && nextStatus == model.StatusDisabled {
			count, err := repo.CountEnabledSKUs(ctx, merchantID, productID, skuID)
			if err != nil {
				return err
			}
			if count == 0 {
				return fmt.Errorf("上架商品至少需要一个启用 SKU，请先下架商品")
			}
		}
		beforeStock := sku.Stock
		sku.Name = name
		sku.Image = image
		sku.Price = req.Price
		sku.Stock = req.Stock
		if req.LowStockThreshold != nil {
			sku.LowStockThreshold = *req.LowStockThreshold
		}
		sku.Status = nextStatus
		if err := repo.UpdateSKU(ctx, sku); err != nil {
			return err
		}
		if beforeStock != sku.Stock {
			if err := repo.CreateInventoryLog(ctx, &model.InventoryLog{
				MerchantID:    merchantID,
				ProductID:     productID,
				SKUID:         sku.ID,
				ProductName:   product.Name,
				SKUName:       sku.Name,
				ChangeType:    model.InventoryChangeMerchantAdjustment,
				Quantity:      sku.Stock - beforeStock,
				BeforeStock:   beforeStock,
				AfterStock:    sku.Stock,
				ReferenceType: model.InventoryReferenceSKU,
				ReferenceID:   sku.ID,
				OperatorType:  model.InventoryOperatorMerchant,
				OperatorID:    operatorID,
				Remark:        "商家调整 SKU 库存",
			}); err != nil {
				return err
			}
		}
		updatedSKU = sku
		return nil
	})
	if err != nil {
		return nil, err
	}
	response := toMerchantSKUResponse(*updatedSKU)
	return &response, nil
}

func (s *merchantCatalogService) DeleteSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) error {
	if productID <= 0 || skuID <= 0 {
		return fmt.Errorf("商品 ID 或 SKU ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return err
	}
	product, err := s.repo.FindProduct(ctx, merchantID, productID)
	if err != nil {
		return fmt.Errorf("商品不存在")
	}
	sku, err := s.repo.FindSKU(ctx, merchantID, productID, skuID)
	if err != nil {
		return fmt.Errorf("SKU 不存在")
	}
	if product.Status == model.ProductStatusOnSale && sku.Status == model.StatusEnabled {
		count, err := s.repo.CountEnabledSKUs(ctx, merchantID, productID, skuID)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("上架商品至少需要一个启用 SKU，请先下架商品")
		}
	}
	return s.repo.DeleteSKU(ctx, merchantID, productID, skuID)
}
