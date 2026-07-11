package service

import (
	"context"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakeMerchantDashboardRepository struct {
	merchant          model.Merchant
	overview          repository.MerchantDashboardOverview
	trend             []repository.MerchantDailySalesRecord
	topProducts       []repository.MerchantTopProductRecord
	gotMerchantID     int64
	gotTodayStart     time.Time
	gotTomorrowStart  time.Time
	gotAnalyticsStart time.Time
	gotAnalyticsEnd   time.Time
	gotTopLimit       int
}

func (r *fakeMerchantDashboardRepository) Analytics(
	_ context.Context,
	merchantID int64,
	start time.Time,
	end time.Time,
	topLimit int,
) ([]repository.MerchantDailySalesRecord, []repository.MerchantTopProductRecord, error) {
	r.gotMerchantID = merchantID
	r.gotAnalyticsStart = start
	r.gotAnalyticsEnd = end
	r.gotTopLimit = topLimit
	return r.trend, r.topProducts, nil
}

func (r *fakeMerchantDashboardRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if r.merchant.ID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	merchant := r.merchant
	return &merchant, nil
}

func (r *fakeMerchantDashboardRepository) Overview(
	_ context.Context,
	merchantID int64,
	todayStart time.Time,
	tomorrowStart time.Time,
) (*repository.MerchantDashboardOverview, error) {
	r.gotMerchantID = merchantID
	r.gotTodayStart = todayStart
	r.gotTomorrowStart = tomorrowStart
	overview := r.overview
	return &overview, nil
}

func TestMerchantDashboardOverview(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 7, 11, 15, 30, 0, 0, location)
	repo := &fakeMerchantDashboardRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
		overview: repository.MerchantDashboardOverview{
			TotalProducts: 8, OnSaleProducts: 5, LowStockSKUs: 3, OutOfStockSKUs: 1,
			PendingPaymentOrders: 4, PendingShipmentOrders: 2,
			TodayPaidOrders: 3, TodayPaidAmount: 9900, TotalPaidOrders: 20, TotalPaidAmount: 88000,
		},
	}
	service := &merchantDashboardService{repo: repo, now: func() time.Time { return now }}

	result, err := service.Overview(context.Background(), 1)
	if err != nil {
		t.Fatalf("overview returned error: %v", err)
	}
	if result.TotalProducts != 8 || result.PendingShipmentOrders != 2 || result.TodayPaidAmount != 9900 || result.TotalPaidAmount != 88000 {
		t.Fatalf("unexpected dashboard overview: %+v", result)
	}
	if repo.gotMerchantID != 1 || repo.gotTodayStart.Hour() != 0 || repo.gotTomorrowStart.Sub(repo.gotTodayStart) != 24*time.Hour {
		t.Fatalf("unexpected repository arguments: merchant=%d today=%v tomorrow=%v", repo.gotMerchantID, repo.gotTodayStart, repo.gotTomorrowStart)
	}
}

func TestMerchantDashboardRejectsDisabledMerchant(t *testing.T) {
	service := NewMerchantDashboardService(&fakeMerchantDashboardRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusDisabled},
	})
	if _, err := service.Overview(context.Background(), 1); err == nil {
		t.Fatal("expected disabled merchant rejection")
	}
}

func TestMerchantDashboardAnalyticsFillsMissingDates(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 7, 11, 15, 30, 0, 0, location)
	repo := &fakeMerchantDashboardRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
		trend: []repository.MerchantDailySalesRecord{
			{SalesDate: "2026-07-09", PaidOrders: 2, PaidAmount: 5000},
			{SalesDate: "2026-07-11", PaidOrders: 1, PaidAmount: 3000},
		},
		topProducts: []repository.MerchantTopProductRecord{
			{ProductID: 10, ProductName: "测试商品", PaidOrders: 2, Quantity: 3, SalesAmount: 8000},
		},
	}
	service := &merchantDashboardService{repo: repo, now: func() time.Time { return now }}

	result, err := service.Analytics(context.Background(), 1, dto.MerchantDashboardAnalyticsRequest{Days: 3, TopLimit: 5})
	if err != nil {
		t.Fatalf("analytics returned error: %v", err)
	}
	if result.StartDate != "2026-07-09" || result.EndDate != "2026-07-11" || len(result.SalesTrend) != 3 {
		t.Fatalf("unexpected analytics period: %+v", result)
	}
	if result.SalesTrend[1].Date != "2026-07-10" || result.SalesTrend[1].PaidOrders != 0 || result.SalesTrend[1].PaidAmount != 0 {
		t.Fatalf("missing date was not filled: %+v", result.SalesTrend)
	}
	if len(result.TopProducts) != 1 || result.TopProducts[0].Quantity != 3 || repo.gotTopLimit != 5 {
		t.Fatalf("unexpected top products: %+v", result.TopProducts)
	}
}

func TestMerchantDashboardAnalyticsValidatesRange(t *testing.T) {
	service := NewMerchantDashboardService(&fakeMerchantDashboardRepository{
		merchant: model.Merchant{ID: 1, Status: model.StatusEnabled},
	})
	if _, err := service.Analytics(context.Background(), 1, dto.MerchantDashboardAnalyticsRequest{Days: 31}); err == nil {
		t.Fatal("expected days validation error")
	}
	if _, err := service.Analytics(context.Background(), 1, dto.MerchantDashboardAnalyticsRequest{TopLimit: 21}); err == nil {
		t.Fatal("expected top_limit validation error")
	}
}
