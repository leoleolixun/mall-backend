package dto

type AddCartItemRequest struct {
	SKUID    int64 `json:"sku_id"`
	Quantity int   `json:"quantity"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity"`
}

type CartItemResponse struct {
	ProductID   int64  `json:"product_id"`
	SKUID       int64  `json:"sku_id"`
	ProductName string `json:"product_name"`
	SKUName     string `json:"sku_name"`
	SKUImage    string `json:"sku_image"`
	Price       int64  `json:"price"`
	Quantity    int    `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
	Stock       int    `json:"stock"`
	Available   bool   `json:"available"`
	Message     string `json:"message,omitempty"`
}
