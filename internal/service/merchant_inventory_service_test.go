package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakeMerchantInventoryRepository struct {
	merchant model.Merchant
	product  model.Product
	sku      model.ProductSKU
	logs     []model.InventoryLog
	alerts   []repository.MerchantInventoryAlertRecord
}

func (r *fakeMerchantInventoryRepository) Transaction(_ context.Context, fn func(repository.MerchantInventoryRepository) error) error {
	return fn(r)
}

func (r *fakeMerchantInventoryRepository) FindSKUForUpdate(_ context.Context, merchantID int64, skuID int64) (*model.ProductSKU, error) {
	if r.sku.ID != skuID || r.sku.MerchantID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	return &r.sku, nil
}

func (r *fakeMerchantInventoryRepository) FindProductByID(_ context.Context, merchantID int64, productID int64) (*model.Product, error) {
	if r.product.ID != productID || r.product.MerchantID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	return &r.product, nil
}

func (r *fakeMerchantInventoryRepository) UpdateSKUStock(_ context.Context, sku *model.ProductSKU) error {
	r.sku = *sku
	return nil
}

func (r *fakeMerchantInventoryRepository) CreateInventoryLog(_ context.Context, log *model.InventoryLog) error {
	r.logs = append(r.logs, *log)
	return nil
}

func (r *fakeMerchantInventoryRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if r.merchant.ID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.merchant
	return &copy, nil
}

func (r *fakeMerchantInventoryRepository) ListAlerts(
	_ context.Context,
	merchantID int64,
	productID int64,
	skuID int64,
	keyword string,
	_ int,
	_ int,
) ([]repository.MerchantInventoryAlertRecord, int64, error) {
	result := make([]repository.MerchantInventoryAlertRecord, 0)
	for _, alert := range r.alerts {
		if alert.MerchantID != merchantID || productID > 0 && alert.ProductID != productID || skuID > 0 && alert.SKUID != skuID {
			continue
		}
		if keyword != "" && !strings.Contains(alert.ProductName, keyword) && !strings.Contains(alert.SKUName, keyword) {
			continue
		}
		result = append(result, alert)
	}
	return result, int64(len(result)), nil
}

func (r *fakeMerchantInventoryRepository) List(
	_ context.Context,
	merchantID int64,
	productID int64,
	skuID int64,
	changeType string,
	_ int,
	_ int,
) ([]model.InventoryLog, int64, error) {
	result := make([]model.InventoryLog, 0)
	for _, log := range r.logs {
		if log.MerchantID != merchantID || productID > 0 && log.ProductID != productID || skuID > 0 && log.SKUID != skuID || changeType != "" && log.ChangeType != changeType {
			continue
		}
		result = append(result, log)
	}
	return result, int64(len(result)), nil
}

func TestMerchantInventoryListFiltersCurrentMerchant(t *testing.T) {
	repo := &fakeMerchantInventoryRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
		logs: []model.InventoryLog{
			{ID: 1, MerchantID: 1, ProductID: 10, SKUID: 20, ChangeType: model.InventoryChangeMerchantAdjustment, Quantity: -2, BeforeStock: 10, AfterStock: 8, CreatedAt: time.Now()},
			{ID: 2, MerchantID: 2, ProductID: 10, SKUID: 20, ChangeType: model.InventoryChangeMerchantAdjustment, Quantity: 5, BeforeStock: 5, AfterStock: 10, CreatedAt: time.Now()},
		},
	}
	service := NewMerchantInventoryService(repo)

	result, err := service.List(context.Background(), 1, dto.MerchantInventoryLogListRequest{
		ProductID:  10,
		SKUID:      20,
		ChangeType: model.InventoryChangeMerchantAdjustment,
	})
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].MerchantID != 1 {
		t.Fatalf("unexpected inventory list: %+v", result)
	}
}

func TestMerchantInventoryRejectsUnknownChangeType(t *testing.T) {
	service := NewMerchantInventoryService(&fakeMerchantInventoryRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
	})

	_, err := service.List(context.Background(), 1, dto.MerchantInventoryLogListRequest{ChangeType: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "change_type") {
		t.Fatalf("expected change type validation error, got %v", err)
	}
}

func TestMerchantInventoryAlertsFilterCurrentMerchantAndMapSeverity(t *testing.T) {
	now := time.Now()
	repo := &fakeMerchantInventoryRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
		alerts: []repository.MerchantInventoryAlertRecord{
			{MerchantID: 1, ProductID: 10, SKUID: 20, ProductName: "测试手机", SKUName: "黑色", Stock: 2, LowStockThreshold: 5, UpdatedAt: now},
			{MerchantID: 1, ProductID: 10, SKUID: 21, ProductName: "测试手机", SKUName: "白色", Stock: 0, LowStockThreshold: 5, UpdatedAt: now},
			{MerchantID: 2, ProductID: 10, SKUID: 22, ProductName: "测试手机", SKUName: "蓝色", Stock: 1, LowStockThreshold: 5, UpdatedAt: now},
		},
	}
	service := NewMerchantInventoryService(repo)

	result, err := service.ListAlerts(context.Background(), 1, dto.MerchantInventoryAlertListRequest{Keyword: "测试"})
	if err != nil {
		t.Fatalf("list alerts returned error: %v", err)
	}
	if result.Total != 2 || len(result.List) != 2 {
		t.Fatalf("unexpected alert list: %+v", result)
	}
	if result.List[0].Severity != "low_stock" || result.List[1].Severity != "out_of_stock" {
		t.Fatalf("unexpected alert severity: %+v", result.List)
	}
}

func TestMerchantInventoryAdjustStockRecordsOperator(t *testing.T) {
	threshold := 4
	stock := 3
	repo := &fakeMerchantInventoryRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
		product:  model.Product{ID: 10, MerchantID: 1, Name: "测试商品"},
		sku:      model.ProductSKU{ID: 20, MerchantID: 1, ProductID: 10, Name: "默认规格", Stock: 8},
	}
	service := NewMerchantInventoryService(repo)

	result, err := service.AdjustStock(context.Background(), 1, 99, 20, dto.MerchantStockAdjustmentRequest{
		Stock: &stock, LowStockThreshold: &threshold, Remark: "盘点调整",
	})
	if err != nil {
		t.Fatalf("adjust stock returned error: %v", err)
	}
	if result.Stock != 3 || result.LowStockThreshold != 4 || len(repo.logs) != 1 {
		t.Fatalf("unexpected stock adjustment: result=%+v logs=%+v", result, repo.logs)
	}
	log := repo.logs[0]
	if log.Quantity != -5 || log.BeforeStock != 8 || log.AfterStock != 3 || log.OperatorID != 99 || log.Remark != "盘点调整" {
		t.Fatalf("unexpected inventory log: %+v", log)
	}
}

func TestMerchantInventoryAdjustStockRequiresStock(t *testing.T) {
	repo := &fakeMerchantInventoryRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
	}
	service := NewMerchantInventoryService(repo)
	if _, err := service.AdjustStock(context.Background(), 1, 99, 20, dto.MerchantStockAdjustmentRequest{}); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("expected missing stock rejection, got %v", err)
	}
}
