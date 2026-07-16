package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentRepository interface {
	Transaction(ctx context.Context, fn func(repo PaymentRepository) error) error

	Create(ctx context.Context, payment *model.Payment) error
	CreateAllocations(ctx context.Context, allocations []model.PaymentAllocation) error
	FindByPaymentNo(ctx context.Context, paymentNo string) (*model.Payment, error)
	FindByPaymentNoForUpdate(ctx context.Context, paymentNo string) (*model.Payment, error)
	FindByPaymentNoAndUserID(ctx context.Context, paymentNo string, userID int64) (*model.Payment, error)
	FindLatestByOrderIDUserIDChannelScene(ctx context.Context, orderID int64, userID int64, payChannel string, payScene string) (*model.Payment, error)
	FindPendingByOrderID(ctx context.Context, orderID int64) ([]model.Payment, error)
	FindPaidByOrderID(ctx context.Context, orderID int64) (*model.Payment, error)
	FindPendingByTradeID(ctx context.Context, tradeID int64) ([]model.Payment, error)
	FindPaidByTradeID(ctx context.Context, tradeID int64) (*model.Payment, error)
	FindAllocationsByPaymentID(ctx context.Context, paymentID int64) ([]model.PaymentAllocation, error)
	MarkPaid(ctx context.Context, id int64, userID int64, transactionID string, paidAt time.Time) error
	MarkClosed(ctx context.Context, id int64, userID int64, closedAt time.Time) error
	ClosePendingByOrderID(ctx context.Context, orderID int64, closedAt time.Time) error
	ClosePendingByTradeID(ctx context.Context, tradeID int64, closedAt time.Time) error
	FindOrderByIDAndUserID(ctx context.Context, orderID int64, userID int64) (*model.Order, error)
	FindTradeByIDAndUserID(ctx context.Context, tradeID int64, userID int64) (*model.Trade, error)
	FindOrdersByTradeID(ctx context.Context, tradeID int64, userID int64) ([]model.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int64, userID int64, currentStatus int, nextStatus int, paidAt *time.Time) error
	UpdateTradeStatus(ctx context.Context, tradeID int64, userID int64, currentStatus int, nextStatus int, paidAt *time.Time) error
	UpdateTradeOrdersStatus(ctx context.Context, tradeID int64, userID int64, expectedCount int64, currentStatus int, nextStatus int, paidAt *time.Time) error
}

type paymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{
		db: db,
	}
}

func (r *paymentRepository) Transaction(ctx context.Context, fn func(repo PaymentRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&paymentRepository{db: tx})
	})
}

func (r *paymentRepository) Create(ctx context.Context, payment *model.Payment) error {
	return r.db.WithContext(ctx).Create(payment).Error
}

func (r *paymentRepository) CreateAllocations(ctx context.Context, allocations []model.PaymentAllocation) error {
	if len(allocations) == 0 {
		return fmt.Errorf("支付分配不能为空")
	}
	return r.db.WithContext(ctx).Create(&allocations).Error
}

