package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type fakeMerchantCatalogRepository struct {
	merchants     map[int64]model.Merchant
	categories    map[int64]model.Category
	products      map[int64]model.Product
	skus          map[int64]model.ProductSKU
	inventoryLogs []model.InventoryLog
	nextID        int64
}

func (r *fakeMerchantCatalogRepository) Transaction(_ context.Context, fn func(repo repository.MerchantCatalogRepository) error) error {
	return fn(r)
}

func newFakeMerchantCatalogRepository() *fakeMerchantCatalogRepository {
	return &fakeMerchantCatalogRepository{
		merchants: map[int64]model.Merchant{
			1: {ID: 1, Name: "商户一", Status: model.StatusEnabled},
			2: {ID: 2, Name: "商户二", Status: model.StatusEnabled},
		},
		categories: map[int64]model.Category{
			10: {ID: 10, MerchantID: 1, Name: "数码", Status: model.StatusEnabled},
			20: {ID: 20, MerchantID: 2, Name: "其他商户分类", Status: model.StatusEnabled},
		},
		products: map[int64]model.Product{},
		skus:     map[int64]model.ProductSKU{},
		nextID:   100,
	}
}

func (r *fakeMerchantCatalogRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	merchant, ok := r.merchants[merchantID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &merchant, nil
}

func (r *fakeMerchantCatalogRepository) ListCategories(_ context.Context, merchantID int64) ([]model.Category, error) {
	result := []model.Category{}
	for _, category := range r.categories {
		if category.MerchantID == merchantID {
			result = append(result, category)
		}
	}
	return result, nil
}

func (r *fakeMerchantCatalogRepository) FindCategory(_ context.Context, merchantID int64, categoryID int64) (*model.Category, error) {
	category, ok := r.categories[categoryID]
	if !ok || category.MerchantID != merchantID {
		return nil, fmt.Errorf("not found")
	}
	return &category, nil
}

func (r *fakeMerchantCatalogRepository) CreateCategory(_ context.Context, category *model.Category) error {
	r.nextID++
	category.ID = r.nextID
	r.categories[category.ID] = *category
	return nil
}

func (r *fakeMerchantCatalogRepository) UpdateCategory(_ context.Context, category *model.Category) error {
	r.categories[category.ID] = *category
	return nil
}

func (r *fakeMerchantCatalogRepository) DeleteCategory(_ context.Context, merchantID int64, categoryID int64) error {
	category, ok := r.categories[categoryID]
	if !ok || category.MerchantID != merchantID {
		return fmt.Errorf("not found")
	}
	delete(r.categories, categoryID)
	return nil
}

func (r *fakeMerchantCatalogRepository) CountChildCategories(_ context.Context, merchantID int64, categoryID int64) (int64, error) {
	var count int64
	for _, category := range r.categories {
		if category.MerchantID == merchantID && category.ParentID == categoryID {
			count++
		}
	}
	return count, nil
}

func (r *fakeMerchantCatalogRepository) CountProductsByCategory(_ context.Context, merchantID int64, categoryID int64) (int64, error) {
	var count int64
	for _, product := range r.products {
		if product.MerchantID == merchantID && product.CategoryID == categoryID {
			count++
		}
	}
	return count, nil
}

func (r *fakeMerchantCatalogRepository) CountProductsByCategoryAndStatus(_ context.Context, merchantID int64, categoryID int64, status int) (int64, error) {
	var count int64
	for _, product := range r.products {
		if product.MerchantID == merchantID && product.CategoryID == categoryID && product.Status == status {
			count++
		}
	}
	return count, nil
}

func (r *fakeMerchantCatalogRepository) ListProducts(_ context.Context, merchantID int64, _ int, _ int, status *int, keyword string) ([]model.Product, int64, error) {
	result := []model.Product{}
	for _, product := range r.products {
		if product.MerchantID != merchantID || status != nil && product.Status != *status || keyword != "" && !strings.Contains(product.Name, keyword) {
			continue
		}
		result = append(result, product)
	}
	return result, int64(len(result)), nil
}

func (r *fakeMerchantCatalogRepository) FindProduct(_ context.Context, merchantID int64, productID int64) (*model.Product, error) {
	product, ok := r.products[productID]
	if !ok || product.MerchantID != merchantID {
		return nil, fmt.Errorf("not found")
	}
	return &product, nil
}

func (r *fakeMerchantCatalogRepository) CreateProduct(_ context.Context, product *model.Product) error {
	r.nextID++
	product.ID = r.nextID
	product.CreatedAt = time.Now()
	product.UpdatedAt = product.CreatedAt
	r.products[product.ID] = *product
	return nil
}

func (r *fakeMerchantCatalogRepository) UpdateProduct(_ context.Context, product *model.Product) error {
	r.products[product.ID] = *product
	return nil
}

func (r *fakeMerchantCatalogRepository) UpdateProductStatus(_ context.Context, merchantID int64, productID int64, status int) error {
	product, ok := r.products[productID]
	if !ok || product.MerchantID != merchantID {
		return fmt.Errorf("not found")
	}
	product.Status = status
	r.products[productID] = product
	return nil
}

func (r *fakeMerchantCatalogRepository) DeleteProduct(_ context.Context, merchantID int64, productID int64) error {
	product, ok := r.products[productID]
	if !ok || product.MerchantID != merchantID {
		return fmt.Errorf("not found")
	}
	delete(r.products, productID)
	return nil
}

func (r *fakeMerchantCatalogRepository) ListSKUsByProductID(_ context.Context, merchantID int64, productID int64) ([]model.ProductSKU, error) {
	return r.ListSKUsByProductIDs(context.Background(), merchantID, []int64{productID})
}

func (r *fakeMerchantCatalogRepository) ListSKUsByProductIDs(_ context.Context, merchantID int64, productIDs []int64) ([]model.ProductSKU, error) {
	ids := map[int64]struct{}{}
	for _, id := range productIDs {
		ids[id] = struct{}{}
	}
	result := []model.ProductSKU{}
	for _, sku := range r.skus {
		if sku.MerchantID == merchantID {
			if _, ok := ids[sku.ProductID]; ok {
				result = append(result, sku)
			}
		}
	}
	return result, nil
}

func (r *fakeMerchantCatalogRepository) FindSKU(_ context.Context, merchantID int64, productID int64, skuID int64) (*model.ProductSKU, error) {
	sku, ok := r.skus[skuID]
	if !ok || sku.MerchantID != merchantID || sku.ProductID != productID {
		return nil, fmt.Errorf("not found")
	}
	return &sku, nil
}

func (r *fakeMerchantCatalogRepository) FindSKUForUpdate(ctx context.Context, merchantID int64, productID int64, skuID int64) (*model.ProductSKU, error) {
	return r.FindSKU(ctx, merchantID, productID, skuID)
}

func (r *fakeMerchantCatalogRepository) CreateSKU(_ context.Context, sku *model.ProductSKU) error {
	r.nextID++
	sku.ID = r.nextID
	r.skus[sku.ID] = *sku
	return nil
}

func (r *fakeMerchantCatalogRepository) UpdateSKU(_ context.Context, sku *model.ProductSKU) error {
	r.skus[sku.ID] = *sku
	return nil
}

func (r *fakeMerchantCatalogRepository) CreateInventoryLog(_ context.Context, log *model.InventoryLog) error {
	r.inventoryLogs = append(r.inventoryLogs, *log)
	return nil
}

func (r *fakeMerchantCatalogRepository) DeleteSKU(_ context.Context, merchantID int64, productID int64, skuID int64) error {
	if _, err := r.FindSKU(context.Background(), merchantID, productID, skuID); err != nil {
		return err
	}
	delete(r.skus, skuID)
	return nil
}

func (r *fakeMerchantCatalogRepository) DeleteSKUsByProductID(_ context.Context, merchantID int64, productID int64) error {
	for id, sku := range r.skus {
		if sku.MerchantID == merchantID && sku.ProductID == productID {
			delete(r.skus, id)
		}
	}
	return nil
}

func (r *fakeMerchantCatalogRepository) CountEnabledSKUs(_ context.Context, merchantID int64, productID int64, excludeSKUID int64) (int64, error) {
	var count int64
	for _, sku := range r.skus {
		if sku.MerchantID == merchantID && sku.ProductID == productID && sku.ID != excludeSKUID && sku.Status == model.StatusEnabled && sku.Price > 0 {
			count++
		}
	}
	return count, nil
}

func intPointer(value int) *int {
	return &value
}

func TestMerchantCatalogDefaultsAndTenantIsolation(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	service := NewMerchantCatalogService(repo)
	ctx := context.Background()

	category, err := service.CreateCategory(ctx, 1, dto.MerchantCategoryRequest{Name: " 新分类 "})
	if err != nil {
		t.Fatalf("create category returned error: %v", err)
	}
	if category.Status != model.StatusEnabled || category.Name != "新分类" || category.MerchantID != 1 {
		t.Fatalf("unexpected category: %+v", category)
	}

	product, err := service.CreateProduct(ctx, 1, dto.MerchantProductRequest{CategoryID: 10, Name: "测试商品"})
	if err != nil {
		t.Fatalf("create product returned error: %v", err)
	}
	if product.Status != model.ProductStatusDraft || product.MerchantID != 1 {
		t.Fatalf("unexpected product: %+v", product)
	}

	if _, err := service.ProductDetail(ctx, 2, product.ID); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("expected cross-merchant access rejection, got %v", err)
	}
}

