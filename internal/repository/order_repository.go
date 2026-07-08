package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type OrderRepository interface {
	Transaction(ctx context.Context, fn func(repo OrderRepository) error) error

	Create(ctx context.Context, order *model.Order) error
	CreateItems(ctx context.Context, items []model.OrderItem) error
	Update(ctx context.Context, order *model.Order) error
	UpdateStatus(ctx context.Context, id int64, userID int64, currentStatus int, nextStatus int, paidAt *time.Time, cancelledAt *time.Time) error

	FindByIDAndUserID(ctx context.Context, id int64, userID int64) (*model.Order, error)
	FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error)
	ListByUserID(ctx context.Context, userID int64, offset int, limit int, status int) ([]model.Order, int64, error)

	DecreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) error
	IncreaseSKUStock(ctx context.Context, skuID int64, quantity int) error
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

func (r *orderRepository) FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	var items []model.OrderItem

	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&items).Error

	return items, err
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

func (r *orderRepository) DecreaseSKUStock(ctx context.Context, merchantID int64, skuID int64, quantity int) error {
	result := r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Where("merchant_id = ? AND id = ? AND stock >= ?", merchantID, skuID, quantity).
		UpdateColumn("stock", gorm.Expr("stock - ?", quantity))

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("库存不足")
	}

	return nil
}

func (r *orderRepository) IncreaseSKUStock(ctx context.Context, skuID int64, quantity int) error {
	return r.db.WithContext(ctx).
		Model(&model.ProductSKU{}).
		Where("id = ?", skuID).
		UpdateColumn("stock", gorm.Expr("stock + ?", quantity)).
		Error
}
