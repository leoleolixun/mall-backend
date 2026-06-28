package repository

import (
	"context"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

type CategoryRepository interface {
	ListEnabled(ctx context.Context, merchantID int64) ([]model.Category, error)
}

type categoryRepository struct {
	db *gorm.DB
}

// 创建一个 CategoryRepository 实例
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{
		db: db,
	}
}

func (r *categoryRepository) ListEnabled(ctx context.Context, merchantID int64) ([]model.Category, error) {
	var categories []model.Category

	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND status = ?", merchantID, model.StatusEnabled).
		Order("sort DESC, id ASC").
		Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}
