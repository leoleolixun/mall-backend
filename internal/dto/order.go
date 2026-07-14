package dto

type OrderRequestItem struct {
	SKUID    int64 `json:"sku_id"`
	Quantity int   `json:"quantity"`
}

type OrderPreviewRequest struct {
	AddressID    int64              `json:"address_id"`
	UserCouponID int64              `json:"user_coupon_id"`
	Items        []OrderRequestItem `json:"items"`
}

type CreateOrderRequest struct {
	AddressID        int64              `json:"address_id"`
	UserCouponID     int64              `json:"user_coupon_id"`
	Remark           string             `json:"remark"`
	IdempotencyToken string             `json:"idempotency_token"`
	Items            []OrderRequestItem `json:"items"`
}

type OrderListRequest struct {
	Page     int
	PageSize int
	Status   int
}

type OrderItemResponse struct {
	ID          int64  `json:"id"`
	ProductID   int64  `json:"product_id"`
	SKUID       int64  `json:"sku_id"`
	ProductName string `json:"product_name"`
	SKUName     string `json:"sku_name"`
	SKUImage    string `json:"sku_image"`
	Price       int64  `json:"price"`
	Quantity    int    `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

type OrderPreviewResponse struct {
	IdempotencyToken string              `json:"idempotency_token"`
	MerchantID       int64               `json:"merchant_id"`
	MerchantName     string              `json:"merchant_name"`
	Address          AddressResponse     `json:"address"`
	Items            []OrderItemResponse `json:"items"`
	GoodsAmount      int64               `json:"goods_amount"`
	FreightAmount    int64               `json:"freight_amount"`
	DiscountAmount   int64               `json:"discount_amount"`
	PayableAmount    int64               `json:"payable_amount"`
	UserCouponID     int64               `json:"user_coupon_id"`
}

type OrderResponse struct {
	ID              int64               `json:"id"`
	OrderNo         string              `json:"order_no"`
	UserID          int64               `json:"user_id"`
	MerchantID      int64               `json:"merchant_id"`
	MerchantName    string              `json:"merchant_name"`
	Status          int                 `json:"status"`
	StatusText      string              `json:"status_text"`
	ReceiverName    string              `json:"receiver_name"`
	ReceiverPhone   string              `json:"receiver_phone"`
	ReceiverAddress string              `json:"receiver_address"`
	GoodsAmount     int64               `json:"goods_amount"`
	FreightAmount   int64               `json:"freight_amount"`
	DiscountAmount  int64               `json:"discount_amount"`
	PayableAmount   int64               `json:"payable_amount"`
	UserCouponID    int64               `json:"user_coupon_id"`
	Remark          string              `json:"remark"`
	PaidAt          *string             `json:"paid_at"`
	CancelledAt     *string             `json:"cancelled_at"`
	CompletedAt     *string             `json:"completed_at"`
	Items           []OrderItemResponse `json:"items"`
	Shipment        *ShipmentResponse   `json:"shipment,omitempty"`
	CreatedAt       string              `json:"created_at"`
	UpdatedAt       string              `json:"updated_at"`
}

type LogisticsTraceResponse struct {
	Time    string `json:"time"`
	Content string `json:"content"`
}

type LogisticsResponse struct {
	OrderID            int64                    `json:"order_id"`
	DeliveryType       string                   `json:"delivery_type"`
	LogisticsCompany   string                   `json:"logistics_company"`
	TrackingNo         string                   `json:"tracking_no"`
	ShippedAt          string                   `json:"shipped_at"`
	EstimatedArrivalAt *string                  `json:"estimated_arrival_at"`
	ReceivedAt         *string                  `json:"received_at"`
	Traces             []LogisticsTraceResponse `json:"traces"`
}
