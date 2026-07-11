package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type fakeOrderTimeoutRepository struct {
	order              model.Order
	items              []model.OrderItem
	restored           map[int64]int
	closePaymentCalls  int
	markCancelledCalls int
	listedOrderIDs     []int64
	inventoryLogs      []model.InventoryLog
}

func (r *fakeOrderTimeoutRepository) Transaction(_ context.Context, fn func(repo repository.OrderTimeoutRepository) error) error {
	return fn(r)
}

func (r *fakeOrderTimeoutRepository) ListExpiredPendingOrderIDs(context.Context, time.Time, int) ([]int64, error) {
	return append([]int64(nil), r.listedOrderIDs...), nil
}

func (r *fakeOrderTimeoutRepository) FindOrderForUpdate(_ context.Context, orderID int64) (*model.Order, error) {
	if r.order.ID != orderID {
		return nil, fmt.Errorf("not found")
	}
	copy := r.order
	return &copy, nil
}

func (r *fakeOrderTimeoutRepository) FindItemsByOrderID(_ context.Context, orderID int64) ([]model.OrderItem, error) {
	return append([]model.OrderItem(nil), r.items...), nil
}

func (r *fakeOrderTimeoutRepository) RestoreSKUStock(_ context.Context, _ int64, skuID int64, quantity int) (int, error) {
	r.restored[skuID] += quantity
	return 10 + r.restored[skuID], nil
}

func (r *fakeOrderTimeoutRepository) CreateInventoryLogs(_ context.Context, logs []model.InventoryLog) error {
	r.inventoryLogs = append(r.inventoryLogs, logs...)
	return nil
}

func (r *fakeOrderTimeoutRepository) ClosePendingPayments(_ context.Context, _ int64, _ time.Time) error {
	r.closePaymentCalls++
	return nil
}

func (r *fakeOrderTimeoutRepository) MarkOrderCancelled(_ context.Context, _ int64, cancelledAt time.Time) error {
	r.markCancelledCalls++
	r.order.Status = model.OrderStatusCancelled
	r.order.CancelledAt = &cancelledAt
	return nil
}

type fakePaymentTimeoutCoordinator struct {
	paid bool
	err  error
}

func (c fakePaymentTimeoutCoordinator) PrepareOrderForTimeoutCancel(context.Context, int64) (bool, error) {
	return c.paid, c.err
}

func newOrderTimeoutServiceForTest(now time.Time, coordinator PaymentTimeoutCoordinator) (*orderTimeoutService, *fakeOrderTimeoutRepository) {
	repo := &fakeOrderTimeoutRepository{
		order: model.Order{
			ID: 9, MerchantID: 1, Status: model.OrderStatusPendingPayment,
			CreatedAt: now.Add(-20 * time.Minute),
		},
		items:          []model.OrderItem{{OrderID: 9, SKUID: 3, Quantity: 2}},
		restored:       map[int64]int{},
		listedOrderIDs: []int64{9},
	}
	service := NewOrderTimeoutService(repo, coordinator, config.OrderConfig{
		PendingPaymentTimeoutMinutes: 15,
		CancelBatchSize:              100,
	}).(*orderTimeoutService)
	return service, repo
}

func TestOrderTimeoutCancelsOrderAndRestoresStock(t *testing.T) {
	now := time.Now()
	service, repo := newOrderTimeoutServiceForTest(now, fakePaymentTimeoutCoordinator{})
	report, err := service.Run(context.Background(), now)
	if err != nil {
		t.Fatalf("timeout run returned error: %v", err)
	}
	if report.Cancelled != 1 || repo.order.Status != model.OrderStatusCancelled || repo.restored[3] != 2 {
		t.Fatalf("unexpected timeout result: report=%+v repo=%+v", report, repo)
	}
	if repo.closePaymentCalls != 1 || repo.markCancelledCalls != 1 {
		t.Fatalf("expected payment close and order cancel once: %+v", repo)
	}
	if len(repo.inventoryLogs) != 1 || repo.inventoryLogs[0].ChangeType != model.InventoryChangeOrderTimeout {
		t.Fatalf("unexpected inventory logs: %+v", repo.inventoryLogs)
	}
}

func TestOrderTimeoutSkipsOrderPaidDuringReconciliation(t *testing.T) {
	now := time.Now()
	service, repo := newOrderTimeoutServiceForTest(now, fakePaymentTimeoutCoordinator{paid: true})
	report, err := service.Run(context.Background(), now)
	if err != nil {
		t.Fatalf("timeout run returned error: %v", err)
	}
	if report.Paid != 1 || report.Cancelled != 0 || repo.markCancelledCalls != 0 {
		t.Fatalf("unexpected paid result: report=%+v repo=%+v", report, repo)
	}
}

func TestOrderTimeoutReportsPaymentQueryFailure(t *testing.T) {
	now := time.Now()
	service, repo := newOrderTimeoutServiceForTest(now, fakePaymentTimeoutCoordinator{err: fmt.Errorf("gateway unavailable")})
	report, err := service.Run(context.Background(), now)
	if err == nil || report.Failed != 1 || repo.markCancelledCalls != 0 {
		t.Fatalf("unexpected failure result: report=%+v err=%v", report, err)
	}
}