func TestMerchantCatalogProductSaleRequiresSKU(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	service := NewMerchantCatalogService(repo)
	ctx := context.Background()
	product, err := service.CreateProduct(ctx, 1, dto.MerchantProductRequest{CategoryID: 10, Name: "测试商品"})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.OnSaleProduct(ctx, 1, product.ID); err == nil || !strings.Contains(err.Error(), "SKU") {
		t.Fatalf("expected missing SKU rejection, got %v", err)
	}
	sku, err := service.CreateSKU(ctx, 1, 99, product.ID, dto.MerchantSKURequest{Name: "默认规格", Price: 9900, Stock: 10})
	if err != nil {
		t.Fatal(err)
	}
	if sku.Status != model.StatusEnabled {
		t.Fatalf("new SKU should default to enabled: %+v", sku)
	}
	if err := service.OnSaleProduct(ctx, 1, product.ID); err != nil {
		t.Fatalf("on sale returned error: %v", err)
	}
	if err := service.DeleteSKU(ctx, 1, product.ID, sku.ID); err == nil || !strings.Contains(err.Error(), "先下架商品") {
		t.Fatalf("expected last SKU deletion rejection, got %v", err)
	}
	if _, err := service.UpdateSKU(ctx, 1, 99, product.ID, sku.ID, dto.MerchantSKURequest{
		Name: "默认规格", Price: 9900, Stock: 10, Status: intPointer(model.StatusDisabled),
	}); err == nil || !strings.Contains(err.Error(), "先下架商品") {
		t.Fatalf("expected last SKU disable rejection, got %v", err)
	}
}

