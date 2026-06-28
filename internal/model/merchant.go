package model

import "time"

type Merchant struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	Logo      string    `gorm:"type:varchar(255)" json:"logo"`
	Status    int       `gorm:"not null;default:1;index" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
