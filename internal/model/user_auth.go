package model

import (
	"time"
)

const (
	AuthProviderPassword          = "password"
	AuthProviderWechatMiniProgram = "wechat_mini_program"
)

type UserAuth struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	Provider    string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_provider_uid" json:"provider"`
	ProviderUID string    `gorm:"type:varchar(100);not null;uniqueIndex:uk_provider_uid" json:"provider_uid"`
	Credential  string    `gorm:"type:varchar(255)" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
