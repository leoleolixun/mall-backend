package bootstrap

import (
	"go-mall/internal/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Merchant{},
		&model.Category{},
		&model.Product{},
		&model.ProductSKU{},
		&model.User{},
		&model.UserAuth{},
		&model.Address{},
	)
}
