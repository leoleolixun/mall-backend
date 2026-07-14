package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

func TestMerchantCustomerRepositoryIntegration(t *testing.T) {
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
	var merchant model.Merchant
	if err := db.Where("status = ?", model.StatusEnabled).First(&merchant).Error; err != nil {
		t.Skipf("no active merchant available: %v", err)
	}

	repo := repository.NewMerchantCustomerRepository(db)
	overview, err := repo.Overview(context.Background(), merchant.ID, time.Now().AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalCustomers < 0 || overview.RepeatCustomers > overview.TotalCustomers || overview.TotalPaidAmount < 0 {
		t.Fatalf("invalid customer overview: %+v", overview)
	}
	records, total, err := repo.List(context.Background(), merchant.ID, 0, 10, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if total < int64(len(records)) || len(records) > 10 {
		t.Fatalf("invalid customer page: total=%d records=%d", total, len(records))
	}
	if len(records) == 0 {
		return
	}
	record, err := repo.Find(context.Background(), merchant.ID, records[0].UserID)
	if err != nil {
		t.Fatal(err)
	}
	if record.UserID != records[0].UserID || record.PaidOrders <= 0 || record.FirstPaidAt.IsZero() || record.LastPaidAt.IsZero() {
		t.Fatalf("invalid customer detail record: %+v", record)
	}
	orders, err := repo.ListRecentOrders(context.Background(), merchant.ID, record.UserID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) > 10 {
		t.Fatalf("recent orders exceeded limit: %d", len(orders))
	}
}
