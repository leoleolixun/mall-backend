package model

import "time"

const (
	MerchantRoleOwner     = "owner"
	MerchantRoleAdmin     = "admin"
	MerchantRoleOperator  = "operator"
	MerchantRoleSales     = "sales"
	MerchantRoleWarehouse = "warehouse"
)

func IsValidMerchantRole(role string) bool {
	switch role {
	case MerchantRoleOwner, MerchantRoleAdmin, MerchantRoleOperator, MerchantRoleSales, MerchantRoleWarehouse:
		return true
	default:
		return false
	}
}

type MerchantAccount struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	MerchantID   int64      `gorm:"not null;index" json:"merchant_id"`
	Username     string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	Nickname     string     `gorm:"type:varchar(100);not null" json:"nickname"`
	Role         string     `gorm:"type:varchar(32);not null;default:operator;index" json:"role"`
	Status       int        `gorm:"not null;default:1;index" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	Merchant Merchant `gorm:"foreignKey:MerchantID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
}
