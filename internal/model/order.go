package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	OrderStatusPendingPayment = 1 // 待付款
	OrderStatusPaid           = 2 // 已付款
	OrderStatusShipped        = 3 // 已发货
	OrderStatusCompleted      = 4 // 已完成
	OrderStatusCancelled      = 5 // 已取消
)

type Order struct {
	ID         int64  `gorm:"primaryKey" json:"id"`
	OrderNo    string `gorm:"type:varchar(64);not null;uniqueIndex" json:"order_no"`
	UserID     int64  `gorm:"not null;index;index:idx_orders_merchant_status_user_paid_at,priority:3" json:"user_id"`
	MerchantID int64  `gorm:"not null;index;index:idx_orders_merchant_status_user_paid_at,priority:1" json:"merchant_id"`
	Status     int    `gorm:"not null;default:1;index;index:idx_orders_status_created_at,priority:1;index:idx_orders_merchant_status_user_paid_at,priority:2" json:"status"`

	ReceiverName    string `gorm:"type:varchar(100);not null" json:"receiver_name"`
	ReceiverPhone   string `gorm:"type:varchar(20);not null" json:"receiver_phone"`
	ReceiverAddress string `gorm:"type:varchar(500);not null" json:"receiver_address"`

	GoodsAmount    int64 `gorm:"not null;default:0" json:"goods_amount"`
	FreightAmount  int64 `gorm:"not null;default:0" json:"freight_amount"`
	DiscountAmount int64 `gorm:"not null;default:0" json:"discount_amount"`
	PayableAmount  int64 `gorm:"not null;default:0" json:"payable_amount"`
	UserCouponID   int64 `gorm:"index" json:"user_coupon_id"`

	Remark      string         `gorm:"type:varchar(255)" json:"remark"`
	PaidAt      *time.Time     `gorm:"index:idx_orders_merchant_status_user_paid_at,priority:4" json:"paid_at"`
	CancelledAt *time.Time     `json:"cancelled_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	CreatedAt   time.Time      `gorm:"index:idx_orders_status_created_at,priority:2" json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
