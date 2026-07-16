package model

import "time"

const (
	TradeStatusPendingPayment    = 1
	TradeStatusPaid              = 2
	TradeStatusClosed            = 3
	TradeStatusPartiallyRefunded = 4
	TradeStatusRefunded          = 5
)

type Trade struct {
	ID             int64      `gorm:"primaryKey" json:"id"`
	TradeNo        string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"trade_no"`
	UserID         int64      `gorm:"not null;index" json:"user_id"`
	Status         int        `gorm:"not null;default:1;index" json:"status"`
	GoodsAmount    int64      `gorm:"not null;default:0" json:"goods_amount"`
	FreightAmount  int64      `gorm:"not null;default:0" json:"freight_amount"`
	DiscountAmount int64      `gorm:"not null;default:0" json:"discount_amount"`
	PayableAmount  int64      `gorm:"not null;default:0" json:"payable_amount"`
	IdempotencyKey string     `gorm:"type:varchar(64);not null" json:"-"`
	PaidAt         *time.Time `json:"paid_at"`
	ClosedAt       *time.Time `json:"closed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PaymentAllocation struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	PaymentID      int64     `gorm:"not null;index" json:"payment_id"`
	TradeID        int64     `gorm:"not null;index" json:"trade_id"`
	OrderID        int64     `gorm:"not null;index" json:"order_id"`
	MerchantID     int64     `gorm:"not null;index" json:"merchant_id"`
	Amount         int64     `gorm:"not null" json:"amount"`
	RefundedAmount int64     `gorm:"not null;default:0" json:"refunded_amount"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

const (
	SettlementStatusPending   = 1
	SettlementStatusConfirmed = 2
	SettlementStatusPaid      = 3
)

const (
	SettlementEntrySale             = "sale"
	SettlementEntryCommission       = "commission"
	SettlementEntryRefund           = "refund"
	SettlementEntryCommissionRefund = "commission_refund"
	SettlementEntryAdjustment       = "adjustment"
)

type MerchantSettlement struct {
	ID               int64      `gorm:"primaryKey" json:"id"`
	SettlementNo     string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"settlement_no"`
	MerchantID       int64      `gorm:"not null;index" json:"merchant_id"`
	PeriodStart      time.Time  `gorm:"not null" json:"period_start"`
	PeriodEnd        time.Time  `gorm:"not null" json:"period_end"`
	GrossAmount      int64      `gorm:"not null;default:0" json:"gross_amount"`
	CommissionAmount int64      `gorm:"not null;default:0" json:"commission_amount"`
	RefundAmount     int64      `gorm:"not null;default:0" json:"refund_amount"`
	NetAmount        int64      `gorm:"not null;default:0" json:"net_amount"`
	Status           int        `gorm:"not null;default:1;index" json:"status"`
	ConfirmedAt      *time.Time `json:"confirmed_at"`
	PaidAt           *time.Time `json:"paid_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type SettlementEntry struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	EntryNo      string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"entry_no"`
	MerchantID   int64     `gorm:"not null;index" json:"merchant_id"`
	OrderID      *int64    `gorm:"index" json:"order_id,omitempty"`
	RefundID     *int64    `gorm:"index" json:"refund_id,omitempty"`
	EntryType    string    `gorm:"type:varchar(32);not null" json:"entry_type"`
	Amount       int64     `gorm:"not null" json:"amount"`
	AvailableAt  time.Time `gorm:"not null;index" json:"available_at"`
	SettlementID *int64    `gorm:"index" json:"settlement_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
