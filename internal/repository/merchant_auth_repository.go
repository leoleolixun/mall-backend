package repository

import (
	"context"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type MerchantAuthRepository interface {
	Create(ctx context.Context, account *model.MerchantAccount) error
	FindByUsername(ctx context.Context, username string) (*model.MerchantAccount, error)
	FindByID(ctx context.Context, accountID int64) (*model.MerchantAccount, error)
	FindByIDAndMerchantID(ctx context.Context, accountID int64, merchantID int64) (*model.MerchantAccount, error)
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	UpdateLastLoginAt(ctx context.Context, accountID int64, lastLoginAt time.Time) error
}

type merchantAuthRepository struct {
	db *gorm.DB
}

func NewMerchantAuthRepository(db *gorm.DB) MerchantAuthRepository {
	return &merchantAuthRepository{db: db}
}

func (r *merchantAuthRepository) Create(ctx context.Context, account *model.MerchantAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *merchantAuthRepository) FindByUsername(ctx context.Context, username string) (*model.MerchantAccount, error) {
	var account model.MerchantAccount
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *merchantAuthRepository) FindByID(ctx context.Context, accountID int64) (*model.MerchantAccount, error) {
	var account model.MerchantAccount
	if err := r.db.WithContext(ctx).Where("id = ?", accountID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *merchantAuthRepository) FindByIDAndMerchantID(ctx context.Context, accountID int64, merchantID int64) (*model.MerchantAccount, error) {
	var account model.MerchantAccount
	if err := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", accountID, merchantID).
		First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *merchantAuthRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantAuthRepository) UpdateLastLoginAt(ctx context.Context, accountID int64, lastLoginAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.MerchantAccount{}).
		Where("id = ?", accountID).
		Update("last_login_at", lastLoginAt).Error
}
