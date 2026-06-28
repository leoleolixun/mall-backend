package model

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	MerchantID int64          `gorm:"not null;index" json:"merchant_id"`
	ParentID   int64          `gorm:"not null;default:0;index" json:"parent_id"`
	Name       string         `gorm:"type:varchar(100);not null" json:"name"`
	Sort       int            `gorm:"not null;default:0" json:"sort"`
	Status     int            `gorm:"not null;default:1;index" json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
