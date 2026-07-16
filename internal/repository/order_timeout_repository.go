package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderTimeoutRepository interface {
	Transaction(ctx context.Context, fn func(repo OrderTimeoutRepository) error) error
	ListExpiredPendingOrderIDs(ctx context.Context, before time.Time, limit int) ([]int64, error)
	ListExpiredPendingTrades(ctx context.Context, before time.Time, limit int) ([]ExpiredPendingTrade, error)
	FindOrderForUpdate(ctx context.Context, orderID int64) (*model.Order, error)
	FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error)
	RestoreSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) (int, error)
	CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error
	ClosePendingPayments(ctx context.Context, orderID int64, closedAt time.Time) error
	MarkOrderCancelled(ctx context.Context, orderID int64, cancelledAt time.Time) error
	ReleaseOrderCoupon(ctx context.Context, order *model.Order) error
}

type ExpiredPendingTrade struct {
	ID     int64
	UserID int64
}

type orderTimeoutRepository struct {
	db *gorm.DB
}

func NewOrderTimeoutRepository(db *gorm.DB) OrderTimeoutRepository {
	return &orderTimeoutRepository{db: db}
}

func (r *orderTimeoutRepository) Transaction(
	ctx context.Context,
	fn func(repo OrderTimeoutRepository) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&orderTimeoutRepository{db: tx})
	})
}

func (r *orderTimeoutRepository) ListExpiredPendingOrderIDs(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("trade_id IS NULL AND status = ? AND created_at <= ?", model.OrderStatusPendingPayment, before).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *orderTimeoutRepository) ListExpiredPendingTrades(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]ExpiredPendingTrade, error) {
	var values []ExpiredPendingTrade
	err := r.db.WithContext(ctx).Model(&model.Trade{}).
		Select("id", "user_id").
		Where("status = ? AND created_at <= ?", model.TradeStatusPendingPayment, before).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Find(&values).Error
	return values, err
}

func (r *orderTimeoutRepository) FindOrderForUpdate(ctx context.Context, orderID int64) (*model.Order, error) {
	var order model.Order
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", orderID).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderTimeoutRepository) FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	var items []model.OrderItem
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *orderTimeoutRepository) RestoreSKUStock(
	ctx context.Context,
	merchantID int64,
	skuID int64,
	quantity int,
) (int, error) {
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
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).
		Unscoped().
		Select("stock").
		Where("id = ? AND merchant_id = ?", skuID, merchantID).
		First(&sku).Error; err != nil {
		return 0, err
	}
	return sku.Stock, nil
}

func (r *orderTimeoutRepository) CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *orderTimeoutRepository) ClosePendingPayments(ctx context.Context, orderID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusClosed,
			"active_order_id": nil,
			"closed_at":       &closedAt,
		}).Error
}

func (r *orderTimeoutRepository) MarkOrderCancelled(ctx context.Context, orderID int64, cancelledAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderStatusPendingPayment).
		Updates(map[string]interface{}{
			"status":       model.OrderStatusCancelled,
			"cancelled_at": &cancelledAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("订单状态已变更")
	}
	return nil
}

func (r *orderTimeoutRepository) ReleaseOrderCoupon(ctx context.Context, order *model.Order) error {
	if order.UserCouponID <= 0 {
		return nil
	}
	var userCoupon model.UserCoupon
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND order_id = ?", order.UserCouponID, order.UserID, order.ID).First(&userCoupon).Error; err != nil {
		return err
	}
	if userCoupon.Status != model.UserCouponStatusUsed {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&userCoupon).Updates(map[string]interface{}{"status": model.UserCouponStatusUnused, "order_id": 0, "used_at": nil}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.Coupon{}).Where("id = ?", userCoupon.CouponID).UpdateColumn("used_quantity", gorm.Expr("GREATEST(used_quantity - 1, 0)")).Error
}
