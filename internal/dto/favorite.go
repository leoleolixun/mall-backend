package dto

type FavoriteProductRequest struct {
	ProductID int64 `json:"product_id" binding:"required"`
}

type FavoriteListRequest struct {
	Page     int
	PageSize int
}
