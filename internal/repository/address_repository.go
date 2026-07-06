package repository

import (
	"context"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

type AddressRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]model.Address, error)
	FindByIDAndUserID(ctx context.Context, id int64, userID int64) (*model.Address, error)
	Create(ctx context.Context, address *model.Address) error
	Update(ctx context.Context, address *model.Address) error
	DeleteByIDAndUserID(ctx context.Context, id int64, userID int64) error
	ClearDefault(ctx context.Context, userID int64) error
	Transaction(ctx context.Context, fn func(repo AddressRepository) error) error
}

type addressRepository struct {
	db *gorm.DB
}

func NewAddressRepository(db *gorm.DB) AddressRepository {
	return &addressRepository{
		db: db,
	}
}

func (r *addressRepository) ListByUserID(ctx context.Context, userID int64) ([]model.Address, error) {
	var addresses []model.Address
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&addresses).Error
	return addresses, err
}

func (r *addressRepository) FindByIDAndUserID(ctx context.Context, id int64, userID int64) (*model.Address, error) {
	var address model.Address
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&address).Error
	if err != nil {
		return nil, err
	}
	return &address, nil
}

func (r *addressRepository) Create(ctx context.Context, address *model.Address) error {
	return r.db.WithContext(ctx).
		Create(address).Error
}

func (r *addressRepository) Update(ctx context.Context, address *model.Address) error {
	return r.db.WithContext(ctx).
		Save(address).Error
}

func (r *addressRepository) DeleteByIDAndUserID(ctx context.Context, id int64, userID int64) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Address{}).Error
}

func (r *addressRepository) ClearDefault(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Address{}).
		Where("user_id = ?", userID).
		Update("is_default", false).Error
}

func (r *addressRepository) Transaction(ctx context.Context, fn func(repo AddressRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&addressRepository{db: tx})
	})
}
