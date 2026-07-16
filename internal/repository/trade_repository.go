package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradeRepository interface {
	Transaction(ctx context.Context, fn func(repo TradeRepository) error) error

	FindAddress(ctx context.Context, id, userID int64) (*model.Address, error)
	FindSKUsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.ProductSKU, error)
	FindProductsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Product, error)
	FindMerchantsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Merchant, error)
	FindUserCouponsByIDs(ctx context.Context, userID int64, ids []int64, forUpdate bool) ([]model.UserCoupon, error)
	FindCouponsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Coupon, error)

	CreateTrade(ctx context.Context, trade *model.Trade) error
	CreateOrder(ctx context.Context, order *model.Order) error
	CreateOrderItems(ctx context.Context, items []model.OrderItem) error
	DecreaseSKUStock(ctx context.Context, merchantID, skuID int64, quantity int) (int, error)
	IncreaseSKUStock(ctx context.Context, merchantID, skuID int64, quantity int) (int, error)
	CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error
	UseUserCoupon(ctx context.Context, id, userID, orderID int64, usedAt time.Time) error
	ReleaseUserCoupon(ctx context.Context, id, userID, orderID int64) error
	IncrementCouponUsed(ctx context.Context, couponID int64, delta int) error

	FindTradeByIDAndUserID(ctx context.Context, id, userID int64, forUpdate bool) (*model.Trade, error)
	FindTradeByIdempotencyKey(ctx context.Context, userID int64, key string) (*model.Trade, error)
	FindOrderTradeID(ctx context.Context, orderID, userID int64) (*int64, error)
	CountOrdersByTradeID(ctx context.Context, tradeID int64) (int64, error)
	ListTradesByUserID(ctx context.Context, userID int64, status, offset, limit int) ([]model.Trade, int64, error)
	FindOrdersByTradeID(ctx context.Context, tradeID, userID int64, forUpdate bool) ([]model.Order, error)
	FindOrdersByTradeIDs(ctx context.Context, tradeIDs []int64, userID int64) ([]model.Order, error)
	FindOrderItemsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.OrderItem, error)
	ClosePendingPaymentsByTradeID(ctx context.Context, tradeID int64, closedAt time.Time) error
	CloseTrade(ctx context.Context, tradeID, userID int64, closedAt time.Time) error
	CancelOrdersByTradeID(ctx context.Context, tradeID, userID int64, cancelledAt time.Time) (int64, error)
}

type tradeRepository struct {
	db *gorm.DB
}

func NewTradeRepository(db *gorm.DB) TradeRepository {
	return &tradeRepository{db: db}
}

func (r *tradeRepository) Transaction(ctx context.Context, fn func(repo TradeRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&tradeRepository{db: tx})
	})
}

func (r *tradeRepository) FindAddress(ctx context.Context, id, userID int64) (*model.Address, error) {
	var address model.Address
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		return nil, err
	}
	return &address, nil
}

func (r *tradeRepository) FindSKUsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.ProductSKU, error) {
	if len(ids) == 0 {
		return []model.ProductSKU{}, nil
	}
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var values []model.ProductSKU
	err := query.Where("id IN ?", ids).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *tradeRepository) FindProductsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Product, error) {
	if len(ids) == 0 {
		return []model.Product{}, nil
	}
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var values []model.Product
	err := query.Where("id IN ?", ids).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *tradeRepository) FindMerchantsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Merchant, error) {
	if len(ids) == 0 {
		return []model.Merchant{}, nil
	}
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var values []model.Merchant
	err := query.Where("id IN ?", ids).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *tradeRepository) FindUserCouponsByIDs(ctx context.Context, userID int64, ids []int64, forUpdate bool) ([]model.UserCoupon, error) {
	if len(ids) == 0 {
		return []model.UserCoupon{}, nil
	}
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var values []model.UserCoupon
	err := query.Where("user_id = ? AND id IN ?", userID, ids).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *tradeRepository) FindCouponsByIDs(ctx context.Context, ids []int64, forUpdate bool) ([]model.Coupon, error) {
	if len(ids) == 0 {
		return []model.Coupon{}, nil
	}
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var values []model.Coupon
	err := query.Where("id IN ?", ids).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *tradeRepository) CreateTrade(ctx context.Context, trade *model.Trade) error {
	return r.db.WithContext(ctx).Create(trade).Error
}

func (r *tradeRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *tradeRepository) CreateOrderItems(ctx context.Context, items []model.OrderItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *tradeRepository) DecreaseSKUStock(ctx context.Context, merchantID, skuID int64, quantity int) (int, error) {
	result := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Where("merchant_id = ? AND id = ? AND stock >= ?", merchantID, skuID, quantity).
		UpdateColumn("stock", gorm.Expr("stock - ?", quantity))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("SKU %d 库存不足", skuID)
	}
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).Select("stock").Where("id = ? AND merchant_id = ?", skuID, merchantID).First(&sku).Error; err != nil {
		return 0, err
	}
	return sku.Stock, nil
}

func (r *tradeRepository) IncreaseSKUStock(ctx context.Context, merchantID, skuID int64, quantity int) (int, error) {
	result := r.db.WithContext(ctx).Unscoped().Model(&model.ProductSKU{}).
		Where("merchant_id = ? AND id = ?", merchantID, skuID).
		UpdateColumn("stock", gorm.Expr("stock + ?", quantity))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("SKU %d 不存在，无法恢复库存", skuID)
	}
	var sku model.ProductSKU
	if err := r.db.WithContext(ctx).Unscoped().Select("stock").Where("id = ? AND merchant_id = ?", skuID, merchantID).First(&sku).Error; err != nil {
		return 0, err
	}
	return sku.Stock, nil
}

