package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CouponRepository interface {
	Transaction(ctx context.Context, fn func(repo CouponRepository) error) error
	Create(ctx context.Context, coupon *model.Coupon) error
	Update(ctx context.Context, coupon *model.Coupon) error
	FindByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.Coupon, error)
	FindForUpdate(ctx context.Context, id int64) (*model.Coupon, error)
	IsMerchantEnabled(ctx context.Context, merchantID int64) (bool, error)
	ListByMerchantID(ctx context.Context, merchantID int64, offset, limit, status int) ([]model.Coupon, int64, error)
	ListAvailable(ctx context.Context, merchantID, userID int64, now time.Time) ([]model.Coupon, map[int64]int64, error)
	CountClaimedByUser(ctx context.Context, couponID, userID int64) (int64, error)
	IncrementClaimed(ctx context.Context, couponID int64) error
	CreateUserCoupon(ctx context.Context, userCoupon *model.UserCoupon) error
	ListUserCoupons(ctx context.Context, userID int64, status int) ([]model.UserCoupon, error)
	ExpireUserCoupons(ctx context.Context, userID int64, now time.Time) error
	FindCoupon(ctx context.Context, id int64) (*model.Coupon, error)
}

type couponRepository struct{ db *gorm.DB }

func NewCouponRepository(db *gorm.DB) CouponRepository { return &couponRepository{db: db} }

func (r *couponRepository) Transaction(ctx context.Context, fn func(repo CouponRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&couponRepository{db: tx}) })
}
func (r *couponRepository) Create(ctx context.Context, value *model.Coupon) error {
	return r.db.WithContext(ctx).Create(value).Error
}
func (r *couponRepository) Update(ctx context.Context, value *model.Coupon) error {
	return r.db.WithContext(ctx).Save(value).Error
}
func (r *couponRepository) FindByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.Coupon, error) {
	var value model.Coupon
	err := r.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", id, merchantID).First(&value).Error
	return &value, err
}
func (r *couponRepository) FindForUpdate(ctx context.Context, id int64) (*model.Coupon, error) {
	var value model.Coupon
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error
	return &value, err
}
func (r *couponRepository) IsMerchantEnabled(ctx context.Context, merchantID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Merchant{}).
		Where("id = ? AND status = ?", merchantID, model.StatusEnabled).
		Count(&count).Error
	return count > 0, err
}
func (r *couponRepository) ListByMerchantID(ctx context.Context, merchantID int64, offset, limit, status int) ([]model.Coupon, int64, error) {
	var values []model.Coupon
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Coupon{}).Where("merchant_id = ?", merchantID)
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&values).Error
	return values, total, err
}
func (r *couponRepository) ListAvailable(ctx context.Context, merchantID, userID int64, now time.Time) ([]model.Coupon, map[int64]int64, error) {
	var values []model.Coupon
	err := r.db.WithContext(ctx).Where("merchant_id = ? AND status = ? AND start_at <= ? AND end_at > ? AND claimed_quantity < total_quantity", merchantID, model.CouponStatusActive, now, now).Order("discount_amount DESC, id DESC").Find(&values).Error
	if err != nil {
		return nil, nil, err
	}
	counts := make(map[int64]int64)
	if len(values) == 0 {
		return values, counts, nil
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	type row struct {
		CouponID int64
		Count    int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&model.UserCoupon{}).Select("coupon_id, COUNT(*) AS count").Where("user_id = ? AND coupon_id IN ?", userID, ids).Group("coupon_id").Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	for _, value := range rows {
		counts[value.CouponID] = value.Count
	}
	return values, counts, nil
}
func (r *couponRepository) CountClaimedByUser(ctx context.Context, couponID, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.UserCoupon{}).Where("coupon_id = ? AND user_id = ?", couponID, userID).Count(&total).Error
	return total, err
}
func (r *couponRepository) IncrementClaimed(ctx context.Context, couponID int64) error {
	result := r.db.WithContext(ctx).Model(&model.Coupon{}).Where("id = ? AND claimed_quantity < total_quantity", couponID).UpdateColumn("claimed_quantity", gorm.Expr("claimed_quantity + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("优惠券已领完")
	}
	return nil
}
func (r *couponRepository) CreateUserCoupon(ctx context.Context, value *model.UserCoupon) error {
	return r.db.WithContext(ctx).Create(value).Error
}
func (r *couponRepository) ListUserCoupons(ctx context.Context, userID int64, status int) ([]model.UserCoupon, error) {
	var values []model.UserCoupon
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	err := query.Order("id DESC").Find(&values).Error
	return values, err
}
func (r *couponRepository) ExpireUserCoupons(ctx context.Context, userID int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserCoupon{}).Where("user_id = ? AND status = ? AND coupon_id IN (?)", userID, model.UserCouponStatusUnused, r.db.Model(&model.Coupon{}).Select("id").Where("end_at <= ?", now)).Update("status", model.UserCouponStatusExpired).Error
}
func (r *couponRepository) FindCoupon(ctx context.Context, id int64) (*model.Coupon, error) {
	var value model.Coupon
	err := r.db.WithContext(ctx).First(&value, id).Error
	return &value, err
}
