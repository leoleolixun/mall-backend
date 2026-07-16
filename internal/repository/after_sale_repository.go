package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AfterSaleRepository interface {
	Transaction(ctx context.Context, fn func(repo AfterSaleRepository) error) error
	Create(ctx context.Context, afterSale *model.AfterSale) error
	FindByIDAndUserID(ctx context.Context, id, userID int64) (*model.AfterSale, error)
	FindByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.AfterSale, error)
	FindForUpdateByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.AfterSale, error)
	FindAfterSaleForUpdate(ctx context.Context, id int64) (*model.AfterSale, error)
	FindOrderByIDAndUserID(ctx context.Context, orderID, userID int64) (*model.Order, error)
	FindOrderItem(ctx context.Context, orderID, orderItemID int64) (*model.OrderItem, error)
	FindLatestByOrderItem(ctx context.Context, orderItemID int64) (*model.AfterSale, error)
	FindOrder(ctx context.Context, orderID int64) (*model.Order, error)
	FindOrderForUpdate(ctx context.Context, orderID int64) (*model.Order, error)
	FindItem(ctx context.Context, orderItemID int64) (*model.OrderItem, error)
	FindPaidPaymentByOrderID(ctx context.Context, orderID int64) (*model.Payment, error)
	FindPaidPaymentAllocationByOrderID(ctx context.Context, orderID int64) (*model.Payment, *model.PaymentAllocation, error)
	FindPaymentByID(ctx context.Context, id int64) (*model.Payment, error)
	FindPaymentForUpdate(ctx context.Context, id int64) (*model.Payment, error)
	FindPaymentAllocationByID(ctx context.Context, id int64) (*model.PaymentAllocation, error)
	FindPaymentAllocationForUpdate(ctx context.Context, id int64) (*model.PaymentAllocation, error)
	FindRefundByAfterSaleID(ctx context.Context, afterSaleID int64) (*model.Refund, error)
	FindRefundByID(ctx context.Context, id int64) (*model.Refund, error)
	FindRefundForUpdate(ctx context.Context, id int64) (*model.Refund, error)
	ListRefundIDsForReconciliation(ctx context.Context, now time.Time, limit int) ([]int64, error)
	CreateRefund(ctx context.Context, refund *model.Refund) error
	CreateSettlementEntries(ctx context.Context, entries []model.SettlementEntry) error
	MarkRefundPending(ctx context.Context, refundID int64) error
	MarkRefundSucceeded(ctx context.Context, refundID int64, transactionID string, refundedAt time.Time) error
	MarkRefundFailed(ctx context.Context, refundID int64, reason string, attemptedAt time.Time) error
	MarkRefundUnknown(ctx context.Context, refundID int64, reason string, attemptedAt, nextRetryAt time.Time) error
	SumSucceededRefunds(ctx context.Context, paymentID int64) (int64, error)
	SumReservedRefundsByAllocation(ctx context.Context, allocationID int64, excludeRefundID int64) (int64, error)
	IncreaseAllocationRefundedAmount(ctx context.Context, allocationID int64, amount int64) error
	UpdatePaymentRefundStatus(ctx context.Context, paymentID int64, status int) error
	UpdateTradeRefundStatus(ctx context.Context, tradeID int64, status int) error
	UpdateStatus(ctx context.Context, id int64, currentStatuses []int, updates map[string]interface{}) error
	ListByUserID(ctx context.Context, userID int64, offset, limit, status int) ([]model.AfterSale, int64, error)
	ListByMerchantID(ctx context.Context, merchantID int64, offset, limit, status int) ([]model.AfterSale, int64, error)
}

type afterSaleRepository struct{ db *gorm.DB }

func NewAfterSaleRepository(db *gorm.DB) AfterSaleRepository { return &afterSaleRepository{db: db} }

func (r *afterSaleRepository) Transaction(ctx context.Context, fn func(repo AfterSaleRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&afterSaleRepository{db: tx}) })
}

func (r *afterSaleRepository) Create(ctx context.Context, afterSale *model.AfterSale) error {
	return r.db.WithContext(ctx).Create(afterSale).Error
}

