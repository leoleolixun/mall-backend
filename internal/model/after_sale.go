package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	AfterSaleTypeRefundOnly   = "refund_only"
	AfterSaleTypeReturnRefund = "return_refund"
)

const (
	AfterSaleStatusPending      = 1
	AfterSaleStatusRefunding    = 2
	AfterSaleStatusRefunded     = 3
	AfterSaleStatusRejected     = 4
	AfterSaleStatusCancelled    = 5
	AfterSaleStatusRefundFailed = 6
)

type AfterSale struct {
	ID           int64  `gorm:"primaryKey" json:"id"`
	AfterSaleNo  string `gorm:"type:varchar(64);not null;uniqueIndex" json:"after_sale_no"`
	OrderID      int64  `gorm:"not null;index" json:"order_id"`
	OrderItemID  int64  `gorm:"not null;index" json:"order_item_id"`
	UserID       int64  `gorm:"not null;index" json:"user_id"`
	MerchantID   int64  `gorm:"not null;index" json:"merchant_id"`
	Type         string `gorm:"type:varchar(32);not null;index" json:"type"`
	Status       int    `gorm:"not null;default:1;index" json:"status"`
	ActiveKey    string `gorm:"type:varchar(128);not null;uniqueIndex" json:"-"`
	Reason       string `gorm:"type:varchar(100);not null" json:"reason"`
	Description  string `gorm:"type:varchar(500)" json:"description"`
	Images       string `gorm:"type:text" json:"-"`
	RefundAmount int64  `gorm:"not null;default:0" json:"refund_amount"`

	RejectReason string         `gorm:"type:varchar(255)" json:"reject_reason"`
	ReviewedBy   int64          `gorm:"index" json:"reviewed_by"`
	ReviewedAt   *time.Time     `json:"reviewed_at"`
	CancelledAt  *time.Time     `json:"cancelled_at"`
	RefundedAt   *time.Time     `json:"refunded_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

const (
	RefundStatusPending   = 1
	RefundStatusSucceeded = 2
	RefundStatusFailed    = 3
)

type Refund struct {
	ID            int64      `gorm:"primaryKey" json:"id"`
	RefundNo      string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"refund_no"`
	AfterSaleID   int64      `gorm:"not null;uniqueIndex" json:"after_sale_id"`
	PaymentID     int64      `gorm:"not null;index" json:"payment_id"`
	OrderID       int64      `gorm:"not null;index" json:"order_id"`
	UserID        int64      `gorm:"not null;index" json:"user_id"`
	MerchantID    int64      `gorm:"not null;index" json:"merchant_id"`
	PayChannel    string     `gorm:"type:varchar(32);not null" json:"pay_channel"`
	Amount        int64      `gorm:"not null" json:"amount"`
	Status        int        `gorm:"not null;default:1;index" json:"status"`
	TransactionID string     `gorm:"type:varchar(128)" json:"transaction_id"`
	FailureReason string     `gorm:"type:varchar(255)" json:"failure_reason"`
	RefundedAt    *time.Time `json:"refunded_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
