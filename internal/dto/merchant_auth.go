package dto

type MerchantLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type MerchantRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type MerchantAccountResponse struct {
	ID           int64    `json:"id"`
	MerchantID   int64    `json:"merchant_id"`
	MerchantName string   `json:"merchant_name"`
	Username     string   `json:"username"`
	Nickname     string   `json:"nickname"`
	Role         string   `json:"role"`
	Permissions  []string `json:"permissions"`
}

type MerchantAuthResponse struct {
	AccessToken  string                  `json:"access_token"`
	RefreshToken string                  `json:"refresh_token"`
	User         MerchantAccountResponse `json:"user"`
}
