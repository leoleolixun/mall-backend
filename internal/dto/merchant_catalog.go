package dto

type MerchantCategoryRequest struct {
	ParentID int64  `json:"parent_id"`
	Name     string `json:"name"`
	Sort     int    `json:"sort"`
	Status   *int   `json:"status"`
}

type MerchantCategoryResponse struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchant_id"`
	ParentID   int64  `json:"parent_id"`
	Name       string `json:"name"`
	Sort       int    `json:"sort"`
	Status     int    `json:"status"`
}

type MerchantProductListRequest struct {
	Page     int
	PageSize int
	Status   *int
	Keyword  string
}

type MerchantProductRequest struct {
	CategoryID  int64  `json:"category_id"`
	Name        string `json:"name"`
	Cover       string `json:"cover"`
	Description string `json:"description"`
	Status      *int   `json:"status"`
}

type MerchantSKURequest struct {
	Name              string `json:"name"`
	Image             string `json:"image"`
	Price             int64  `json:"price"`
	Stock             int    `json:"stock"`
	LowStockThreshold *int   `json:"low_stock_threshold"`
	Status            *int   `json:"status"`
}

type MerchantSKUResponse struct {
	ID                int64  `json:"id"`
	MerchantID        int64  `json:"merchant_id"`
	ProductID         int64  `json:"product_id"`
	Name              string `json:"name"`
	Image             string `json:"image"`
	Price             int64  `json:"price"`
	Stock             int    `json:"stock"`
	LowStockThreshold int    `json:"low_stock_threshold"`
	Status            int    `json:"status"`
}

type MerchantProductResponse struct {
	ID          int64                 `json:"id"`
	MerchantID  int64                 `json:"merchant_id"`
	CategoryID  int64                 `json:"category_id"`
	Name        string                `json:"name"`
	Cover       string                `json:"cover"`
	Description string                `json:"description"`
	Status      int                   `json:"status"`
	SKUs        []MerchantSKUResponse `json:"skus"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
}
