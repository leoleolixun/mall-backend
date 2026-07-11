package bootstrap

import (
	"go-mall/internal/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantAccount{},
		&model.Category{},
		&model.Product{},
		&model.ProductSKU{},
		&model.InventoryLog{},
		&model.User{},
		&model.UserAuth{},
		&model.Address{},
		&model.Order{},
		&model.OrderItem{},
		&model.Shipment{},
		&model.Payment{},
	)
}
