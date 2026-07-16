package authorization

import (
	"fmt"

	"go-mall/internal/model"
)

type MerchantPermission string

func MerchantAccountSessionVersionKey(accountID int64) string {
	return fmt.Sprintf("mall:merchant:auth:session_version:%d", accountID)
}

const (
	MerchantPermissionDashboardRead  MerchantPermission = "dashboard:read"
	MerchantPermissionOrderRead      MerchantPermission = "order:read"
	MerchantPermissionOrderShip      MerchantPermission = "order:ship"
	MerchantPermissionCatalogRead    MerchantPermission = "catalog:read"
	MerchantPermissionCatalogWrite   MerchantPermission = "catalog:write"
	MerchantPermissionInventoryRead  MerchantPermission = "inventory:read"
	MerchantPermissionInventoryWrite MerchantPermission = "inventory:write"
	MerchantPermissionUpload         MerchantPermission = "upload:write"
	MerchantPermissionAccountRead    MerchantPermission = "account:read"
	MerchantPermissionAccountWrite   MerchantPermission = "account:write"
	MerchantPermissionCustomerRead   MerchantPermission = "customer:read"
	MerchantPermissionAfterSaleRead  MerchantPermission = "after_sale:read"
	MerchantPermissionAfterSaleWrite MerchantPermission = "after_sale:write"
	MerchantPermissionMarketingRead  MerchantPermission = "marketing:read"
	MerchantPermissionMarketingWrite MerchantPermission = "marketing:write"
	MerchantPermissionSettlementRead MerchantPermission = "settlement:read"
)

var allMerchantPermissions = []MerchantPermission{
	MerchantPermissionDashboardRead,
	MerchantPermissionOrderRead,
	MerchantPermissionOrderShip,
	MerchantPermissionCatalogRead,
	MerchantPermissionCatalogWrite,
	MerchantPermissionInventoryRead,
	MerchantPermissionInventoryWrite,
	MerchantPermissionUpload,
	MerchantPermissionAccountRead,
	MerchantPermissionAccountWrite,
	MerchantPermissionCustomerRead,
	MerchantPermissionAfterSaleRead,
	MerchantPermissionAfterSaleWrite,
	MerchantPermissionMarketingRead,
	MerchantPermissionMarketingWrite,
	MerchantPermissionSettlementRead,
}

var merchantOperatorPermissions = []MerchantPermission{
	MerchantPermissionDashboardRead,
	MerchantPermissionOrderRead,
	MerchantPermissionOrderShip,
	MerchantPermissionCatalogRead,
	MerchantPermissionCatalogWrite,
	MerchantPermissionInventoryRead,
	MerchantPermissionInventoryWrite,
	MerchantPermissionUpload,
	MerchantPermissionCustomerRead,
	MerchantPermissionAfterSaleRead,
	MerchantPermissionAfterSaleWrite,
	MerchantPermissionMarketingRead,
	MerchantPermissionMarketingWrite,
}

var merchantRolePermissions = map[string][]MerchantPermission{
	model.MerchantRoleOperator: merchantOperatorPermissions,
	model.MerchantRoleSales: {
		MerchantPermissionDashboardRead,
		MerchantPermissionOrderRead,
		MerchantPermissionCatalogRead,
		MerchantPermissionCustomerRead,
		MerchantPermissionAfterSaleRead,
		MerchantPermissionMarketingRead,
		MerchantPermissionMarketingWrite,
	},
	model.MerchantRoleWarehouse: {
		MerchantPermissionOrderRead,
		MerchantPermissionOrderShip,
		MerchantPermissionCatalogRead,
		MerchantPermissionInventoryRead,
		MerchantPermissionInventoryWrite,
		MerchantPermissionAfterSaleRead,
	},
}

func PermissionsForMerchantRole(role string) []MerchantPermission {
	if role == model.MerchantRoleOwner || role == model.MerchantRoleAdmin {
		return append([]MerchantPermission(nil), allMerchantPermissions...)
	}
	return append([]MerchantPermission(nil), merchantRolePermissions[role]...)
}

func MerchantRoleHasPermission(role string, permission MerchantPermission) bool {
	for _, allowed := range PermissionsForMerchantRole(role) {
		if allowed == permission {
			return true
		}
	}
	return false
}

func MerchantPermissionNames(role string) []string {
	permissions := PermissionsForMerchantRole(role)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, string(permission))
	}
	return names
}
