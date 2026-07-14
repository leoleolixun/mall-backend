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
	FindOrderByIDAndUserID(ctx context.Context, orderID, userID int64) (*model.Order, error)
	FindOrderItem(ctx context.Context, orderID, orderItemID int64) (*model.OrderItem, error)
	FindLatestByOrderItem(ctx context.Context, orderItemID int64) (*model.AfterSale, error)
	FindOrder(ctx context.Context, orderID int64) (*model.Order, error)
	FindItem(ctx context.Context, orderItemID int64) (*model.OrderItem, error)
	FindPaidPaymentByOrderID(ctx context.Context, orderID int64) (*model.Payment, error)
	FindPaymentByID(ctx context.Context, id int64) (*model.Payment, error)
	FindRefundByAfterSaleID(ctx context.Context, afterSaleID int64) (*model.Refund, error)
	CreateRefund(ctx context.Context, refund *model.Refund) error
	MarkRefundPending(ctx context.Context, refundID int64) error
	MarkRefundSucceeded(ctx context.Context, refundID int64, transactionID string, refundedAt time.Time) error
	MarkRefundFailed(ctx context.Context, refundID int64, reason string) error
	SumSucceededRefunds(ctx context.Context, paymentID int64) (int64, error)
	UpdatePaymentRefundStatus(ctx context.Context, paymentID int64, status int) error
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

func (r *afterSaleRepository) FindPaymentByID(ctx context.Context, id int64) (*model.Payment, error) {
	var value model.Payment
	err := r.db.WithContext(ctx).First(&value, id).Error
	return &value, err
}

func (r *afterSaleRepository) FindRefundByAfterSaleID(ctx context.Context, afterSaleID int64) (*model.Refund, error) {
	var value model.Refund
	err := r.db.WithContext(ctx).Where("after_sale_id = ?", afterSaleID).First(&value).Error
	return &value, err
}

func (r *afterSaleRepository) CreateRefund(ctx context.Context, refund *model.Refund) error {
	return r.db.WithContext(ctx).Create(refund).Error
}

func (r *afterSaleRepository) MarkRefundPending(ctx context.Context, refundID int64) error {
	return r.db.WithContext(ctx).Model(&model.Refund{}).Where("id = ? AND status = ?", refundID, model.RefundStatusFailed).Updates(map[string]interface{}{"status": model.RefundStatusPending, "failure_reason": ""}).Error
}

func (r *afterSaleRepository) MarkRefundSucceeded(ctx context.Context, refundID int64, transactionID string, refundedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Refund{}).Where("id = ? AND status = ?", refundID, model.RefundStatusPending).Updates(map[string]interface{}{"status": model.RefundStatusSucceeded, "transaction_id": transactionID, "refunded_at": &refundedAt, "failure_reason": ""}).Error
}

func (r *afterSaleRepository) MarkRefundFailed(ctx context.Context, refundID int64, reason string) error {
	return r.db.WithContext(ctx).Model(&model.Refund{}).Where("id = ? AND status = ?", refundID, model.RefundStatusPending).Updates(map[string]interface{}{"status": model.RefundStatusFailed, "failure_reason": reason}).Error
}

func (r *afterSaleRepository) SumSucceededRefunds(ctx context.Context, paymentID int64) (int64, error) {
	var amount int64
	err := r.db.WithContext(ctx).Model(&model.Refund{}).Where("payment_id = ? AND status = ?", paymentID, model.RefundStatusSucceeded).Select("COALESCE(SUM(amount), 0)").Scan(&amount).Error
	return amount, err
}

func (r *afterSaleRepository) UpdatePaymentRefundStatus(ctx context.Context, paymentID int64, status int) error {
	return r.db.WithContext(ctx).Model(&model.Payment{}).Where("id = ?", paymentID).Update("status", status).Error
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
