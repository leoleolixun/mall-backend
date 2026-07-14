package dto

type MerchantCustomerListRequest struct {
	Page       int
	PageSize   int
	Keyword    string
	RepeatOnly bool
}

type MerchantCustomerOverviewResponse struct {
	TotalCustomers     int64  `json:"total_customers"`
	RepeatCustomers    int64  `json:"repeat_customers"`
	RepeatRateBPS      int64  `json:"repeat_rate_bps"`
	NewCustomers30D    int64  `json:"new_customers_30d"`
	ActiveCustomers30D int64  `json:"active_customers_30d"`
	TotalPaidAmount    int64  `json:"total_paid_amount"`
	AveragePaidAmount  int64  `json:"average_paid_amount"`
	GeneratedAt        string `json:"generated_at"`
}

type MerchantCustomerListItem struct {
	UserID          int64  `json:"user_id"`
	Nickname        string `json:"nickname"`
	Avatar          string `json:"avatar"`
	MobileMasked    string `json:"mobile_masked"`
	UserStatus      int    `json:"user_status"`
	PaidOrders      int64  `json:"paid_orders"`
	TotalPaidAmount int64  `json:"total_paid_amount"`
	FirstPaidAt     string `json:"first_paid_at"`
	LastPaidAt      string `json:"last_paid_at"`
	RegisteredAt    string `json:"registered_at"`
	IsRepeat        bool   `json:"is_repeat"`
}

type MerchantCustomerOrderResponse struct {
	ID            int64  `json:"id"`
	OrderNo       string `json:"order_no"`
	Status        int    `json:"status"`
	StatusText    string `json:"status_text"`
	PayableAmount int64  `json:"payable_amount"`
	PaidAt        string `json:"paid_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type MerchantCustomerDetailResponse struct {
	Customer     MerchantCustomerListItem        `json:"customer"`
	RecentOrders []MerchantCustomerOrderResponse `json:"recent_orders"`
}
