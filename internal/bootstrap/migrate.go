package bootstrap

import (
	"go-mall/internal/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
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
		&model.AfterSale{},
		&model.Refund{},
		&model.Coupon{},
		&model.UserCoupon{},
	); err != nil {
		return err
	}
	if db.Migrator().HasIndex(&model.AfterSale{}, "idx_after_sales_active_item") {
		return db.Migrator().DropIndex(&model.AfterSale{}, "idx_after_sales_active_item")
	}
	return nil
}
