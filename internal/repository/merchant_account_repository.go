package repository

import (
	"context"
	"strings"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchantAccountRepository interface {
	Transaction(ctx context.Context, fn func(repo MerchantAccountRepository) error) error
	List(ctx context.Context, merchantID int64, offset int, limit int, role string, status *int, keyword string) ([]model.MerchantAccount, int64, error)
	FindForUpdate(ctx context.Context, merchantID int64, accountID int64) (*model.MerchantAccount, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	Create(ctx context.Context, account *model.MerchantAccount) error
	Update(ctx context.Context, account *model.MerchantAccount) error
	UpdatePassword(ctx context.Context, merchantID int64, accountID int64, passwordHash string) error
	CountEnabledOwnersForUpdate(ctx context.Context, merchantID int64) (int64, error)
	CountByRole(ctx context.Context, merchantID int64) (map[string]int64, error)
}

type merchantAccountRepository struct {
	db *gorm.DB
}

func NewMerchantAccountRepository(db *gorm.DB) MerchantAccountRepository {
	return &merchantAccountRepository{db: db}
}

func (r *merchantAccountRepository) Transaction(ctx context.Context, fn func(repo MerchantAccountRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&merchantAccountRepository{db: tx})
	})
}

func (r *merchantAccountRepository) List(
	ctx context.Context,
	merchantID int64,
	offset int,
	limit int,
	role string,
	status *int,
	keyword string,
) ([]model.MerchantAccount, int64, error) {
	var accounts []model.MerchantAccount
	var total int64
	query := r.db.WithContext(ctx).Model(&model.MerchantAccount{}).Where("merchant_id = ?", merchantID)
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&accounts).Error; err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}

func (r *merchantAccountRepository) FindForUpdate(ctx context.Context, merchantID int64, accountID int64) (*model.MerchantAccount, error) {
	var account model.MerchantAccount
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND merchant_id = ?", accountID, merchantID).
		First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *merchantAccountRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.MerchantAccount{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (r *merchantAccountRepository) Create(ctx context.Context, account *model.MerchantAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *merchantAccountRepository) Update(ctx context.Context, account *model.MerchantAccount) error {
	return r.db.WithContext(ctx).Model(&model.MerchantAccount{}).
		Where("id = ? AND merchant_id = ?", account.ID, account.MerchantID).
		Updates(map[string]interface{}{
			"nickname": account.Nickname,
			"role":     account.Role,
			"status":   account.Status,
		}).Error
}

func (r *merchantAccountRepository) UpdatePassword(ctx context.Context, merchantID int64, accountID int64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.MerchantAccount{}).
		Where("id = ? AND merchant_id = ?", accountID, merchantID).
		Update("password_hash", passwordHash).Error
}

func (r *merchantAccountRepository) CountEnabledOwnersForUpdate(ctx context.Context, merchantID int64) (int64, error) {
	var accounts []model.MerchantAccount
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("merchant_id = ? AND role = ? AND status = ?", merchantID, model.MerchantRoleOwner, model.StatusEnabled).
		Find(&accounts).Error; err != nil {
		return 0, err
	}
	return int64(len(accounts)), nil
}

func (r *merchantAccountRepository) CountByRole(ctx context.Context, merchantID int64) (map[string]int64, error) {
	type roleCount struct {
		Role  string
		Count int64
	}
	var rows []roleCount
	if err := r.db.WithContext(ctx).Model(&model.MerchantAccount{}).
		Select("role, COUNT(*) AS count").
		Where("merchant_id = ?", merchantID).
		Group("role").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Role] = row.Count
	}
	return counts, nil
}
