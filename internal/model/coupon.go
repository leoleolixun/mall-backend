package model

import "time"

const (
	CouponStatusDraft    = 0
	CouponStatusActive   = 1
	CouponStatusDisabled = 2
)

type Coupon struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	MerchantID      int64     `gorm:"not null;index" json:"merchant_id"`
	Name            string    `gorm:"type:varchar(100);not null" json:"name"`
	ThresholdAmount int64     `gorm:"not null;default:0" json:"threshold_amount"`
	DiscountAmount  int64     `gorm:"not null" json:"discount_amount"`
	TotalQuantity   int       `gorm:"not null" json:"total_quantity"`
	ClaimedQuantity int       `gorm:"not null;default:0" json:"claimed_quantity"`
	UsedQuantity    int       `gorm:"not null;default:0" json:"used_quantity"`
	PerUserLimit    int       `gorm:"not null;default:1" json:"per_user_limit"`
	Status          int       `gorm:"not null;default:0;index" json:"status"`
	StartAt         time.Time `gorm:"not null;index" json:"start_at"`
	EndAt           time.Time `gorm:"not null;index" json:"end_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

const (
	UserCouponStatusUnused  = 1
	UserCouponStatusUsed    = 2
	UserCouponStatusExpired = 3
)

type UserCoupon struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	CouponID   int64      `gorm:"not null;index" json:"coupon_id"`
	UserID     int64      `gorm:"not null;index;index:idx_user_coupons_user_status,priority:1" json:"user_id"`
	MerchantID int64      `gorm:"not null;index" json:"merchant_id"`
	Status     int        `gorm:"not null;default:1;index:idx_user_coupons_user_status,priority:2" json:"status"`
	OrderID    int64      `gorm:"index" json:"order_id"`
	ClaimedAt  time.Time  `gorm:"not null" json:"claimed_at"`
	UsedAt     *time.Time `json:"used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
