package dto

type PublicMerchantListRequest struct {
	Page     int
	PageSize int
}

type PublicMerchantResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Logo   string `json:"logo"`
	Status int    `json:"status"`
}
