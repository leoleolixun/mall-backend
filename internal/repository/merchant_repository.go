package repository

import (
	"context"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type MerchantRepository interface {
	ListEnabled(ctx context.Context, offset, limit int) ([]model.Merchant, int64, error)
	FindEnabledByID(ctx context.Context, id int64) (*model.Merchant, error)
	FindByIDs(ctx context.Context, ids []int64) ([]model.Merchant, error)
}

type merchantRepository struct {
	db *gorm.DB
}

func NewMerchantRepository(db *gorm.DB) MerchantRepository {
	return &merchantRepository{db: db}
}

func (r *merchantRepository) ListEnabled(ctx context.Context, offset, limit int) ([]model.Merchant, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Merchant{}).Where("status = ?", model.StatusEnabled)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var merchants []model.Merchant
	if err := query.Order("id ASC").Offset(offset).Limit(limit).Find(&merchants).Error; err != nil {
		return nil, 0, err
	}
	return merchants, total, nil
}

func (r *merchantRepository) FindEnabledByID(ctx context.Context, id int64) (*model.Merchant, error) {
	var merchant model.Merchant
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, model.StatusEnabled).
		First(&merchant).Error
	if err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantRepository) FindByIDs(ctx context.Context, ids []int64) ([]model.Merchant, error) {
	if len(ids) == 0 {
		return []model.Merchant{}, nil
	}
	var merchants []model.Merchant
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&merchants).Error
	return merchants, err
}
