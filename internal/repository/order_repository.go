package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderRepository interface {
	Transaction(ctx context.Context, fn func(repo OrderRepository) error) error

	CreateTrade(ctx context.Context, trade *model.Trade) error
	Create(ctx context.Context, order *model.Order) error
	CreateItems(ctx context.Context, items []model.OrderItem) error
	Update(ctx context.Context, order *model.Order) error
	UpdateStatus(ctx context.Context, id int64, userID int64, currentStatus int, nextStatus int, paidAt *time.Time, cancelledAt *time.Time) error

	FindByIDAndUserID(ctx context.Context, id int64, userID int64) (*model.Order, error)
	FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error)
	FindShipmentByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error)
	ListByUserID(ctx context.Context, userID int64, offset int, limit int, status int) ([]model.Order, int64, error)
	Complete(ctx context.Context, id int64, userID int64, completedAt time.Time) error

	DecreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) (int, error)
	IncreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) (int, error)
	CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error
	ClosePendingPayments(ctx context.Context, orderID int64, closedAt time.Time) error
	FindUserCoupon(ctx context.Context, id, userID int64) (*model.UserCoupon, error)
	FindUserCouponForUpdate(ctx context.Context, id, userID int64) (*model.UserCoupon, error)
	FindCoupon(ctx context.Context, id int64) (*model.Coupon, error)
	FindMerchantByID(ctx context.Context, id int64) (*model.Merchant, error)
	UseUserCoupon(ctx context.Context, id, userID, orderID int64, usedAt time.Time) error
	ReleaseUserCoupon(ctx context.Context, id, userID, orderID int64) error
	IncrementCouponUsed(ctx context.Context, couponID int64, delta int) error
}

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{
		db: db,
	}
}

func (r *orderRepository) Transaction(ctx context.Context, fn func(repo OrderRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&orderRepository{db: tx})
	})
}

func (r *orderRepository) CreateTrade(ctx context.Context, trade *model.Trade) error {
	return r.db.WithContext(ctx).Create(trade).Error
}

func (r *orderRepository) Create(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderRepository) CreateItems(ctx context.Context, items []model.OrderItem) error {
	if len(items) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *orderRepository) Update(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *orderRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	userID int64,
	currentStatus int,
	nextStatus int,
	paidAt *time.Time,
	cancelledAt *time.Time,
) error {
	updates := map[string]interface{}{
		"status": nextStatus,
	}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}
	if cancelledAt != nil {
		updates["cancelled_at"] = cancelledAt
	}

	result := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, currentStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("订单状态已变更")
	}

	return nil
}

func (r *orderRepository) FindByIDAndUserID(ctx context.Context, id int64, userID int64) (*model.Order, error) {
	var order model.Order

	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *orderRepository) FindMerchantByID(ctx context.Context, id int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *orderRepository) FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	var items []model.OrderItem

	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&items).Error

	return items, err
}

func (r *orderRepository) FindShipmentByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error) {
	var shipment model.Shipment
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&shipment).Error; err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *orderRepository) Complete(ctx context.Context, id int64, userID int64, completedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Order{}).Where("id = ? AND user_id = ? AND status = ?", id, userID, model.OrderStatusShipped).Updates(map[string]interface{}{"status": model.OrderStatusCompleted, "completed_at": completedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("订单状态已变更")
		}
		return tx.Model(&model.Shipment{}).Where("order_id = ? AND received_at IS NULL", id).Update("received_at", &completedAt).Error
	})
}

func (r *orderRepository) FindUserCoupon(ctx context.Context, id, userID int64) (*model.UserCoupon, error) {
	var value model.UserCoupon
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&value).Error
	return &value, err
}

func (r *orderRepository) FindUserCouponForUpdate(ctx context.Context, id, userID int64) (*model.UserCoupon, error) {
	var value model.UserCoupon
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", id, userID).First(&value).Error
	return &value, err
}

func (r *orderRepository) FindCoupon(ctx context.Context, id int64) (*model.Coupon, error) {
	var value model.Coupon
	err := r.db.WithContext(ctx).First(&value, id).Error
	return &value, err
}

func (r *orderRepository) UseUserCoupon(ctx context.Context, id, userID, orderID int64, usedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.UserCoupon{}).Where("id = ? AND user_id = ? AND status = ?", id, userID, model.UserCouponStatusUnused).Updates(map[string]interface{}{"status": model.UserCouponStatusUsed, "order_id": orderID, "used_at": &usedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("优惠券状态已变更")
	}
	return nil
}

func (r *orderRepository) ReleaseUserCoupon(ctx context.Context, id, userID, orderID int64) error {
	return r.db.WithContext(ctx).Model(&model.UserCoupon{}).Where("id = ? AND user_id = ? AND order_id = ? AND status = ?", id, userID, orderID, model.UserCouponStatusUsed).Updates(map[string]interface{}{"status": model.UserCouponStatusUnused, "order_id": 0, "used_at": nil}).Error
}

func (r *orderRepository) IncrementCouponUsed(ctx context.Context, couponID int64, delta int) error {
	return r.db.WithContext(ctx).Model(&model.Coupon{}).Where("id = ?", couponID).UpdateColumn("used_quantity", gorm.Expr("GREATEST(used_quantity + ?, 0)", delta)).Error
}

func (r *orderRepository) ListByUserID(
	ctx context.Context,
	userID int64,
	offset int,
	limit int,
	status int,
) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("user_id = ?", userID)

	if status > 0 {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *orderRepository) DecreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) (int, error) {
	result := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Where("merchant_id = ? AND id = ? AND stock >= ?", merchantID, skuID, quantity).
		UpdateColumn("stock", gorm.Expr("stock - ?", quantity))

	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("库存不足")
	}

	return r.findSKUStock(ctx, merchantID, skuID, false)
}

func (r *orderRepository) IncreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) (int, error) {
	result := r.db.WithContext(ctx).
		Unscoped().
		Model(&model.ProductSKU{}).
		Where("id = ? AND merchant_id = ?", skuID, merchantID).
		UpdateColumn("stock", gorm.Expr("stock + ?", quantity))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("SKU %d 不存在，无法恢复库存", skuID)
	}
	return r.findSKUStock(ctx, merchantID, skuID, true)
}

func (r *orderRepository) findSKUStock(ctx context.Context, merchantID int64, skuID int64, unscoped bool) (int, error) {
	query := r.db.WithContext(ctx).Model(&model.ProductSKU{})
	if unscoped {
		query = query.Unscoped()
	}
	var sku model.ProductSKU
	if err := query.Select("stock").Where("id = ? AND merchant_id = ?", skuID, merchantID).First(&sku).Error; err != nil {
		return 0, err
	}
	return sku.Stock, nil
}

func (r *orderRepository) CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *orderRepository) ClosePendingPayments(ctx context.Context, orderID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusClosed,
			"active_order_id": nil,
			"closed_at":       &closedAt,
		}).Error
}
