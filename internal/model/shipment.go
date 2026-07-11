package model

import "time"

const (
	DeliveryTypeExpress      = "express"
	DeliveryTypeSelfDelivery = "self_delivery"
)

type Shipment struct {
	ID               int64     `gorm:"primaryKey" json:"id"`
	OrderID          int64     `gorm:"not null;uniqueIndex" json:"order_id"`
	MerchantID       int64     `gorm:"not null;index" json:"merchant_id"`
	DeliveryType     string    `gorm:"type:varchar(32);not null;default:express;index" json:"delivery_type"`
	LogisticsCompany string    `gorm:"type:varchar(100);not null" json:"logistics_company"`
	TrackingNo       string    `gorm:"type:varchar(100);not null;index" json:"tracking_no"`
	ShippedAt        time.Time `gorm:"not null" json:"shipped_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Order    Order    `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
	Merchant Merchant `gorm:"foreignKey:MerchantID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
}
