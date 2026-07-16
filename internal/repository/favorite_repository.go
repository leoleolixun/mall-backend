package repository

import (
	"context"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FavoriteRepository interface {
	FindOnSaleProduct(ctx context.Context, productID int64) (*model.Product, error)
	Create(ctx context.Context, favorite *model.FavoriteProduct) error
	Delete(ctx context.Context, userID, productID int64) error
	ListProducts(ctx context.Context, userID int64, offset, limit int) ([]model.Product, map[int64]int64, int64, error)
}

type favoriteRepository struct {
	db *gorm.DB
}

func NewFavoriteRepository(db *gorm.DB) FavoriteRepository {
	return &favoriteRepository{db: db}
}

func (r *favoriteRepository) FindOnSaleProduct(ctx context.Context, productID int64) (*model.Product, error) {
	var product model.Product
	err := r.db.WithContext(ctx).Model(&model.Product{}).
		Joins("JOIN merchants ON merchants.id = products.merchant_id AND merchants.status = ?", model.StatusEnabled).
		Where("products.id = ? AND products.status = ?", productID, model.ProductStatusOnSale).
		Select("products.*").
		First(&product).Error
	return &product, err
}

func (r *favoriteRepository) Create(ctx context.Context, favorite *model.FavoriteProduct) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "merchant_id"}, {Name: "product_id"}},
			DoNothing: true,
		}).
		Create(favorite).Error
}

func (r *favoriteRepository) Delete(ctx context.Context, userID, productID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.FavoriteProduct{}).Error
}

func (r *favoriteRepository) ListProducts(ctx context.Context, userID int64, offset, limit int) ([]model.Product, map[int64]int64, int64, error) {
	base := r.db.WithContext(ctx).
		Table("favorite_products AS favorites").
		Joins("JOIN merchants ON merchants.id = favorites.merchant_id AND merchants.status = ?", model.StatusEnabled).
		Joins("JOIN products ON products.id = favorites.product_id AND products.merchant_id = favorites.merchant_id").
		Where("favorites.user_id = ? AND products.status = ? AND products.deleted_at IS NULL", userID, model.ProductStatusOnSale)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	var products []model.Product
	if err := base.Select("products.*").Order("favorites.id DESC").Offset(offset).Limit(limit).Scan(&products).Error; err != nil {
		return nil, nil, 0, err
	}

	prices := make(map[int64]int64, len(products))
	if len(products) == 0 {
		return products, prices, total, nil
	}

	productIDs := make([]int64, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}
	type priceRow struct {
		ProductID int64 `gorm:"column:product_id"`
		MinPrice  int64 `gorm:"column:min_price"`
	}
	var rows []priceRow
	if err := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Select("product_id, MIN(price) AS min_price").
		Where("product_id IN ? AND status = ?", productIDs, model.StatusEnabled).
		Group("product_id").
		Scan(&rows).Error; err != nil {
		return nil, nil, 0, err
	}
	for _, row := range rows {
		prices[row.ProductID] = row.MinPrice
	}
	return products, prices, total, nil
}
