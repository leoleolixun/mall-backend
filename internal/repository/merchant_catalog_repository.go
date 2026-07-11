package repository

import (
	"context"
	"errors"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchantCatalogRepository interface {
	Transaction(ctx context.Context, fn func(repo MerchantCatalogRepository) error) error
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)

	ListCategories(ctx context.Context, merchantID int64) ([]model.Category, error)
	FindCategory(ctx context.Context, merchantID int64, categoryID int64) (*model.Category, error)
	CreateCategory(ctx context.Context, category *model.Category) error
	UpdateCategory(ctx context.Context, category *model.Category) error
	DeleteCategory(ctx context.Context, merchantID int64, categoryID int64) error
	CountChildCategories(ctx context.Context, merchantID int64, categoryID int64) (int64, error)
	CountProductsByCategory(ctx context.Context, merchantID int64, categoryID int64) (int64, error)
	CountProductsByCategoryAndStatus(ctx context.Context, merchantID int64, categoryID int64, status int) (int64, error)

	ListProducts(ctx context.Context, merchantID int64, offset int, limit int, status *int, keyword string) ([]model.Product, int64, error)
	FindProduct(ctx context.Context, merchantID int64, productID int64) (*model.Product, error)
	CreateProduct(ctx context.Context, product *model.Product) error
	UpdateProduct(ctx context.Context, product *model.Product) error
	UpdateProductStatus(ctx context.Context, merchantID int64, productID int64, status int) error
	DeleteProduct(ctx context.Context, merchantID int64, productID int64) error

	ListSKUsByProductID(ctx context.Context, merchantID int64, productID int64) ([]model.ProductSKU, error)
	ListSKUsByProductIDs(ctx context.Context, merchantID int64, productIDs []int64) ([]model.ProductSKU, error)
	FindSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) (*model.ProductSKU, error)
	FindSKUForUpdate(ctx context.Context, merchantID int64, productID int64, skuID int64) (*model.ProductSKU, error)
	CreateSKU(ctx context.Context, sku *model.ProductSKU) error
	UpdateSKU(ctx context.Context, sku *model.ProductSKU) error
	CreateInventoryLog(ctx context.Context, log *model.InventoryLog) error
	DeleteSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) error
	DeleteSKUsByProductID(ctx context.Context, merchantID int64, productID int64) error
	CountEnabledSKUs(ctx context.Context, merchantID int64, productID int64, excludeSKUID int64) (int64, error)
}

type merchantCatalogRepository struct {
	db *gorm.DB
}

func NewMerchantCatalogRepository(db *gorm.DB) MerchantCatalogRepository {
	return &merchantCatalogRepository{db: db}
}

func (r *merchantCatalogRepository) Transaction(
	ctx context.Context,
	fn func(repo MerchantCatalogRepository) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&merchantCatalogRepository{db: tx})
	})
}

func (r *merchantCatalogRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantCatalogRepository) ListCategories(ctx context.Context, merchantID int64) ([]model.Category, error) {
	var categories []model.Category
	err := r.db.WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		Order("sort DESC, id ASC").
		Find(&categories).Error
	return categories, err
}

func (r *merchantCatalogRepository) FindCategory(ctx context.Context, merchantID int64, categoryID int64) (*model.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", categoryID, merchantID).
		First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *merchantCatalogRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *merchantCatalogRepository) UpdateCategory(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ? AND merchant_id = ?", category.ID, category.MerchantID).
		Updates(map[string]interface{}{
			"parent_id": category.ParentID,
			"name":      category.Name,
			"sort":      category.Sort,
			"status":    category.Status,
		}).Error
}

func (r *merchantCatalogRepository) DeleteCategory(ctx context.Context, merchantID int64, categoryID int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", categoryID, merchantID).
		Delete(&model.Category{})
	return affectedOrNotFound(result, "分类不存在")
}

func (r *merchantCatalogRepository) CountChildCategories(ctx context.Context, merchantID int64, categoryID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Category{}).
		Where("merchant_id = ? AND parent_id = ?", merchantID, categoryID).
		Count(&count).Error
	return count, err
}

func (r *merchantCatalogRepository) CountProductsByCategory(ctx context.Context, merchantID int64, categoryID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("merchant_id = ? AND category_id = ?", merchantID, categoryID).
		Count(&count).Error
	return count, err
}

func (r *merchantCatalogRepository) CountProductsByCategoryAndStatus(
	ctx context.Context,
	merchantID int64,
	categoryID int64,
	status int,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("merchant_id = ? AND category_id = ? AND status = ?", merchantID, categoryID, status).
		Count(&count).Error
	return count, err
}

