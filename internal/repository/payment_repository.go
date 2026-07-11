package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type PaymentRepository interface {
	Transaction(ctx context.Context, fn func(repo PaymentRepository) error) error

	Create(ctx context.Context, payment *model.Payment) error
	FindByPaymentNo(ctx context.Context, paymentNo string) (*model.Payment, error)
	FindByPaymentNoAndUserID(ctx context.Context, paymentNo string, userID int64) (*model.Payment, error)
	FindLatestByOrderIDUserIDChannelScene(ctx context.Context, orderID int64, userID int64, payChannel string, payScene string) (*model.Payment, error)
	FindPendingByOrderID(ctx context.Context, orderID int64) ([]model.Payment, error)
	FindPaidByOrderID(ctx context.Context, orderID int64) (*model.Payment, error)
	MarkPaid(ctx context.Context, id int64, userID int64, transactionID string, paidAt time.Time) error
	MarkClosed(ctx context.Context, id int64, userID int64, closedAt time.Time) error
	ClosePendingByOrderID(ctx context.Context, orderID int64, closedAt time.Time) error
	FindOrderByIDAndUserID(ctx context.Context, orderID int64, userID int64) (*model.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int64, userID int64, currentStatus int, nextStatus int, paidAt *time.Time) error
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

func (r *paymentRepository) MarkPaid(ctx context.Context, id int64, userID int64, transactionID string, paidAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":         model.PaymentStatusPaid,
			"transaction_id": transactionID,
			"paid_at":        &paidAt,
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
			"status":    model.PaymentStatusClosed,
			"closed_at": &closedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("支付单状态已变更")
	}
	return nil
}

func (r *paymentRepository) ClosePendingByOrderID(ctx context.Context, orderID int64, closedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("order_id = ? AND status = ?", orderID, model.PaymentStatusPending).
		Updates(map[string]interface{}{
			"status":    model.PaymentStatusClosed,
			"closed_at": &closedAt,
		}).Error
}

func (r *paymentRepository) FindOrderByIDAndUserID(ctx context.Context, orderID int64, userID int64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", orderID, userID).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
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
