package repository

import (
	"context"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	CreateUserAuth(ctx context.Context, auth *model.UserAuth) error
	FindAuthByProvider(ctx context.Context, provider string, providerUID string) (*model.UserAuth, error)
	FindUserByID(ctx context.Context, userID int64) (*model.User, error)
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *authRepository) CreateUserAuth(ctx context.Context, auth *model.UserAuth) error {
	return r.db.WithContext(ctx).Create(auth).Error
}

func (r *authRepository) FindAuthByProvider(ctx context.Context, provider string, providerUID string) (*model.UserAuth, error) {
	var auth model.UserAuth
	err := r.db.WithContext(ctx).Where("provider = ? AND provider_uid = ?", provider, providerUID).First(&auth).Error
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

func (r *authRepository) FindUserByID(ctx context.Context, userID int64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id =? AND status =?", userID, model.StatusEnabled).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
