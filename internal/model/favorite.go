package model

import "time"

type FavoriteProduct struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     int64     `gorm:"not null;index;uniqueIndex:idx_favorite_user_merchant_product,priority:1" json:"user_id"`
	MerchantID int64     `gorm:"not null;index;uniqueIndex:idx_favorite_user_merchant_product,priority:2" json:"merchant_id"`
	ProductID  int64     `gorm:"not null;index;uniqueIndex:idx_favorite_user_merchant_product,priority:3" json:"product_id"`
	CreatedAt  time.Time `json:"created_at"`
}
