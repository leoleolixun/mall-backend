package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PayChannelMock   = "mock" // 模拟支付渠道
	PayChannelWechat = "wechat"
	PayChannelAlipay = "alipay"
)

const (
	PaySceneMock       = "mock"
	PaySceneAlipayPage = "page"
	PaySceneAlipayWap  = "wap"
)

const (
	PaymentStatusPending           = 1 // 待支付
	PaymentStatusPaid              = 2 // 已支付
	PaymentStatusClosed            = 3 // 已关闭
	PaymentStatusFailed            = 4 // 支付失败
	PaymentStatusRefunded          = 5 // 已退款
	PaymentStatusPartiallyRefunded = 6 // 部分退款
)

type Payment struct {
	ID            int64  `gorm:"primaryKey" json:"id"`
	PaymentNo     string `gorm:"type:varchar(64);not null;uniqueIndex" json:"payment_no"`
	TradeID       *int64 `gorm:"column:trade_id;-:migration" json:"trade_id,omitempty"`
	ActiveTradeID *int64 `gorm:"column:active_trade_id;-:migration" json:"-"`
	OrderID       *int64 `gorm:"index;index:idx_payments_order_status,priority:1" json:"order_id,omitempty"`
	// ActiveOrderID is populated only while the payment is pending. MySQL permits multiple NULLs in a unique index.
	ActiveOrderID *int64 `gorm:"uniqueIndex:idx_payments_active_order" json:"-"`
	OrderNo       string `gorm:"type:varchar(64);index" json:"order_no,omitempty"`
	UserID        int64  `gorm:"not null;index" json:"user_id"`
	MerchantID    *int64 `gorm:"index" json:"merchant_id,omitempty"`
	PayChannel    string `gorm:"type:varchar(32);not null;index" json:"pay_channel"`
	PayScene      string `gorm:"type:varchar(32);not null;default:'';index" json:"pay_scene"`
	Status        int    `gorm:"not null;default:1;index;index:idx_payments_order_status,priority:2" json:"status"`
	Amount        int64  `gorm:"not null;default:0" json:"amount"`

	TransactionID string `gorm:"type:varchar(128);index" json:"transaction_id"`
	FailureReason string `gorm:"type:varchar(255)" json:"failure_reason"`

	PaidAt    *time.Time     `json:"paid_at"`
	ClosedAt  *time.Time     `json:"closed_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