func (r *tradeRepository) CreateInventoryLogs(ctx context.Context, logs []model.InventoryLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *tradeRepository) UseUserCoupon(ctx context.Context, id, userID, orderID int64, usedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, model.UserCouponStatusUnused).
		Updates(map[string]any{"status": model.UserCouponStatusUsed, "order_id": orderID, "used_at": &usedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("优惠券状态已变更")
	}
	return nil
}

func (r *tradeRepository) ReleaseUserCoupon(ctx context.Context, id, userID, orderID int64) error {
	result := r.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Where("id = ? AND user_id = ? AND order_id = ? AND status = ?", id, userID, orderID, model.UserCouponStatusUsed).
		Updates(map[string]any{"status": model.UserCouponStatusUnused, "order_id": 0, "used_at": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("优惠券状态已变更")
	}
	return nil
}

func (r *tradeRepository) IncrementCouponUsed(ctx context.Context, couponID int64, delta int) error {
	return r.db.WithContext(ctx).Model(&model.Coupon{}).Where("id = ?", couponID).
		UpdateColumn("used_quantity", gorm.Expr("GREATEST(used_quantity + ?, 0)", delta)).Error
}

func (r *tradeRepository) FindTradeByIDAndUserID(ctx context.Context, id, userID int64, forUpdate bool) (*model.Trade, error) {
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var trade model.Trade
	if err := query.Where("id = ? AND user_id = ?", id, userID).First(&trade).Error; err != nil {
		return nil, err
	}
	return &trade, nil
}

func (r *tradeRepository) FindTradeByIdempotencyKey(ctx context.Context, userID int64, key string) (*model.Trade, error) {
	var trades []model.Trade
	if err := r.db.WithContext(ctx).Where("user_id = ? AND idempotency_key = ?", userID, key).Limit(1).Find(&trades).Error; err != nil {
		return nil, err
	}
	if len(trades) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &trades[0], nil
}

func (r *tradeRepository) FindOrderTradeID(ctx context.Context, orderID, userID int64) (*int64, error) {
	type orderTradeLink struct {
		TradeID *int64 `gorm:"column:trade_id"`
	}
	var link orderTradeLink
	if err := r.db.WithContext(ctx).Table("orders").Select("trade_id").Where("id = ? AND user_id = ?", orderID, userID).Take(&link).Error; err != nil {
		return nil, err
	}
	return link.TradeID, nil
}

func (r *tradeRepository) CountOrdersByTradeID(ctx context.Context, tradeID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Order{}).Where("trade_id = ?", tradeID).Count(&count).Error
	return count, err
}

func (r *tradeRepository) ListTradesByUserID(ctx context.Context, userID int64, status, offset, limit int) ([]model.Trade, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Trade{}).Where("user_id = ?", userID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var trades []model.Trade
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&trades).Error; err != nil {
		return nil, 0, err
	}
	return trades, total, nil
}

func (r *tradeRepository) FindOrdersByTradeID(ctx context.Context, tradeID, userID int64, forUpdate bool) ([]model.Order, error) {
	query := withOptionalUpdateLock(r.db.WithContext(ctx), forUpdate)
	var orders []model.Order
	err := query.Where("trade_id = ? AND user_id = ?", tradeID, userID).Order("merchant_id ASC, id ASC").Find(&orders).Error
	return orders, err
}

func (r *tradeRepository) FindOrdersByTradeIDs(ctx context.Context, tradeIDs []int64, userID int64) ([]model.Order, error) {
	if len(tradeIDs) == 0 {
		return []model.Order{}, nil
	}
	var orders []model.Order
	err := r.db.WithContext(ctx).Where("trade_id IN ? AND user_id = ?", tradeIDs, userID).
		Order("trade_id DESC, merchant_id ASC, id ASC").Find(&orders).Error
	return orders, err
}

func (r *tradeRepository) FindOrderItemsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.OrderItem, error) {
	if len(orderIDs) == 0 {
		return []model.OrderItem{}, nil
	}
	var items []model.OrderItem
	err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).Order("order_id ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *tradeRepository) ClosePendingPaymentsByTradeID(ctx context.Context, tradeID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).Table("payments").
		Where("trade_id = ? AND status = ?", tradeID, model.PaymentStatusPending).
		Updates(map[string]any{"status": model.PaymentStatusClosed, "active_trade_id": nil, "active_order_id": nil, "closed_at": &closedAt}).Error
}

func (r *tradeRepository) CloseTrade(ctx context.Context, tradeID, userID int64, closedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.Trade{}).
		Where("id = ? AND user_id = ? AND status = ?", tradeID, userID, model.TradeStatusPendingPayment).
		Updates(map[string]any{"status": model.TradeStatusClosed, "closed_at": &closedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("交易状态已变更")
	}
	return nil
}

func (r *tradeRepository) CancelOrdersByTradeID(ctx context.Context, tradeID, userID int64, cancelledAt time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("trade_id = ? AND user_id = ? AND status = ?", tradeID, userID, model.OrderStatusPendingPayment).
		Updates(map[string]any{"status": model.OrderStatusCancelled, "cancelled_at": &cancelledAt})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("子订单状态已变更")
	}
	return result.RowsAffected, nil
}

func withOptionalUpdateLock(query *gorm.DB, forUpdate bool) *gorm.DB {
	if forUpdate {
		return query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return query
}
