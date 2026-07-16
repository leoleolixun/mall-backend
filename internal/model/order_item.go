package model

import "time"

type OrderItem struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	OrderID        int64     `gorm:"not null;index" json:"order_id"`
	ProductID      int64     `gorm:"not null;index" json:"product_id"`
	SKUID          int64     `gorm:"not null;index" json:"sku_id"`
	ProductName    string    `gorm:"type:varchar(200);not null" json:"product_name"`
	SKUName        string    `gorm:"type:varchar(200);not null" json:"sku_name"`
	SKUImage       string    `gorm:"type:varchar(255)" json:"sku_image"`
	Price          int64     `gorm:"not null;default:0" json:"price"`
	Quantity       int       `gorm:"not null;default:0" json:"quantity"`
	Subtotal       int64     `gorm:"not null;default:0" json:"subtotal"`
	DiscountAmount int64     `gorm:"not null;default:0" json:"discount_amount"`
	PayableAmount  int64     `gorm:"not null;default:0" json:"payable_amount"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
