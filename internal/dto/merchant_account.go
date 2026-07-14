package dto

type MerchantAccountListRequest struct {
	Page     int
	PageSize int
	Role     string
	Status   *int
	Keyword  string
}

type MerchantAccountCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   *int   `json:"status"`
}

type MerchantAccountUpdateRequest struct {
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   *int   `json:"status"`
}

type MerchantAccountPasswordRequest struct {
	Password string `json:"password"`
}

type MerchantAccountListItem struct {
	ID          int64   `json:"id"`
	MerchantID  int64   `json:"merchant_id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Role        string  `json:"role"`
	RoleName    string  `json:"role_name"`
	Status      int     `json:"status"`
	LastLoginAt *string `json:"last_login_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type MerchantRoleResponse struct {
	Role        string   `json:"role"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	MemberCount int64    `json:"member_count"`
}
