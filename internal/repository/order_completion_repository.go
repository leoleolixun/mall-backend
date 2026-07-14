package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type OrderCompletionRepository interface {
	ListExpiredShippedOrderIDs(ctx context.Context, before time.Time, limit int) ([]int64, error)
	CompleteShippedOrder(ctx context.Context, orderID int64, completedAt time.Time) (bool, error)
}

type orderCompletionRepository struct{ db *gorm.DB }

func NewOrderCompletionRepository(db *gorm.DB) OrderCompletionRepository {
	return &orderCompletionRepository{db: db}
}
func (r *orderCompletionRepository) ListExpiredShippedOrderIDs(ctx context.Context, before time.Time, limit int) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Table("orders").Select("orders.id").Joins("JOIN shipments ON shipments.order_id = orders.id").Where("orders.status = ? AND shipments.shipped_at <= ?", model.OrderStatusShipped, before).Order("shipments.shipped_at ASC, orders.id ASC").Limit(limit).Scan(&ids).Error
	return ids, err
}
func (r *orderCompletionRepository) CompleteShippedOrder(ctx context.Context, orderID int64, completedAt time.Time) (bool, error) {
	completed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Order{}).Where("id = ? AND status = ?", orderID, model.OrderStatusShipped).Updates(map[string]interface{}{"status": model.OrderStatusCompleted, "completed_at": &completedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		shipment := tx.Model(&model.Shipment{}).Where("order_id = ? AND received_at IS NULL", orderID).Update("received_at", &completedAt)
		if shipment.Error != nil {
			return shipment.Error
		}
		if shipment.RowsAffected == 0 {
			return fmt.Errorf("订单 %d 缺少有效发货记录", orderID)
		}
		completed = true
		return nil
	})
	return completed, err
}
