package dto

type CreateAfterSaleRequest struct {
	OrderID     int64    `json:"order_id" binding:"required"`
	OrderItemID int64    `json:"order_item_id" binding:"required"`
	Type        string   `json:"type" binding:"required"`
	Reason      string   `json:"reason" binding:"required"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
}

type RejectAfterSaleRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type AfterSaleListRequest struct {
	Page     int
	PageSize int
	Status   int
}

type RefundResponse struct {
	RefundNo      string  `json:"refund_no"`
	PayChannel    string  `json:"pay_channel"`
	Amount        int64   `json:"amount"`
	Status        int     `json:"status"`
	StatusText    string  `json:"status_text"`
	TransactionID string  `json:"transaction_id"`
	FailureReason string  `json:"failure_reason"`
	RefundedAt    *string `json:"refunded_at"`
}

type AfterSaleResponse struct {
	ID           int64           `json:"id"`
	AfterSaleNo  string          `json:"after_sale_no"`
	OrderID      int64           `json:"order_id"`
	OrderNo      string          `json:"order_no"`
	OrderItemID  int64           `json:"order_item_id"`
	ProductName  string          `json:"product_name"`
	SKUName      string          `json:"sku_name"`
	SKUImage     string          `json:"sku_image"`
	UserID       int64           `json:"user_id"`
	MerchantID   int64           `json:"merchant_id"`
	Type         string          `json:"type"`
	TypeText     string          `json:"type_text"`
	Status       int             `json:"status"`
	StatusText   string          `json:"status_text"`
	Reason       string          `json:"reason"`
	Description  string          `json:"description"`
	Images       []string        `json:"images"`
	RefundAmount int64           `json:"refund_amount"`
	RejectReason string          `json:"reject_reason"`
	ReviewedAt   *string         `json:"reviewed_at"`
	CancelledAt  *string         `json:"cancelled_at"`
	RefundedAt   *string         `json:"refunded_at"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	Refund       *RefundResponse `json:"refund,omitempty"`
}
