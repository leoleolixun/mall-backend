package model

import (
	"time"

	"gorm.io/gorm"
)

type ProductSKU struct {
	ID                int64          `gorm:"primaryKey" json:"id"`
	MerchantID        int64          `gorm:"not null;index" json:"merchant_id"`
	ProductID         int64          `gorm:"not null;index" json:"product_id"`
	Name              string         `gorm:"type:varchar(200);not null" json:"name"`
	Image             string         `gorm:"type:varchar(255)" json:"image"`
	Price             int64          `gorm:"not null;default:0" json:"price"`
	Stock             int            `gorm:"not null;default:0" json:"stock"`
	LowStockThreshold int            `gorm:"not null;default:0;index" json:"low_stock_threshold"`
	Status            int            `gorm:"not null;default:1;index" json:"status"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}
