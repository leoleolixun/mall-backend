package dto

type MerchantOrderListRequest struct {
	Page     int
	PageSize int
	Status   int
}

type ShipOrderRequest struct {
	DeliveryType     string `json:"delivery_type"`
	LogisticsCompany string `json:"logistics_company"`
	TrackingNo       string `json:"tracking_no"`
}

type ShipmentResponse struct {
	ID               int64  `json:"id"`
	OrderID          int64  `json:"order_id"`
	DeliveryType     string `json:"delivery_type"`
	LogisticsCompany string `json:"logistics_company"`
	TrackingNo       string `json:"tracking_no"`
	ShippedAt        string `json:"shipped_at"`
}