func (r *merchantCatalogRepository) ListProducts(
	ctx context.Context,
	merchantID int64,
	offset int,
	limit int,
	status *int,
	keyword string,
) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Product{}).Where("merchant_id = ?", merchantID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *merchantCatalogRepository) FindProduct(ctx context.Context, merchantID int64, productID int64) (*model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", productID, merchantID).
		First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *merchantCatalogRepository) CreateProduct(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).Create(product).Error
}

func (r *merchantCatalogRepository) UpdateProduct(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ? AND merchant_id = ?", product.ID, product.MerchantID).
		Updates(map[string]interface{}{
			"category_id": product.CategoryID,
			"name":        product.Name,
			"cover":       product.Cover,
			"description": product.Description,
			"status":      product.Status,
		}).Error
}

func (r *merchantCatalogRepository) UpdateProductStatus(ctx context.Context, merchantID int64, productID int64, status int) error {
	return r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ? AND merchant_id = ?", productID, merchantID).
		Update("status", status).Error
}

func (r *merchantCatalogRepository) DeleteProduct(ctx context.Context, merchantID int64, productID int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", productID, merchantID).
		Delete(&model.Product{})
	return affectedOrNotFound(result, "商品不存在")
}

func (r *merchantCatalogRepository) ListSKUsByProductID(ctx context.Context, merchantID int64, productID int64) ([]model.ProductSKU, error) {
	return r.ListSKUsByProductIDs(ctx, merchantID, []int64{productID})
}

func (r *merchantCatalogRepository) ListSKUsByProductIDs(ctx context.Context, merchantID int64, productIDs []int64) ([]model.ProductSKU, error) {
	if len(productIDs) == 0 {
		return []model.ProductSKU{}, nil
	}
	var skus []model.ProductSKU
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND product_id IN ?", merchantID, productIDs).
		Order("product_id ASC, id ASC").
		Find(&skus).Error
	return skus, err
}

func (r *merchantCatalogRepository) FindSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) (*model.ProductSKU, error) {
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).
		Where("id = ? AND product_id = ? AND merchant_id = ?", skuID, productID, merchantID).
		First(&sku).Error; err != nil {
		return nil, err
	}
	return &sku, nil
}

func (r *merchantCatalogRepository) FindSKUForUpdate(
	ctx context.Context,
	merchantID int64,
	productID int64,
	skuID int64,
) (*model.ProductSKU, error) {
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND product_id = ? AND merchant_id = ?", skuID, productID, merchantID).
		First(&sku).Error; err != nil {
		return nil, err
	}
	return &sku, nil
}

func (r *merchantCatalogRepository) CreateSKU(ctx context.Context, sku *model.ProductSKU) error {
	return r.db.WithContext(ctx).Create(sku).Error
}

func (r *merchantCatalogRepository) UpdateSKU(ctx context.Context, sku *model.ProductSKU) error {
	return r.db.WithContext(ctx).Model(&model.ProductSKU{}).
		Where("id = ? AND product_id = ? AND merchant_id = ?", sku.ID, sku.ProductID, sku.MerchantID).
		Updates(map[string]interface{}{
			"name":                sku.Name,
			"image":               sku.Image,
			"price":               sku.Price,
			"stock":               sku.Stock,
			"low_stock_threshold": sku.LowStockThreshold,
			"status":              sku.Status,
		}).Error
}

func (r *merchantCatalogRepository) CreateInventoryLog(ctx context.Context, log *model.InventoryLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *merchantCatalogRepository) DeleteSKU(ctx context.Context, merchantID int64, productID int64, skuID int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND product_id = ? AND merchant_id = ?", skuID, productID, merchantID).
		Delete(&model.ProductSKU{})
	return affectedOrNotFound(result, "SKU 不存在")
}

func (r *merchantCatalogRepository) DeleteSKUsByProductID(ctx context.Context, merchantID int64, productID int64) error {
	return r.db.WithContext(ctx).
		Where("merchant_id = ? AND product_id = ?", merchantID, productID).
		Delete(&model.ProductSKU{}).Error
}

func (r *merchantCatalogRepository) CountEnabledSKUs(ctx context.Context, merchantID int64, productID int64, excludeSKUID int64) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.ProductSKU{}).
		Where("merchant_id = ? AND product_id = ? AND status = ? AND price > 0", merchantID, productID, model.StatusEnabled)
	if excludeSKUID > 0 {
		query = query.Where("id <> ?", excludeSKUID)
	}
	err := query.Count(&count).Error
	return count, err
}

func affectedOrNotFound(result *gorm.DB, message string) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New(message)
	}
	return nil
}
