package repository

import (
	"context"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

// 声明 ProductRepository 接口，定义了商品相关的数据库操作方法
type ProductRepository interface {
	ListOnSale(ctx context.Context, merchantID int64, categoryID int64, keyword string, offset int, limit int) ([]model.Product, int64, error)
	FindOnSaleByID(ctx context.Context, merchantID int64, id int64) (*model.Product, error)
	ListEnabledSKUs(ctx context.Context, merchantID int64, productID int64) ([]model.ProductSKU, error)
	FindMinPrices(ctx context.Context, merchantID int64, productIDs []int64) (map[int64]int64, error)
	FindSKUsByIDs(ctx context.Context, merchantID int64, skuIDs []int64) ([]model.ProductSKU, error)
	FindProductsByIDs(ctx context.Context, merchantID int64, productIDs []int64) ([]model.Product, error)
}

// ProductRepository 接口的具体实现，封装了对商品相关数据的操作
type productRepository struct {
	db *gorm.DB
}

// 创建一个新的ProductRepository实例
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{
		db: db,
	}
}

// 查询在售商品列表，支持按分类和关键字搜索，并返回总数
func (r *productRepository) ListOnSale(ctx context.Context, merchantID int64, categoryID int64, keyword string, offset int, limit int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Product{}).
		Joins("JOIN merchants ON merchants.id = products.merchant_id AND merchants.status = ?", model.StatusEnabled).
		Where("products.status = ?", model.ProductStatusOnSale)

	if merchantID > 0 {
		query = query.Where("products.merchant_id = ?", merchantID)
	}

	if categoryID > 0 {
		query = query.Where("products.category_id = ?", categoryID)
	}

	if keyword != "" {
		query = query.Where("products.name LIKE ?", "%"+keyword+"%")
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Select("products.*").Order("products.id DESC").Offset(offset).Limit(limit).Find(&products).Error
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// 根据商品ID查询在售商品详情
func (r *productRepository) FindOnSaleByID(ctx context.Context, merchantID int64, id int64) (*model.Product, error) {
	var product model.Product
	query := r.db.WithContext(ctx).Model(&model.Product{}).
		Joins("JOIN merchants ON merchants.id = products.merchant_id AND merchants.status = ?", model.StatusEnabled).
		Where("products.id = ? AND products.status = ?", id, model.ProductStatusOnSale)
	if merchantID > 0 {
		query = query.Where("products.merchant_id = ?", merchantID)
	}
	err := query.Select("products.*").
		First(&product).Error
	if err != nil {
		return nil, err
	}

	return &product, nil
}

// 查询指定商品的在售SKU列表
func (r *productRepository) ListEnabledSKUs(ctx context.Context, merchantID int64, productID int64) ([]model.ProductSKU, error) {
	var skus []model.ProductSKU
	query := r.db.WithContext(ctx).
		Where("product_id = ? AND status = ?", productID, model.StatusEnabled)
	if merchantID > 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	err := query.
		Order("id ASC").
		Find(&skus).Error
	if err != nil {
		return nil, err
	}

	return skus, nil
}

// FindMinPrices 查询指定商品ID列表的最小价格，返回一个映射，键为商品ID，值为最小价格
func (r *productRepository) FindMinPrices(ctx context.Context, merchantID int64, productIDs []int64) (map[int64]int64, error) {
	if len(productIDs) == 0 {
		return map[int64]int64{}, nil
	}

	type Result struct {
		ProductID int64 `gorm:"column:product_id"`
		MinPrice  int64 `gorm:"column:min_price"`
	}

	var results []Result
	query := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Select("product_id, MIN(price) as min_price").
		Where("product_id IN ? AND status = ?", productIDs, model.StatusEnabled)
	if merchantID > 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	err := query.
		Group("product_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	minPrices := make(map[int64]int64)
	for _, result := range results {
		minPrices[result.ProductID] = result.MinPrice
	}

	return minPrices, nil
}

func (r *productRepository) FindSKUsByIDs(ctx context.Context, merchantID int64, skuIDs []int64) ([]model.ProductSKU, error) {
	if len(skuIDs) == 0 {
		return []model.ProductSKU{}, nil
	}

	var skus []model.ProductSKU
	query := r.db.WithContext(ctx).Where("id IN ?", skuIDs)
	if merchantID > 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	err := query.
		Find(&skus).Error
	return skus, err
}

func (r *productRepository) FindProductsByIDs(ctx context.Context, merchantID int64, productIDs []int64) ([]model.Product, error) {
	if len(productIDs) == 0 {
		return []model.Product{}, nil
	}

	var products []model.Product
	query := r.db.WithContext(ctx).Where("id IN ?", productIDs)
	if merchantID > 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	err := query.
		Find(&products).Error
	return products, err
}
