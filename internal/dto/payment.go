package dto

type CreatePaymentRequest struct {
	OrderID    int64  `json:"order_id"`
	PayChannel string `json:"pay_channel"`
	PayScene   string `json:"pay_scene"`
}

type PaymentResponse struct {
	ID            int64   `json:"id"`
	PaymentNo     string  `json:"payment_no"`
	OrderID       int64   `json:"order_id"`
	OrderNo       string  `json:"order_no"`
	UserID        int64   `json:"user_id"`
	MerchantID    int64   `json:"merchant_id"`
	PayChannel    string  `json:"pay_channel"`
	PayScene      string  `json:"pay_scene"`
	Status        int     `json:"status"`
	StatusText    string  `json:"status_text"`
	Amount        int64   `json:"amount"`
	TransactionID string  `json:"transaction_id"`
	FailureReason string  `json:"failure_reason"`
	PayParams     any     `json:"pay_params,omitempty"`
	PaidAt        *string `json:"paid_at"`
	ClosedAt      *string `json:"closed_at"`
}
