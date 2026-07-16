package dto

type SettlementEntryListRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	EntryType string `form:"entry_type"`
}

type MerchantSettlementListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
	Status   int `form:"status"`
}

type SettlementEntryResponse struct {
	ID           int64  `json:"id"`
	EntryNo      string `json:"entry_no"`
	MerchantID   int64  `json:"merchant_id"`
	OrderID      *int64 `json:"order_id,omitempty"`
	RefundID     *int64 `json:"refund_id,omitempty"`
	EntryType    string `json:"entry_type"`
	Amount       int64  `json:"amount"`
	AvailableAt  string `json:"available_at"`
	SettlementID *int64 `json:"settlement_id,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type MerchantSettlementResponse struct {
	ID               int64                     `json:"id"`
	SettlementNo     string                    `json:"settlement_no"`
	MerchantID       int64                     `json:"merchant_id"`
	PeriodStart      string                    `json:"period_start"`
	PeriodEnd        string                    `json:"period_end"`
	GrossAmount      int64                     `json:"gross_amount"`
	CommissionAmount int64                     `json:"commission_amount"`
	RefundAmount     int64                     `json:"refund_amount"`
	NetAmount        int64                     `json:"net_amount"`
	Status           int                       `json:"status"`
	StatusText       string                    `json:"status_text"`
	ConfirmedAt      *string                   `json:"confirmed_at"`
	PaidAt           *string                   `json:"paid_at"`
	CreatedAt        string                    `json:"created_at"`
	UpdatedAt        string                    `json:"updated_at"`
	Entries          []SettlementEntryResponse `json:"entries,omitempty"`
}
