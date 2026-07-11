package dto

type MerchantDashboardOverviewResponse struct {
	TotalProducts         int64  `json:"total_products"`
	OnSaleProducts        int64  `json:"on_sale_products"`
	LowStockSKUs          int64  `json:"low_stock_skus"`
	OutOfStockSKUs        int64  `json:"out_of_stock_skus"`
	PendingPaymentOrders  int64  `json:"pending_payment_orders"`
	PendingShipmentOrders int64  `json:"pending_shipment_orders"`
	TodayPaidOrders       int64  `json:"today_paid_orders"`
	TodayPaidAmount       int64  `json:"today_paid_amount"`
	TotalPaidOrders       int64  `json:"total_paid_orders"`
	TotalPaidAmount       int64  `json:"total_paid_amount"`
	GeneratedAt           string `json:"generated_at"`
}

type MerchantDashboardAnalyticsRequest struct {
	Days     int
	TopLimit int
}

type MerchantDailySalesResponse struct {
	Date       string `json:"date"`
	PaidOrders int64  `json:"paid_orders"`
	PaidAmount int64  `json:"paid_amount"`
}

type MerchantTopProductResponse struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	PaidOrders  int64  `json:"paid_orders"`
	Quantity    int64  `json:"quantity"`
	SalesAmount int64  `json:"sales_amount"`
}

type MerchantDashboardAnalyticsResponse struct {
	StartDate   string                       `json:"start_date"`
	EndDate     string                       `json:"end_date"`
	SalesTrend  []MerchantDailySalesResponse `json:"sales_trend"`
	TopProducts []MerchantTopProductResponse `json:"top_products"`
	GeneratedAt string                       `json:"generated_at"`
}
