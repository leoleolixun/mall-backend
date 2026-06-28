package model

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID          int64          `gorm:"primaryKey" json:"id"`
	MerchantID  int64          `gorm:"not null;index" json:"merchant_id"`
	CategoryID  int64          `gorm:"not null;index" json:"category_id"`
	Name        string         `gorm:"type:varchar(200);not null;index" json:"name"`
	Cover       string         `gorm:"type:varchar(255)" json:"cover"`
	Description string         `gorm:"type:text" json:"description"`
	Status      int            `gorm:"not null;default:0;index" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