func (r *afterSaleRepository) FindByIDAndUserID(ctx context.Context, id, userID int64) (*model.AfterSale, error) {
	var value model.AfterSale
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.AfterSale, error) {
	var value model.AfterSale
	err := r.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", id, merchantID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindForUpdateByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.AfterSale, error) {
	var value model.AfterSale
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND merchant_id = ?", id, merchantID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindAfterSaleForUpdate(ctx context.Context, id int64) (*model.AfterSale, error) {
	var value model.AfterSale
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindOrderByIDAndUserID(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	var value model.Order
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", orderID, userID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindOrderItem(ctx context.Context, orderID, orderItemID int64) (*model.OrderItem, error) {
	var value model.OrderItem
	err := r.db.WithContext(ctx).Where("id = ? AND order_id = ?", orderItemID, orderID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindLatestByOrderItem(ctx context.Context, orderItemID int64) (*model.AfterSale, error) {
	var value model.AfterSale
	err := r.db.WithContext(ctx).Where("order_item_id = ?", orderItemID).Order("id DESC").First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindOrder(ctx context.Context, orderID int64) (*model.Order, error) {
	var value model.Order
	err := r.db.WithContext(ctx).First(&value, orderID).Error
	return &value, err
}

func (r *afterSaleRepository) FindOrderForUpdate(ctx context.Context, orderID int64) (*model.Order, error) {
	var value model.Order
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, orderID).Error
	return &value, err
}

func (r *afterSaleRepository) FindItem(ctx context.Context, orderItemID int64) (*model.OrderItem, error) {
	var value model.OrderItem
	err := r.db.WithContext(ctx).First(&value, orderItemID).Error
	return &value, err
}

func (r *afterSaleRepository) FindPaidPaymentByOrderID(ctx context.Context, orderID int64) (*model.Payment, error) {
	var value model.Payment
	err := r.db.WithContext(ctx).Where("order_id = ? AND status IN ?", orderID, []int{model.PaymentStatusPaid, model.PaymentStatusPartiallyRefunded}).Order("id DESC").First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindPaidPaymentAllocationByOrderID(
	ctx context.Context,
	orderID int64,
) (*model.Payment, *model.PaymentAllocation, error) {
	var allocation model.PaymentAllocation
	err := r.db.WithContext(ctx).Table("payment_allocations AS pa").
		Select("pa.*").
		Joins("JOIN payments p ON p.id = pa.payment_id").
		Where("pa.order_id = ? AND p.status IN ?", orderID, []int{
			model.PaymentStatusPaid, model.PaymentStatusPartiallyRefunded, model.PaymentStatusRefunded,
		}).
		Order("p.id DESC").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&allocation).Error
	if err != nil {
		return nil, nil, err
	}
	payment, err := r.FindPaymentForUpdate(ctx, allocation.PaymentID)
	if err != nil {
		return nil, nil, err
	}
	return payment, &allocation, nil
}

func (r *afterSaleRepository) FindPaymentByID(ctx context.Context, id int64) (*model.Payment, error) {
	var value model.Payment
	err := r.db.WithContext(ctx).First(&value, id).Error
	return &value, err
}

func (r *afterSaleRepository) FindPaymentForUpdate(ctx context.Context, id int64) (*model.Payment, error) {
	var value model.Payment
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error
	return &value, err
}

func (r *afterSaleRepository) FindPaymentAllocationByID(ctx context.Context, id int64) (*model.PaymentAllocation, error) {
	var value model.PaymentAllocation
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindPaymentAllocationForUpdate(ctx context.Context, id int64) (*model.PaymentAllocation, error) {
	var value model.PaymentAllocation
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindRefundByAfterSaleID(ctx context.Context, afterSaleID int64) (*model.Refund, error) {
	var value model.Refund
	err := r.db.WithContext(ctx).Where("after_sale_id = ?", afterSaleID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindRefundByID(ctx context.Context, id int64) (*model.Refund, error) {
	var value model.Refund
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) FindRefundForUpdate(ctx context.Context, id int64) (*model.Refund, error) {
	var value model.Refund
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) ListRefundIDsForReconciliation(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).
		Model(&model.Refund{}).
		Where("status IN ?", []int{model.RefundStatusPending, model.RefundStatusUnknown}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", now).
		Order("COALESCE(next_retry_at, created_at) ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *afterSaleRepository) CreateRefund(ctx context.Context, refund *model.Refund) error {
	return r.db.WithContext(ctx).Create(refund).Error
}

func (r *afterSaleRepository) CreateSettlementEntries(ctx context.Context, entries []model.SettlementEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&entries).Error
}

func (r *afterSaleRepository) MarkRefundPending(ctx context.Context, refundID int64) error {
	result := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status = ?", refundID, model.RefundStatusFailed).
		Updates(map[string]interface{}{
			"status":         model.RefundStatusPending,
			"failure_reason": "",
			"last_error":     "",
			"next_retry_at":  nil,
		})
	return refundUpdateError(result)
}

func (r *afterSaleRepository) MarkRefundSucceeded(ctx context.Context, refundID int64, transactionID string, refundedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status IN ?", refundID, []int{model.RefundStatusPending, model.RefundStatusUnknown}).
		Updates(map[string]interface{}{
			"status":          model.RefundStatusSucceeded,
			"transaction_id":  transactionID,
			"refunded_at":     &refundedAt,
			"failure_reason":  "",
			"last_error":      "",
			"last_attempt_at": &refundedAt,
			"next_retry_at":   nil,
		})
	return refundUpdateError(result)
}

func (r *afterSaleRepository) MarkRefundFailed(ctx context.Context, refundID int64, reason string, attemptedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status IN ?", refundID, []int{model.RefundStatusPending, model.RefundStatusUnknown}).
		Updates(map[string]interface{}{
			"status":          model.RefundStatusFailed,
			"failure_reason":  reason,
			"last_error":      "",
			"last_attempt_at": &attemptedAt,
			"next_retry_at":   nil,
		})
	return refundUpdateError(result)
}

func (r *afterSaleRepository) MarkRefundUnknown(ctx context.Context, refundID int64, reason string, attemptedAt, nextRetryAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status IN ?", refundID, []int{model.RefundStatusPending, model.RefundStatusUnknown}).
		Updates(map[string]interface{}{
			"status":          model.RefundStatusUnknown,
			"failure_reason":  "",
			"last_error":      reason,
			"retry_count":     gorm.Expr("retry_count + 1"),
			"last_attempt_at": &attemptedAt,
			"next_retry_at":   &nextRetryAt,
		})
	return refundUpdateError(result)
}

func refundUpdateError(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("退款单状态已变更")
	}
	return nil
}

func (r *afterSaleRepository) SumSucceededRefunds(ctx context.Context, paymentID int64) (int64, error) {
	var refunds []model.Refund
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "amount").
		Where("payment_id = ? AND status = ?", paymentID, model.RefundStatusSucceeded).
		Find(&refunds).Error; err != nil {
		return 0, err
	}
	var amount int64
	for _, refund := range refunds {
		amount += refund.Amount
	}
	return amount, nil
}

func (r *afterSaleRepository) SumReservedRefundsByAllocation(
	ctx context.Context,
	allocationID int64,
	excludeRefundID int64,
) (int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("payment_allocation_id = ? AND status IN ?", allocationID, []int{
			model.RefundStatusPending, model.RefundStatusSucceeded, model.RefundStatusUnknown,
		})
	if excludeRefundID > 0 {
		query = query.Where("id <> ?", excludeRefundID)
	}
	var amount int64
	if err := query.Select("COALESCE(SUM(amount), 0)").Scan(&amount).Error; err != nil {
		return 0, err
	}
	return amount, nil
}

func (r *afterSaleRepository) IncreaseAllocationRefundedAmount(ctx context.Context, allocationID int64, amount int64) error {
	result := r.db.WithContext(ctx).Model(&model.PaymentAllocation{}).
		Where("id = ? AND refunded_amount + ? <= amount", allocationID, amount).
		UpdateColumn("refunded_amount", gorm.Expr("refunded_amount + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("退款金额超过子订单剩余可退金额")
	}
	return nil
}

func (r *afterSaleRepository) UpdatePaymentRefundStatus(ctx context.Context, paymentID int64, status int) error {
	return r.db.WithContext(ctx).Model(&model.Payment{}).Where("id = ?", paymentID).Updates(map[string]interface{}{
		"status":          status,
		"active_order_id": nil,
		"active_trade_id": nil,
	}).Error
}

func (r *afterSaleRepository) UpdateTradeRefundStatus(ctx context.Context, tradeID int64, status int) error {
	result := r.db.WithContext(ctx).Model(&model.Trade{}).
		Where("id = ? AND status IN ?", tradeID, []int{
			model.TradeStatusPaid, model.TradeStatusPartiallyRefunded, model.TradeStatusRefunded,
		}).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("交易退款状态已变更")
	}
	return nil
}

func (r *afterSaleRepository) UpdateStatus(ctx context.Context, id int64, currentStatuses []int, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.AfterSale{}).Where("id = ? AND status IN ?", id, currentStatuses).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("售后状态已变更")
	}
	return nil
}

func (r *afterSaleRepository) ListByUserID(ctx context.Context, userID int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	return r.list(ctx, "user_id", userID, offset, limit, status)
}

func (r *afterSaleRepository) ListByMerchantID(ctx context.Context, merchantID int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	return r.list(ctx, "merchant_id", merchantID, offset, limit, status)
}

func (r *afterSaleRepository) list(ctx context.Context, field string, id int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	var values []model.AfterSale
	var total int64
	query := r.db.WithContext(ctx).Model(&model.AfterSale{}).Where(field+" = ?", id)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&values).Error
	return values, total, err
}
