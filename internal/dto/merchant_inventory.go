package dto

type MerchantInventoryLogListRequest struct {
	Page       int
	PageSize   int
	ProductID  int64
	SKUID      int64
	ChangeType string
}

type MerchantInventoryLogResponse struct {
	ID            int64  `json:"id"`
	MerchantID    int64  `json:"merchant_id"`
	ProductID     int64  `json:"product_id"`
	SKUID         int64  `json:"sku_id"`
	ProductName   string `json:"product_name"`
	SKUName       string `json:"sku_name"`
	ChangeType    string `json:"change_type"`
	Quantity      int    `json:"quantity"`
	BeforeStock   int    `json:"before_stock"`
	AfterStock    int    `json:"after_stock"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   int64  `json:"reference_id"`
	OperatorType  string `json:"operator_type"`
	OperatorID    int64  `json:"operator_id"`
	Remark        string `json:"remark"`
	CreatedAt     string `json:"created_at"`
}

type MerchantInventoryAlertListRequest struct {
	Page      int
	PageSize  int
	ProductID int64
	SKUID     int64
	Keyword   string
}

type MerchantInventoryAlertResponse struct {
	MerchantID        int64  `json:"merchant_id"`
	ProductID         int64  `json:"product_id"`
	SKUID             int64  `json:"sku_id"`
	ProductName       string `json:"product_name"`
	SKUName           string `json:"sku_name"`
	Image             string `json:"image"`
	Stock             int    `json:"stock"`
	LowStockThreshold int    `json:"low_stock_threshold"`
	Severity          string `json:"severity"`
	UpdatedAt         string `json:"updated_at"`
}

type MerchantStockAdjustmentRequest struct {
	Stock             *int   `json:"stock"`
	LowStockThreshold *int   `json:"low_stock_threshold"`
	Remark            string `json:"remark"`
}

type MerchantStockResponse struct {
	MerchantID        int64 `json:"merchant_id"`
	ProductID         int64 `json:"product_id"`
	SKUID             int64 `json:"sku_id"`
	Stock             int   `json:"stock"`
	LowStockThreshold int   `json:"low_stock_threshold"`
}
