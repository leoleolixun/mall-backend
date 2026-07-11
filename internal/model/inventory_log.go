package model

import "time"

const (
	InventoryChangeOrderCreate        = "order_create"
	InventoryChangeOrderCancel        = "order_cancel"
	InventoryChangeOrderTimeout       = "order_timeout"
	InventoryChangeMerchantInit       = "merchant_init"
	InventoryChangeMerchantAdjustment = "merchant_adjustment"
)

const (
	InventoryReferenceOrder = "order"
	InventoryReferenceSKU   = "sku"
)

const (
	InventoryOperatorSystem   = "system"
	InventoryOperatorUser     = "user"
	InventoryOperatorMerchant = "merchant"
)

type InventoryLog struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	MerchantID    int64     `gorm:"not null;index;index:idx_inventory_merchant_created,priority:1;index:idx_inventory_merchant_sku_created,priority:1" json:"merchant_id"`
	ProductID     int64     `gorm:"not null;index" json:"product_id"`
	SKUID         int64     `gorm:"column:sku_id;not null;index:idx_inventory_logs_sku_id;index:idx_inventory_merchant_sku_created,priority:2" json:"sku_id"`
	ProductName   string    `gorm:"type:varchar(200);not null" json:"product_name"`
	SKUName       string    `gorm:"type:varchar(200);not null" json:"sku_name"`
	ChangeType    string    `gorm:"type:varchar(32);not null;index" json:"change_type"`
	Quantity      int       `gorm:"not null" json:"quantity"`
	BeforeStock   int       `gorm:"not null" json:"before_stock"`
	AfterStock    int       `gorm:"not null" json:"after_stock"`
	ReferenceType string    `gorm:"type:varchar(32);not null;index:idx_inventory_reference,priority:1" json:"reference_type"`
	ReferenceID   int64     `gorm:"not null;index:idx_inventory_reference,priority:2" json:"reference_id"`
	OperatorType  string    `gorm:"type:varchar(32);not null" json:"operator_type"`
	OperatorID    int64     `gorm:"not null;default:0" json:"operator_id"`
	Remark        string    `gorm:"type:varchar(255);not null;default:''" json:"remark"`
	CreatedAt     time.Time `gorm:"index:idx_inventory_merchant_created,priority:2;index:idx_inventory_merchant_sku_created,priority:3" json:"created_at"`
}
