package model

import (
	"time"

	"gorm.io/gorm"
)

type Address struct {
	ID            int64          `gorm:"primaryKey" json:"id"`
	UserID        int64          `gorm:"not null;index" json:"user_id"`
	ReceiverName  string         `gorm:"type:varchar(100);not null" json:"receiver_name"`
	ReceiverPhone string         `gorm:"type:varchar(20);not null" json:"receiver_phone"`
	Province      string         `gorm:"type:varchar(50);not null" json:"province"`
	City          string         `gorm:"type:varchar(100);not null" json:"city"`
	District      string         `gorm:"type:varchar(50);not null" json:"district"`
	Detail        string         `gorm:"type:varchar(255);not null" json:"detail"`
	IsDefault     bool           `gorm:"not null;default:false;index" json:"is_default"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
