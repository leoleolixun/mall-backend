package dto

type MerchantCouponSelection struct {
	MerchantID   int64 `json:"merchant_id"`
	UserCouponID int64 `json:"user_coupon_id"`
}

type TradePreviewRequest struct {
	AddressID       int64                     `json:"address_id"`
	MerchantCoupons []MerchantCouponSelection `json:"merchant_coupons"`
	Items           []OrderRequestItem        `json:"items"`
}

type CreateTradeRequest struct {
	AddressID        int64                     `json:"address_id"`
	MerchantCoupons  []MerchantCouponSelection `json:"merchant_coupons"`
	Remark           string                    `json:"remark"`
	IdempotencyToken string                    `json:"idempotency_token"`
	Items            []OrderRequestItem        `json:"items"`
}

type TradeMerchantGroupResponse struct {
	MerchantID     int64               `json:"merchant_id"`
	MerchantName   string              `json:"merchant_name"`
	MerchantLogo   string              `json:"merchant_logo"`
	Items          []OrderItemResponse `json:"items"`
	GoodsAmount    int64               `json:"goods_amount"`
	FreightAmount  int64               `json:"freight_amount"`
	DiscountAmount int64               `json:"discount_amount"`
	PayableAmount  int64               `json:"payable_amount"`
	UserCouponID   int64               `json:"user_coupon_id"`
}

type TradePreviewResponse struct {
	IdempotencyToken string                       `json:"idempotency_token"`
	Address          AddressResponse              `json:"address"`
	MerchantGroups   []TradeMerchantGroupResponse `json:"merchant_groups"`
	GoodsAmount      int64                        `json:"goods_amount"`
	FreightAmount    int64                        `json:"freight_amount"`
	DiscountAmount   int64                        `json:"discount_amount"`
	PayableAmount    int64                        `json:"payable_amount"`
}

type TradeListRequest struct {
	Page     int
	PageSize int
	Status   int
}

type TradeResponse struct {
	ID             int64           `json:"id"`
	TradeNo        string          `json:"trade_no"`
	UserID         int64           `json:"user_id"`
	Status         int             `json:"status"`
	StatusText     string          `json:"status_text"`
	GoodsAmount    int64           `json:"goods_amount"`
	FreightAmount  int64           `json:"freight_amount"`
	DiscountAmount int64           `json:"discount_amount"`
	PayableAmount  int64           `json:"payable_amount"`
	PaidAt         *string         `json:"paid_at"`
	ClosedAt       *string         `json:"closed_at"`
	Orders         []OrderResponse `json:"orders"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}
