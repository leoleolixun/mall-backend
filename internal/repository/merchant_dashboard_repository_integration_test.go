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

func TestMerchantDashboardRepositoryIntegration(t *testing.T) {
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
		t.Fatal(err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	overview, err := repository.NewMerchantDashboardRepository(db).Overview(
		context.Background(),
		merchant.ID,
		todayStart,
		todayStart.AddDate(0, 0, 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	var expectedProducts int64
	if err := db.Model(&model.Product{}).Where("merchant_id = ?", merchant.ID).Count(&expectedProducts).Error; err != nil {
		t.Fatal(err)
	}
	if overview.TotalProducts != expectedProducts {
		t.Fatalf("unexpected product count: got=%d want=%d", overview.TotalProducts, expectedProducts)
	}
	if overview.TotalProducts < 0 || overview.TotalPaidAmount < 0 {
		t.Fatalf("dashboard aggregate must not be negative: %+v", overview)
	}
	trend, topProducts, err := repository.NewMerchantDashboardRepository(db).Analytics(
		context.Background(), merchant.ID, todayStart.AddDate(0, 0, -6), todayStart.AddDate(0, 0, 1), 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range trend {
		if record.SalesDate == "" || record.PaidOrders < 0 || record.PaidAmount < 0 {
			t.Fatalf("invalid sales trend record: %+v", record)
		}
	}
	if len(topProducts) > 10 {
		t.Fatalf("top products exceeded limit: %d", len(topProducts))
	}
}
