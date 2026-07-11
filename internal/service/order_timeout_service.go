package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type PaymentTimeoutCoordinator interface {
	PrepareOrderForTimeoutCancel(ctx context.Context, orderID int64) (paid bool, err error)
}

type OrderTimeoutReport struct {
	Scanned   int
	Cancelled int
	Paid      int
	Skipped   int
	Failed    int
}

type OrderTimeoutService interface {
	Run(ctx context.Context, now time.Time) (OrderTimeoutReport, error)
}

type orderTimeoutService struct {
	repo               repository.OrderTimeoutRepository
	paymentCoordinator PaymentTimeoutCoordinator
	timeout            time.Duration
	batchSize          int
}

func NewOrderTimeoutService(
	repo repository.OrderTimeoutRepository,
	paymentCoordinator PaymentTimeoutCoordinator,
	cfg config.OrderConfig,
) OrderTimeoutService {
	timeoutMinutes := cfg.PendingPaymentTimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = 15
	}
	batchSize := cfg.CancelBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	return &orderTimeoutService{
		repo:               repo,
		paymentCoordinator: paymentCoordinator,
		timeout:            time.Duration(timeoutMinutes) * time.Minute,
		batchSize:          batchSize,
	}
}

func (s *orderTimeoutService) Run(ctx context.Context, now time.Time) (OrderTimeoutReport, error) {
	var report OrderTimeoutReport
	orderIDs, err := s.repo.ListExpiredPendingOrderIDs(ctx, now.Add(-s.timeout), s.batchSize)
	if err != nil {
		return report, err
	}
	report.Scanned = len(orderIDs)

	var failures []error
	for _, orderID := range orderIDs {
		paid, err := s.paymentCoordinator.PrepareOrderForTimeoutCancel(ctx, orderID)
		if err != nil {
			report.Failed++
			failures = append(failures, fmt.Errorf("订单 %d 支付状态确认失败: %w", orderID, err))
			continue
		}
		if paid {
			report.Paid++
			continue
		}

		cancelled, err := s.cancelOne(ctx, orderID, now)
		if err != nil {
			report.Failed++
			failures = append(failures, fmt.Errorf("订单 %d 取消失败: %w", orderID, err))
			continue
		}
		if cancelled {
			report.Cancelled++
		} else {
			report.Skipped++
		}
	}

	return report, errors.Join(failures...)
}

func (s *orderTimeoutService) cancelOne(ctx context.Context, orderID int64, now time.Time) (bool, error) {
	cancelled := false
	err := s.repo.Transaction(ctx, func(repo repository.OrderTimeoutRepository) error {
		order, err := repo.FindOrderForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status != model.OrderStatusPendingPayment || order.CreatedAt.After(now.Add(-s.timeout)) {
			return nil
		}

		items, err := repo.FindItemsByOrderID(ctx, order.ID)
		if err != nil {
			return err
		}
		inventoryLogs := make([]model.InventoryLog, 0, len(items))
		for _, item := range items {
			afterStock, err := repo.RestoreSKUStock(ctx, order.MerchantID, item.SKUID, item.Quantity)
			if err != nil {
				return err
			}
			inventoryLogs = append(inventoryLogs, model.InventoryLog{
				MerchantID:    order.MerchantID,
				ProductID:     item.ProductID,
				SKUID:         item.SKUID,
				ProductName:   item.ProductName,
				SKUName:       item.SKUName,
				ChangeType:    model.InventoryChangeOrderTimeout,
				Quantity:      item.Quantity,
				BeforeStock:   afterStock - item.Quantity,
				AfterStock:    afterStock,
				ReferenceType: model.InventoryReferenceOrder,
				ReferenceID:   order.ID,
				OperatorType:  model.InventoryOperatorSystem,
				OperatorID:    0,
				Remark:        "超时取消订单恢复库存",
			})
		}
		if err := repo.ClosePendingPayments(ctx, order.ID, now); err != nil {
			return err
		}
		if err := repo.MarkOrderCancelled(ctx, order.ID, now); err != nil {
			return err
		}
		if err := repo.CreateInventoryLogs(ctx, inventoryLogs); err != nil {
			return err
		}
		cancelled = true
		return nil
	})
	return cancelled, err
}
