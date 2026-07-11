package service

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type MerchantDashboardService interface {
	Overview(ctx context.Context, merchantID int64) (*dto.MerchantDashboardOverviewResponse, error)
	Analytics(ctx context.Context, merchantID int64, req dto.MerchantDashboardAnalyticsRequest) (*dto.MerchantDashboardAnalyticsResponse, error)
}

func (s *merchantDashboardService) validateMerchant(ctx context.Context, merchantID int64) error {
	if merchantID <= 0 {
		return fmt.Errorf("商户身份不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return fmt.Errorf("商户不可用")
	}
	return nil
}

type merchantDashboardService struct {
	repo repository.MerchantDashboardRepository
	now  func() time.Time
}

func NewMerchantDashboardService(repo repository.MerchantDashboardRepository) MerchantDashboardService {
	return &merchantDashboardService{repo: repo, now: time.Now}
}

func (s *merchantDashboardService) Overview(ctx context.Context, merchantID int64) (*dto.MerchantDashboardOverviewResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}

	now := s.now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	overview, err := s.repo.Overview(ctx, merchantID, todayStart, todayStart.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	return &dto.MerchantDashboardOverviewResponse{
		TotalProducts:         overview.TotalProducts,
		OnSaleProducts:        overview.OnSaleProducts,
		LowStockSKUs:          overview.LowStockSKUs,
		OutOfStockSKUs:        overview.OutOfStockSKUs,
		PendingPaymentOrders:  overview.PendingPaymentOrders,
		PendingShipmentOrders: overview.PendingShipmentOrders,
		TodayPaidOrders:       overview.TodayPaidOrders,
		TodayPaidAmount:       overview.TodayPaidAmount,
		TotalPaidOrders:       overview.TotalPaidOrders,
		TotalPaidAmount:       overview.TotalPaidAmount,
		GeneratedAt:           now.Format(time.RFC3339),
	}, nil
}

func (s *merchantDashboardService) Analytics(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantDashboardAnalyticsRequest,
) (*dto.MerchantDashboardAnalyticsResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	days := req.Days
	if days == 0 {
		days = 7
	}
	if days < 1 || days > 30 {
		return nil, fmt.Errorf("days 必须在 1 到 30 之间")
	}
	topLimit := req.TopLimit
	if topLimit == 0 {
		topLimit = 10
	}
	if topLimit < 1 || topLimit > 20 {
		return nil, fmt.Errorf("top_limit 必须在 1 到 20 之间")
	}

	now := s.now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := todayStart.AddDate(0, 0, -(days - 1))
	end := todayStart.AddDate(0, 0, 1)
	trendRecords, topRecords, err := s.repo.Analytics(ctx, merchantID, start, end, topLimit)
	if err != nil {
		return nil, err
	}

	trendByDate := make(map[string]repository.MerchantDailySalesRecord, len(trendRecords))
	for _, record := range trendRecords {
		trendByDate[record.SalesDate] = record
	}
	trend := make([]dto.MerchantDailySalesResponse, 0, days)
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		record := trendByDate[date]
		trend = append(trend, dto.MerchantDailySalesResponse{
			Date: date, PaidOrders: record.PaidOrders, PaidAmount: record.PaidAmount,
		})
	}
	topProducts := make([]dto.MerchantTopProductResponse, 0, len(topRecords))
	for _, record := range topRecords {
		topProducts = append(topProducts, dto.MerchantTopProductResponse{
			ProductID: record.ProductID, ProductName: record.ProductName, PaidOrders: record.PaidOrders,
			Quantity: record.Quantity, SalesAmount: record.SalesAmount,
		})
	}
	return &dto.MerchantDashboardAnalyticsResponse{
		StartDate: start.Format("2006-01-02"), EndDate: todayStart.Format("2006-01-02"),
		SalesTrend: trend, TopProducts: topProducts, GeneratedAt: now.Format(time.RFC3339),
	}, nil
}
