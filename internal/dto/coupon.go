package dto

type CouponRequest struct {
	Name            string `json:"name" binding:"required"`
	ThresholdAmount int64  `json:"threshold_amount"`
	DiscountAmount  int64  `json:"discount_amount" binding:"required"`
	TotalQuantity   int    `json:"total_quantity" binding:"required"`
	PerUserLimit    int    `json:"per_user_limit"`
	Status          int    `json:"status"`
	StartAt         string `json:"start_at" binding:"required"`
	EndAt           string `json:"end_at" binding:"required"`
}

type CouponListRequest struct {
	Page     int
	PageSize int
	Status   int
}

type CouponResponse struct {
	ID              int64  `json:"id"`
	MerchantID      int64  `json:"merchant_id"`
	Name            string `json:"name"`
	ThresholdAmount int64  `json:"threshold_amount"`
	DiscountAmount  int64  `json:"discount_amount"`
	TotalQuantity   int    `json:"total_quantity"`
	ClaimedQuantity int    `json:"claimed_quantity"`
	UsedQuantity    int    `json:"used_quantity"`
	PerUserLimit    int    `json:"per_user_limit"`
	Status          int    `json:"status"`
	StatusText      string `json:"status_text"`
	StartAt         string `json:"start_at"`
	EndAt           string `json:"end_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Claimed         bool   `json:"claimed,omitempty"`
}

type UserCouponResponse struct {
	ID         int64          `json:"id"`
	Status     int            `json:"status"`
	StatusText string         `json:"status_text"`
	ClaimedAt  string         `json:"claimed_at"`
	UsedAt     *string        `json:"used_at"`
	OrderID    int64          `json:"order_id"`
	Coupon     CouponResponse `json:"coupon"`
}