func TestMerchantCatalogRecordsStockAdjustments(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.products[30] = model.Product{
		ID: 30, MerchantID: 1, CategoryID: 10, Name: "测试商品", Status: model.ProductStatusDraft,
	}
	service := NewMerchantCatalogService(repo)

	sku, err := service.CreateSKU(context.Background(), 1, 99, 30, dto.MerchantSKURequest{
		Name: "默认规格", Price: 100, Stock: 8,
	})
	if err != nil {
		t.Fatalf("create SKU returned error: %v", err)
	}
	if len(repo.inventoryLogs) != 1 || repo.inventoryLogs[0].Quantity != 8 || repo.inventoryLogs[0].OperatorID != 99 {
		t.Fatalf("unexpected initial inventory log: %+v", repo.inventoryLogs)
	}

	if _, err := service.UpdateSKU(context.Background(), 1, 99, 30, sku.ID, dto.MerchantSKURequest{
		Name: "默认规格", Price: 100, Stock: 3,
	}); err != nil {
		t.Fatalf("update SKU returned error: %v", err)
	}
	if len(repo.inventoryLogs) != 2 || repo.inventoryLogs[1].Quantity != -5 || repo.inventoryLogs[1].BeforeStock != 8 || repo.inventoryLogs[1].AfterStock != 3 {
		t.Fatalf("unexpected adjustment inventory log: %+v", repo.inventoryLogs)
	}
}

