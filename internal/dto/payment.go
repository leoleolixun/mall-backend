package dto

type CreatePaymentRequest struct {
	OrderID    int64  `json:"order_id,omitempty"`
	TradeID    int64  `json:"trade_id,omitempty"`
	PayChannel string `json:"pay_channel"`
	PayScene   string `json:"pay_scene"`
}

type PaymentAllocationResponse struct {
	OrderID    int64 `json:"order_id"`
	MerchantID int64 `json:"merchant_id"`
	Amount     int64 `json:"amount"`
}

type PaymentResponse struct {
	ID            int64                       `json:"id"`
	PaymentNo     string                      `json:"payment_no"`
	TradeID       *int64                      `json:"trade_id,omitempty"`
	TradeNo       string                      `json:"trade_no,omitempty"`
	OrderID       *int64                      `json:"order_id,omitempty"`
	OrderNo       string                      `json:"order_no,omitempty"`
	UserID        int64                       `json:"user_id"`
	MerchantID    *int64                      `json:"merchant_id,omitempty"`
	PayChannel    string                      `json:"pay_channel"`
	PayScene      string                      `json:"pay_scene"`
	Status        int                         `json:"status"`
	StatusText    string                      `json:"status_text"`
	Amount        int64                       `json:"amount"`
	Allocations   []PaymentAllocationResponse `json:"allocations,omitempty"`
	TransactionID string                      `json:"transaction_id"`
	FailureReason string                      `json:"failure_reason"`
	PayParams     any                         `json:"pay_params,omitempty"`
	PaidAt        *string                     `json:"paid_at"`
	ClosedAt      *string                     `json:"closed_at"`
}
