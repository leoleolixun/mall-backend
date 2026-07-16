package repository_test

import (
	"context"
	"errors"
	"testing"

	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

func TestPublicCatalogReadOnlyIntegration(t *testing.T) {
	db := integrationMySQL(t)
	ctx := context.Background()
	merchantRepo := repository.NewMerchantRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db)

	merchants, total, err := merchantRepo.ListEnabled(ctx, 0, 50)
	if err != nil {
		t.Fatalf("list enabled merchants: %v", err)
	}
	if total == 0 || len(merchants) == 0 {
		t.Skip("no enabled merchant available for public catalog integration test")
	}
	merchant := merchants[0]
	if merchant.Status != model.StatusEnabled {
		t.Fatalf("public merchant query returned disabled merchant: %+v", merchant)
	}
	if _, err := merchantRepo.FindEnabledByID(ctx, merchant.ID); err != nil {
		t.Fatalf("find enabled merchant: %v", err)
	}
	if _, err := categoryRepo.ListEnabled(ctx, merchant.ID); err != nil {
		t.Fatalf("list merchant categories: %v", err)
	}

	products, _, err := productRepo.ListOnSale(ctx, merchant.ID, 0, "", 0, 10)
	if err != nil {
		t.Fatalf("list merchant products: %v", err)
	}
	for _, product := range products {
		if product.MerchantID != merchant.ID || product.Status != model.ProductStatusOnSale {
			t.Fatalf("merchant product filter leaked another product: %+v", product)
		}
		found, err := productRepo.FindOnSaleByID(ctx, 0, product.ID)
		if err != nil {
			t.Fatalf("find public product %d: %v", product.ID, err)
		}
		if found.MerchantID != merchant.ID {
			t.Fatalf("public product detail merchant mismatch: %+v", found)
		}
	}

	var disabled model.Merchant
	if err := db.WithContext(ctx).Where("status <> ?", model.StatusEnabled).First(&disabled).Error; err == nil {
		if _, err := merchantRepo.FindEnabledByID(ctx, disabled.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("disabled merchant %d was publicly visible: %v", disabled.ID, err)
		}
	}
}