func (r *paymentRepository) FindByPaymentNo(ctx context.Context, paymentNo string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).
		Where("payment_no = ?", paymentNo).
		First(&payment).Error
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (r *paymentRepository) FindByPaymentNoForUpdate(ctx context.Context, paymentNo string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("payment_no = ?", paymentNo).
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindByPaymentNoAndUserID(ctx context.Context, paymentNo string, userID int64) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).
		Where("payment_no = ? AND user_id = ?", paymentNo, userID).
		First(&payment).Error
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (r *paymentRepository) FindLatestByOrderIDUserIDChannelScene(
	ctx context.Context,
	orderID int64,
	userID int64,
	payChannel string,
	payScene string,
) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND user_id = ? AND pay_channel = ? AND pay_scene = ?", orderID, userID, payChannel, payScene).
		Order("id DESC").
		First(&payment).Error
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (r *paymentRepository) FindPendingByOrderID(ctx context.Context, orderID int64) ([]model.Payment, error) {
	var payments []model.Payment
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
		Order("id ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) FindPaidByOrderID(ctx context.Context, orderID int64) (*model.Payment, error) {
	var payment model.Payment
	if err := r.db.WithContext(ctx).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPaid).
		Order("id DESC").
		First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindPendingByTradeID(ctx context.Context, tradeID int64) ([]model.Payment, error) {
	var payments []model.Payment
	err := r.db.WithContext(ctx).
		Where("trade_id = ? AND status = ?", tradeID, model.PaymentStatusPending).
		Order("id ASC").
		Find(&payments).Error
	return payments, err
}

func (r *paymentRepository) FindPaidByTradeID(ctx context.Context, tradeID int64) (*model.Payment, error) {
	var payment model.Payment
	if err := r.db.WithContext(ctx).
		Where("trade_id = ? AND status IN ?", tradeID, []int{model.PaymentStatusPaid, model.PaymentStatusPartiallyRefunded, model.PaymentStatusRefunded}).
		Order("id DESC").
		First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindAllocationsByPaymentID(ctx context.Context, paymentID int64) ([]model.PaymentAllocation, error) {
	var allocations []model.PaymentAllocation
	err := r.db.WithContext(ctx).
		Where("payment_id = ?", paymentID).
		Order("merchant_id ASC, order_id ASC").
		Find(&allocations).Error
	return allocations, err
}

func (r *paymentRepository) MarkPaid(ctx context.Context, id int64, userID int64, transactionID string, paidAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusPaid,
			"active_order_id": nil,
			"active_trade_id": nil,
			"transaction_id":  transactionID,
			"paid_at":         &paidAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("支付单状态已变更")
	}

	return nil
}

func (r *paymentRepository) MarkClosed(ctx context.Context, id int64, userID int64, closedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusClosed,
			"active_order_id": nil,
			"active_trade_id": nil,
			"closed_at":       &closedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("支付单状态已变更")
	}
	return nil
}

func (r *paymentRepository) ClosePendingByTradeID(ctx context.Context, tradeID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("trade_id = ? AND status = ?", tradeID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusClosed,
			"active_order_id": nil,
			"active_trade_id": nil,
			"closed_at":       &closedAt,
		}).Error
}

func (r *paymentRepository) ClosePendingByOrderID(ctx context.Context, orderID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":          model.PaymentStatusClosed,
			"active_order_id": nil,
			"closed_at":       &closedAt,
		}).Error
}

func (r *paymentRepository) FindOrderByIDAndUserID(ctx context.Context, orderID int64, userID int64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", orderID, userID).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *paymentRepository) FindTradeByIDAndUserID(ctx context.Context, tradeID int64, userID int64) (*model.Trade, error) {
	var trade model.Trade
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", tradeID, userID).
		First(&trade).Error
	if err != nil {
		return nil, err
	}
	return &trade, nil
}

func (r *paymentRepository) FindOrdersByTradeID(ctx context.Context, tradeID int64, userID int64) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("trade_id = ? AND user_id = ?", tradeID, userID).
		Order("merchant_id ASC, id ASC").
		Find(&orders).Error
	return orders, err
}

func (r *paymentRepository) UpdateOrderStatus(
	ctx context.Context,
	orderID int64,
	userID int64,
	currentStatus int,
	nextStatus int,
	paidAt *time.Time,
) error {
	updates := map[string]interface{}{
		"status": nextStatus,
	}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}

	result := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", orderID, userID, currentStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("订单状态已变更")
	}

	return nil
}

func (r *paymentRepository) UpdateTradeStatus(
	ctx context.Context,
	tradeID int64,
	userID int64,
	currentStatus int,
	nextStatus int,
	paidAt *time.Time,
) error {
	updates := map[string]interface{}{"status": nextStatus}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}
	result := r.db.WithContext(ctx).Model(&model.Trade{}).
		Where("id = ? AND user_id = ? AND status = ?", tradeID, userID, currentStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("交易状态已变更")
	}
	return nil
}

func (r *paymentRepository) UpdateTradeOrdersStatus(
	ctx context.Context,
	tradeID int64,
	userID int64,
	expectedCount int64,
	currentStatus int,
	nextStatus int,
	paidAt *time.Time,
) error {
	updates := map[string]interface{}{"status": nextStatus}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("trade_id = ? AND user_id = ? AND status = ?", tradeID, userID, currentStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != expectedCount {
		return fmt.Errorf("子订单状态已变更: 预期更新 %d 张，实际 %d 张", expectedCount, result.RowsAffected)
	}
	return nil
}
