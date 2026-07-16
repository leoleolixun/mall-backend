package dto

// 商品分类响应结构体
type CategoryResponse struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchant_id"`
	ParentID   int64  `json:"parent_id"`
	Name       string `json:"name"`
	Sort       int    `json:"sort"`
}

// 商品响应结构体
type ProductListItem struct {
	ID           int64  `json:"id"`
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	MerchantLogo string `json:"merchant_logo"`
	CategoryID   int64  `json:"category_id"`
	Name         string `json:"name"`
	Cover        string `json:"cover"`
	MinPrice     int64  `json:"min_price"`
}

// 商品 SKU 响应结构体
type SKUResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	Price int64  `json:"price"`
	Stock int    `json:"stock"`
}

// 商品列表请求结构体
type ProductListRequest struct {
	Page       int
	PageSize   int
	MerchantID int64
	CategoryID int64
	Keyword    string
}

// 商品详情响应结构体
type ProductDetailResponse struct {
	ID           int64         `json:"id"`
	MerchantID   int64         `json:"merchant_id"`
	MerchantName string        `json:"merchant_name"`
	MerchantLogo string        `json:"merchant_logo"`
	CategoryID   int64         `json:"category_id"`
	Name         string        `json:"name"`
	Cover        string        `json:"cover"`
	Description  string        `json:"description"`
	SKUs         []SKUResponse `json:"skus"`
}