func TestMerchantCatalogManagesLowStockThreshold(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.products[30] = model.Product{
		ID: 30, MerchantID: 1, CategoryID: 10, Name: "测试商品", Status: model.ProductStatusDraft,
	}
	service := NewMerchantCatalogService(repo)
	threshold := 5

	sku, err := service.CreateSKU(context.Background(), 1, 99, 30, dto.MerchantSKURequest{
		Name: "默认规格", Price: 100, Stock: 8, LowStockThreshold: &threshold,
	})
	if err != nil {
		t.Fatalf("create SKU returned error: %v", err)
	}
	if sku.LowStockThreshold != 5 {
		t.Fatalf("unexpected threshold after create: %+v", sku)
	}

	if _, err := service.UpdateSKU(context.Background(), 1, 99, 30, sku.ID, dto.MerchantSKURequest{
		Name: "默认规格", Price: 100, Stock: 3,
	}); err != nil {
		t.Fatalf("update SKU returned error: %v", err)
	}
	if repo.skus[sku.ID].LowStockThreshold != 5 {
		t.Fatalf("omitted threshold should be preserved: %+v", repo.skus[sku.ID])
	}

	invalidThreshold := -1
	if _, err := service.UpdateSKU(context.Background(), 1, 99, 30, sku.ID, dto.MerchantSKURequest{
		Name: "默认规格", Price: 100, Stock: 3, LowStockThreshold: &invalidThreshold,
	}); err == nil || !strings.Contains(err.Error(), "预警阈值") {
		t.Fatalf("expected invalid threshold rejection, got %v", err)
	}
}

func TestMerchantCatalogRejectsCategoryCycle(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.categories[11] = model.Category{ID: 11, MerchantID: 1, ParentID: 10, Name: "手机", Status: model.StatusEnabled}
	service := NewMerchantCatalogService(repo)

	_, err := service.UpdateCategory(context.Background(), 1, 10, dto.MerchantCategoryRequest{
		ParentID: 11,
		Name:     "数码",
	})
	if err == nil || !strings.Contains(err.Error(), "子分类") {
		t.Fatalf("expected category cycle rejection, got %v", err)
	}
}

func TestMerchantCatalogRejectsDisablingCategoryWithOnSaleProducts(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.products[30] = model.Product{
		ID: 30, MerchantID: 1, CategoryID: 10, Name: "在售商品", Status: model.ProductStatusOnSale,
	}
	service := NewMerchantCatalogService(repo)

	_, err := service.UpdateCategory(context.Background(), 1, 10, dto.MerchantCategoryRequest{
		Name:   "数码",
		Status: intPointer(model.StatusDisabled),
	})
	if err == nil || !strings.Contains(err.Error(), "先下架商品") {
		t.Fatalf("expected category disable rejection, got %v", err)
	}
}

func TestMerchantCatalogDeletesOffSaleProductAndSKUs(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.products[30] = model.Product{
		ID: 30, MerchantID: 1, CategoryID: 10, Name: "下架商品", Status: model.ProductStatusOffSale,
	}
	repo.skus[31] = model.ProductSKU{
		ID: 31, MerchantID: 1, ProductID: 30, Name: "默认规格", Price: 100, Status: model.StatusEnabled,
	}
	service := NewMerchantCatalogService(repo)

	if err := service.DeleteProduct(context.Background(), 1, 30); err != nil {
		t.Fatalf("delete product returned error: %v", err)
	}
	if _, exists := repo.products[30]; exists {
		t.Fatal("product was not deleted")
	}
	if _, exists := repo.skus[31]; exists {
		t.Fatal("product SKUs were not deleted")
	}
}

func TestMerchantCatalogRejectsDeletingOnSaleProduct(t *testing.T) {
	repo := newFakeMerchantCatalogRepository()
	repo.products[30] = model.Product{
		ID: 30, MerchantID: 1, CategoryID: 10, Name: "上架商品", Status: model.ProductStatusOnSale,
	}
	service := NewMerchantCatalogService(repo)

	err := service.DeleteProduct(context.Background(), 1, 30)
	if err == nil || !strings.Contains(err.Error(), "先下架商品") {
		t.Fatalf("expected on-sale deletion rejection, got %v", err)
	}
}
