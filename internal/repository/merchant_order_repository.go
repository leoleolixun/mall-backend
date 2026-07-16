package repository

import (
	"context"
	"fmt"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchantOrderRepository interface {
	Transaction(ctx context.Context, fn func(repo MerchantOrderRepository) error) error
	ListByMerchantID(ctx context.Context, merchantID int64, offset int, limit int, status int, keyword string) ([]model.Order, int64, error)
	FindByIDAndMerchantID(ctx context.Context, orderID int64, merchantID int64) (*model.Order, error)
	FindByIDAndMerchantIDForUpdate(ctx context.Context, orderID int64, merchantID int64) (*model.Order, error)
	FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error)
	FindItemsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.OrderItem, error)
	FindShipmentByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error)
	FindShipmentsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.Shipment, error)
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	CreateShipment(ctx context.Context, shipment *model.Shipment) error
	MarkShipped(ctx context.Context, orderID int64, merchantID int64) error
}

type merchantOrderRepository struct {
	db *gorm.DB
}

func NewMerchantOrderRepository(db *gorm.DB) MerchantOrderRepository {
	return &merchantOrderRepository{db: db}
}

func (r *merchantOrderRepository) Transaction(
	ctx context.Context,
	fn func(repo MerchantOrderRepository) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&merchantOrderRepository{db: tx})
	})
}

func (r *merchantOrderRepository) ListByMerchantID(
	ctx context.Context,
	merchantID int64,
	offset int,
	limit int,
	status int,
	keyword string,
) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64
	query := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("merchant_id = ?", merchantID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where(
			"order_no LIKE ? OR receiver_name LIKE ? OR receiver_phone LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *merchantOrderRepository) FindByIDAndMerchantID(
	ctx context.Context,
	orderID int64,
	merchantID int64,
) (*model.Order, error) {
	var order model.Order
	if err := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", orderID, merchantID).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *merchantOrderRepository) FindByIDAndMerchantIDForUpdate(
	ctx context.Context,
	orderID int64,
	merchantID int64,
) (*model.Order, error) {
	var order model.Order
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND merchant_id = ?", orderID, merchantID).
		First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *merchantOrderRepository) FindItemsByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	var items []model.OrderItem
	if err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *merchantOrderRepository) FindItemsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.OrderItem, error) {
	if len(orderIDs) == 0 {
		return []model.OrderItem{}, nil
	}
	var items []model.OrderItem
	if err := r.db.WithContext(ctx).
		Where("order_id IN ?", orderIDs).
		Order("order_id ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *merchantOrderRepository) FindShipmentByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error) {
	var shipment model.Shipment
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&shipment).Error; err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *merchantOrderRepository) FindShipmentsByOrderIDs(ctx context.Context, orderIDs []int64) ([]model.Shipment, error) {
	if len(orderIDs) == 0 {
		return []model.Shipment{}, nil
	}
	var shipments []model.Shipment
	if err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).Find(&shipments).Error; err != nil {
		return nil, err
	}
	return shipments, nil
}

func (r *merchantOrderRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantOrderRepository) CreateShipment(ctx context.Context, shipment *model.Shipment) error {
	return r.db.WithContext(ctx).Create(shipment).Error
}

func (r *merchantOrderRepository) MarkShipped(ctx context.Context, orderID int64, merchantID int64) error {
	result := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND merchant_id = ? AND status = ?", orderID, merchantID, model.OrderStatusPaid).
		Update("status", model.OrderStatusShipped)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("订单状态已变更")
	}
	return nil
}
