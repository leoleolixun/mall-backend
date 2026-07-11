package repository

import (
	"context"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchantInventoryRepository interface {
	Transaction(ctx context.Context, fn func(repo MerchantInventoryRepository) error) error
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	FindSKUForUpdate(ctx context.Context, merchantID int64, skuID int64) (*model.ProductSKU, error)
	FindProductByID(ctx context.Context, merchantID int64, productID int64) (*model.Product, error)
	UpdateSKUStock(ctx context.Context, sku *model.ProductSKU) error
	CreateInventoryLog(ctx context.Context, log *model.InventoryLog) error
	List(
		ctx context.Context,
		merchantID int64,
		productID int64,
		skuID int64,
		changeType string,
		offset int,
		limit int,
	) ([]model.InventoryLog, int64, error)
	ListAlerts(
		ctx context.Context,
		merchantID int64,
		productID int64,
		skuID int64,
		keyword string,
		offset int,
		limit int,
	) ([]MerchantInventoryAlertRecord, int64, error)
}

type MerchantInventoryAlertRecord struct {
	MerchantID        int64
	ProductID         int64
	SKUID             int64
	ProductName       string
	SKUName           string
	Image             string
	Stock             int
	LowStockThreshold int
	UpdatedAt         time.Time
}

type merchantInventoryRepository struct {
	db *gorm.DB
}

func NewMerchantInventoryRepository(db *gorm.DB) MerchantInventoryRepository {
	return &merchantInventoryRepository{db: db}
}

func (r *merchantInventoryRepository) Transaction(ctx context.Context, fn func(repo MerchantInventoryRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&merchantInventoryRepository{db: tx})
	})
}

func (r *merchantInventoryRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantInventoryRepository) FindSKUForUpdate(ctx context.Context, merchantID int64, skuID int64) (*model.ProductSKU, error) {
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND merchant_id = ?", skuID, merchantID).
		First(&sku).Error; err != nil {
		return nil, err
	}
	return &sku, nil
}

func (r *merchantInventoryRepository) FindProductByID(ctx context.Context, merchantID int64, productID int64) (*model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", productID, merchantID).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *merchantInventoryRepository) UpdateSKUStock(ctx context.Context, sku *model.ProductSKU) error {
	result := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Where("id = ? AND merchant_id = ?", sku.ID, sku.MerchantID).
		Updates(map[string]interface{}{
			"stock":               sku.Stock,
			"low_stock_threshold": sku.LowStockThreshold,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *merchantInventoryRepository) CreateInventoryLog(ctx context.Context, log *model.InventoryLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *merchantInventoryRepository) List(
	ctx context.Context,
	merchantID int64,
	productID int64,
	skuID int64,
	changeType string,
	offset int,
	limit int,
) ([]model.InventoryLog, int64, error) {
	var logs []model.InventoryLog
	var total int64
	query := r.db.WithContext(ctx).Model(&model.InventoryLog{}).Where("merchant_id = ?", merchantID)
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}
	if changeType != "" {
		query = query.Where("change_type = ?", changeType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *merchantInventoryRepository) ListAlerts(
	ctx context.Context,
	merchantID int64,
	productID int64,
	skuID int64,
	keyword string,
	offset int,
	limit int,
) ([]MerchantInventoryAlertRecord, int64, error) {
	var alerts []MerchantInventoryAlertRecord
	var total int64
	query := r.db.WithContext(ctx).
		Table("product_skus AS skus").
		Joins("JOIN products ON products.id = skus.product_id AND products.merchant_id = skus.merchant_id AND products.deleted_at IS NULL").
		Where(
			"skus.merchant_id = ? AND skus.deleted_at IS NULL AND skus.status = ? AND skus.low_stock_threshold > 0 AND skus.stock <= skus.low_stock_threshold",
			merchantID,
			model.StatusEnabled,
		)
	if productID > 0 {
		query = query.Where("skus.product_id = ?", productID)
	}
	if skuID > 0 {
		query = query.Where("skus.id = ?", skuID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("products.name LIKE ? OR skus.name LIKE ?", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Select("skus.merchant_id, skus.product_id, skus.id AS sku_id, products.name AS product_name, skus.name AS sku_name, skus.image, skus.stock, skus.low_stock_threshold, skus.updated_at").
		Order("skus.stock ASC, skus.id DESC").
		Offset(offset).
		Limit(limit).
		Scan(&alerts).Error; err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}
