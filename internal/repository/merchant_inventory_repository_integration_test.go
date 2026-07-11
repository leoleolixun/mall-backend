package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

func TestMerchantInventoryAdjustmentRollsBack(t *testing.T) {
	configPath := os.Getenv("MALL_INTEGRATION_CONFIG")
	if configPath == "" {
		t.Skip("set MALL_INTEGRATION_CONFIG to run MySQL integration tests")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	var original model.ProductSKU
	if err := db.Where("deleted_at IS NULL").First(&original).Error; err != nil {
		t.Skipf("no SKU available for integration test: %v", err)
	}
	var product model.Product
	if err := db.Where("id = ? AND merchant_id = ?", original.ProductID, original.MerchantID).First(&product).Error; err != nil {
		t.Skipf("SKU product unavailable for integration test: %v", err)
	}

	repo := repository.NewMerchantInventoryRepository(db)
	rollback := errors.New("rollback integration test")
	remark := fmt.Sprintf("integration-rollback-%d", time.Now().UnixNano())
	err = repo.Transaction(context.Background(), func(txRepo repository.MerchantInventoryRepository) error {
		sku, err := txRepo.FindSKUForUpdate(context.Background(), original.MerchantID, original.ID)
		if err != nil {
			return err
		}
		sku.Stock++
		if err := txRepo.UpdateSKUStock(context.Background(), sku); err != nil {
			return err
		}
		if err := txRepo.CreateInventoryLog(context.Background(), &model.InventoryLog{
			MerchantID: original.MerchantID, ProductID: original.ProductID, SKUID: original.ID,
			ProductName: product.Name, SKUName: original.Name,
			ChangeType: model.InventoryChangeMerchantAdjustment,
			Quantity:   1, BeforeStock: original.Stock, AfterStock: original.Stock + 1,
			ReferenceType: model.InventoryReferenceSKU, ReferenceID: original.ID,
			OperatorType: model.InventoryOperatorMerchant, OperatorID: 0, Remark: remark,
		}); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	var current model.ProductSKU
	if err := db.Unscoped().First(&current, original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Stock != original.Stock {
		t.Fatalf("stock was not rolled back: got=%d want=%d", current.Stock, original.Stock)
	}
	var logCount int64
	if err := db.Model(&model.InventoryLog{}).Where("remark = ?", remark).Count(&logCount).Error; err != nil {
		t.Fatal(err)
	}
	if logCount != 0 {
		t.Fatalf("inventory log was not rolled back: count=%d", logCount)
	}
}
