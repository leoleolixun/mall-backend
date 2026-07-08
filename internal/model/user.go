package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	Nickname  string         `gorm:"type:varchar(100)" json:"nickname"`
	Avatar    string         `gorm:"type:varchar(255)" json:"avatar"`
	Mobile    string         `gorm:"type:varchar(20);index" json:"mobile"`
	Gender    string         `gorm:"type:varchar(20)" json:"gender"`
	Birthday  string         `gorm:"type:varchar(20)" json:"birthday"`
	Bio       string         `gorm:"type:varchar(500)" json:"bio"`
	Status    int            `gorm:"not null;default:1;index" json:"status"` // 用户状态，1表示启用，0表示禁用
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
