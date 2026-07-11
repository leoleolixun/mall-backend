package authorization

import "go-mall/internal/model"

type MerchantPermission string

const (
	MerchantPermissionDashboardRead  MerchantPermission = "dashboard:read"
	MerchantPermissionOrderRead      MerchantPermission = "order:read"
	MerchantPermissionOrderShip      MerchantPermission = "order:ship"
	MerchantPermissionCatalogRead    MerchantPermission = "catalog:read"
	MerchantPermissionCatalogWrite   MerchantPermission = "catalog:write"
	MerchantPermissionInventoryRead  MerchantPermission = "inventory:read"
	MerchantPermissionInventoryWrite MerchantPermission = "inventory:write"
	MerchantPermissionUpload         MerchantPermission = "upload:write"
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
}

var merchantRolePermissions = map[string][]MerchantPermission{
	model.MerchantRoleOperator: allMerchantPermissions,
	model.MerchantRoleSales: {
		MerchantPermissionDashboardRead,
		MerchantPermissionOrderRead,
		MerchantPermissionCatalogRead,
	},
	model.MerchantRoleWarehouse: {
		MerchantPermissionOrderRead,
		MerchantPermissionOrderShip,
		MerchantPermissionCatalogRead,
		MerchantPermissionInventoryRead,
		MerchantPermissionInventoryWrite,
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
