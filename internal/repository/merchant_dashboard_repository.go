package repository

import (
	"context"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type MerchantDashboardOverview struct {
	TotalProducts         int64
	OnSaleProducts        int64
	LowStockSKUs          int64
	OutOfStockSKUs        int64
	PendingPaymentOrders  int64
	PendingShipmentOrders int64
	TodayPaidOrders       int64
	TodayPaidAmount       int64
	TotalPaidOrders       int64
	TotalPaidAmount       int64
}

type MerchantDailySalesRecord struct {
	SalesDate  string
	PaidOrders int64
	PaidAmount int64
}

type MerchantTopProductRecord struct {
	ProductID   int64
	ProductName string
	PaidOrders  int64
	Quantity    int64
	SalesAmount int64
}

type MerchantDashboardRepository interface {
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	Overview(ctx context.Context, merchantID int64, todayStart time.Time, tomorrowStart time.Time) (*MerchantDashboardOverview, error)
	Analytics(
		ctx context.Context,
		merchantID int64,
		start time.Time,
		end time.Time,
		topLimit int,
	) ([]MerchantDailySalesRecord, []MerchantTopProductRecord, error)
}

func (r *merchantDashboardRepository) Analytics(
	ctx context.Context,
	merchantID int64,
	start time.Time,
	end time.Time,
	topLimit int,
) ([]MerchantDailySalesRecord, []MerchantTopProductRecord, error) {
	paidStatuses := []int{model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted}
	var trend []MerchantDailySalesRecord
	if err := r.db.WithContext(ctx).
		Table("orders").
		Where("merchant_id = ? AND deleted_at IS NULL AND status IN ? AND paid_at >= ? AND paid_at < ?", merchantID, paidStatuses, start, end).
		Select("DATE_FORMAT(paid_at, '%Y-%m-%d') AS sales_date, COUNT(*) AS paid_orders, COALESCE(SUM(payable_amount), 0) AS paid_amount").
		Group("DATE_FORMAT(paid_at, '%Y-%m-%d')").
		Order("sales_date ASC").
		Scan(&trend).Error; err != nil {
		return nil, nil, err
	}

	var topProducts []MerchantTopProductRecord
	if err := r.db.WithContext(ctx).
		Table("orders").
		Joins("JOIN order_items AS items ON items.order_id = orders.id").
		Where("orders.merchant_id = ? AND orders.deleted_at IS NULL AND orders.status IN ? AND orders.paid_at >= ? AND orders.paid_at < ?", merchantID, paidStatuses, start, end).
		Select("items.product_id, MAX(items.product_name) AS product_name, COUNT(DISTINCT orders.id) AS paid_orders, COALESCE(SUM(items.quantity), 0) AS quantity, COALESCE(SUM(items.subtotal), 0) AS sales_amount").
		Group("items.product_id").
		Order("sales_amount DESC, quantity DESC, items.product_id ASC").
		Limit(topLimit).
		Scan(&topProducts).Error; err != nil {
		return nil, nil, err
	}
	return trend, topProducts, nil
}

type merchantDashboardRepository struct {
	db *gorm.DB
}

func NewMerchantDashboardRepository(db *gorm.DB) MerchantDashboardRepository {
	return &merchantDashboardRepository{db: db}
}

func (r *merchantDashboardRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantDashboardRepository) Overview(
	ctx context.Context,
	merchantID int64,
	todayStart time.Time,
	tomorrowStart time.Time,
) (*MerchantDashboardOverview, error) {
	var productStats struct {
		TotalProducts  int64
		OnSaleProducts int64
	}

	if err := r.db.WithContext(ctx).
		Table("products").
		Where("merchant_id = ? AND deleted_at IS NULL", merchantID).
		Select(
			"COUNT(*) AS total_products, COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS on_sale_products",
			model.ProductStatusOnSale,
		).
		Scan(&productStats).Error; err != nil {
		return nil, err
	}

	var inventoryStats struct {
		LowStockSKUs   int64
		OutOfStockSKUs int64
	}
	if err := r.db.WithContext(ctx).
		Table("product_skus AS skus").
		Joins("JOIN products ON products.id = skus.product_id AND products.merchant_id = skus.merchant_id AND products.deleted_at IS NULL").
		Where("skus.merchant_id = ? AND skus.deleted_at IS NULL AND skus.status = ? AND skus.low_stock_threshold > 0 AND skus.stock <= skus.low_stock_threshold", merchantID, model.StatusEnabled).
		Select("COUNT(*) AS low_stock_skus, COALESCE(SUM(CASE WHEN skus.stock = 0 THEN 1 ELSE 0 END), 0) AS out_of_stock_skus").
		Scan(&inventoryStats).Error; err != nil {
		return nil, err
	}

	var orderStats struct {
		PendingPaymentOrders  int64
		PendingShipmentOrders int64
		TodayPaidOrders       int64
		TodayPaidAmount       int64
		TotalPaidOrders       int64
		TotalPaidAmount       int64
	}
	paidStatuses := []int{model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted}
	if err := r.db.WithContext(ctx).
		Table("orders").
		Where("merchant_id = ? AND deleted_at IS NULL", merchantID).
		Select(
			`COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending_payment_orders,
			 COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending_shipment_orders,
			 COALESCE(SUM(CASE WHEN status IN ? AND paid_at >= ? AND paid_at < ? THEN 1 ELSE 0 END), 0) AS today_paid_orders,
			 COALESCE(SUM(CASE WHEN status IN ? AND paid_at >= ? AND paid_at < ? THEN payable_amount ELSE 0 END), 0) AS today_paid_amount,
			 COALESCE(SUM(CASE WHEN status IN ? AND paid_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS total_paid_orders,
			 COALESCE(SUM(CASE WHEN status IN ? AND paid_at IS NOT NULL THEN payable_amount ELSE 0 END), 0) AS total_paid_amount`,
			model.OrderStatusPendingPayment,
			model.OrderStatusPaid,
			paidStatuses, todayStart, tomorrowStart,
			paidStatuses, todayStart, tomorrowStart,
			paidStatuses,
			paidStatuses,
		).
		Scan(&orderStats).Error; err != nil {
		return nil, err
	}

	return &MerchantDashboardOverview{
		TotalProducts:         productStats.TotalProducts,
		OnSaleProducts:        productStats.OnSaleProducts,
		LowStockSKUs:          inventoryStats.LowStockSKUs,
		OutOfStockSKUs:        inventoryStats.OutOfStockSKUs,
		PendingPaymentOrders:  orderStats.PendingPaymentOrders,
		PendingShipmentOrders: orderStats.PendingShipmentOrders,
		TodayPaidOrders:       orderStats.TodayPaidOrders,
		TodayPaidAmount:       orderStats.TodayPaidAmount,
		TotalPaidOrders:       orderStats.TotalPaidOrders,
		TotalPaidAmount:       orderStats.TotalPaidAmount,
	}, nil
}
