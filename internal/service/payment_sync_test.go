package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakePaymentRepository struct {
	payment model.Payment
	order   model.Order
}

func (r *fakePaymentRepository) Transaction(_ context.Context, fn func(repo repository.PaymentRepository) error) error {
	return fn(r)
}

func (r *fakePaymentRepository) Create(_ context.Context, payment *model.Payment) error {
	r.payment = *payment
	return nil
}

func (r *fakePaymentRepository) FindByPaymentNo(_ context.Context, paymentNo string) (*model.Payment, error) {
	if r.payment.PaymentNo != paymentNo {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.payment
	return &copy, nil
}

func (r *fakePaymentRepository) FindByPaymentNoAndUserID(_ context.Context, paymentNo string, userID int64) (*model.Payment, error) {
	if r.payment.PaymentNo != paymentNo || r.payment.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.payment
	return &copy, nil
}

func (r *fakePaymentRepository) FindLatestByOrderIDUserIDChannelScene(
	_ context.Context,
	orderID int64,
	userID int64,
	payChannel string,
	payScene string,
) (*model.Payment, error) {
	if r.payment.OrderID != orderID || r.payment.UserID != userID || r.payment.PayChannel != payChannel || r.payment.PayScene != payScene {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.payment
	return &copy, nil
}

func (r *fakePaymentRepository) FindPendingByOrderID(_ context.Context, orderID int64) ([]model.Payment, error) {
	if r.payment.OrderID == orderID && r.payment.Status == model.PaymentStatusPending {
		return []model.Payment{r.payment}, nil
	}
	return []model.Payment{}, nil
}

func (r *fakePaymentRepository) FindPaidByOrderID(_ context.Context, orderID int64) (*model.Payment, error) {
	if r.payment.OrderID == orderID && r.payment.Status == model.PaymentStatusPaid {
		copy := r.payment
		return &copy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakePaymentRepository) MarkPaid(_ context.Context, id int64, userID int64, transactionID string, paidAt time.Time) error {
	if r.payment.ID != id || r.payment.UserID != userID || r.payment.Status != model.PaymentStatusPending {
		return fmt.Errorf("payment state changed")
	}
	r.payment.Status = model.PaymentStatusPaid
	r.payment.TransactionID = transactionID
	r.payment.PaidAt = &paidAt
	return nil
}

func (r *fakePaymentRepository) MarkClosed(_ context.Context, id int64, userID int64, closedAt time.Time) error {
	if r.payment.ID != id || r.payment.UserID != userID || r.payment.Status != model.PaymentStatusPending {
		return fmt.Errorf("payment state changed")
	}
	r.payment.Status = model.PaymentStatusClosed
	r.payment.ClosedAt = &closedAt
	return nil
}

func (r *fakePaymentRepository) ClosePendingByOrderID(_ context.Context, orderID int64, closedAt time.Time) error {
	if r.payment.OrderID == orderID && r.payment.Status == model.PaymentStatusPending {
		r.payment.Status = model.PaymentStatusClosed
		r.payment.ClosedAt = &closedAt
	}
	return nil
}

func (r *fakePaymentRepository) FindOrderByIDAndUserID(_ context.Context, orderID int64, userID int64) (*model.Order, error) {
	if r.order.ID != orderID || r.order.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.order
	return &copy, nil
}

func (r *fakePaymentRepository) UpdateOrderStatus(
	_ context.Context,
	orderID int64,
	userID int64,
	currentStatus int,
	nextStatus int,
	paidAt *time.Time,
) error {
	if r.order.ID != orderID || r.order.UserID != userID || r.order.Status != currentStatus {
		return fmt.Errorf("order state changed")
	}
	r.order.Status = nextStatus
	r.order.PaidAt = paidAt
	return nil
}

type fakeAlipayGateway struct {
	result     alipayTradeResult
	queryErr   error
	closeErr   error
	closeCalls int
}

func (g *fakeAlipayGateway) Query(context.Context, model.Payment) (*alipayTradeResult, error) {
	if g.queryErr != nil {
		return nil, g.queryErr
	}
	copy := g.result
	return &copy, nil
}

func (g *fakeAlipayGateway) Close(context.Context, model.Payment) error {
	g.closeCalls++
	return g.closeErr
}

func newPaymentSyncService(state alipayTradeState) (*paymentService, *fakePaymentRepository, *fakeAlipayGateway) {
	repo := &fakePaymentRepository{
		payment: model.Payment{
			ID: 1, PaymentNo: "P001", OrderID: 2, UserID: 3, MerchantID: 1,
			PayChannel: model.PayChannelAlipay, PayScene: model.PaySceneAlipayPage,
			Status: model.PaymentStatusPending, Amount: 100,
		},
		order: model.Order{ID: 2, UserID: 3, Status: model.OrderStatusPendingPayment},
	}
	gateway := &fakeAlipayGateway{result: alipayTradeResult{
		State: state, TransactionID: "ALI001", PaidAt: time.Now(),
	}}
	service := NewPaymentService(repo, config.PaymentConfig{}).(*paymentService)
	service.alipay = gateway
	return service, repo, gateway
}

func TestPaymentSyncMarksAlipayTradePaid(t *testing.T) {
	service, repo, _ := newPaymentSyncService(alipayTradeStatePaid)
	result, err := service.Sync(context.Background(), 3, "P001")
	if err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	if result.Status != model.PaymentStatusPaid || repo.order.Status != model.OrderStatusPaid || result.TransactionID != "ALI001" {
		t.Fatalf("unexpected synchronized state: payment=%+v order=%+v", result, repo.order)
	}
}

func TestPaymentSyncMarksClosedTradeClosed(t *testing.T) {
	service, repo, _ := newPaymentSyncService(alipayTradeStateClosed)
	result, err := service.Sync(context.Background(), 3, "P001")
	if err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	if result.Status != model.PaymentStatusClosed || repo.order.Status != model.OrderStatusPendingPayment {
		t.Fatalf("unexpected closed state: payment=%+v order=%+v", result, repo.order)
	}
}

func TestPrepareTimeoutClosesWaitingAlipayTrade(t *testing.T) {
	service, _, gateway := newPaymentSyncService(alipayTradeStateWaiting)
	paid, err := service.PrepareOrderForTimeoutCancel(context.Background(), 2)
	if err != nil {
		t.Fatalf("prepare timeout returned error: %v", err)
	}
	if paid || gateway.closeCalls != 1 {
		t.Fatalf("unexpected timeout preparation: paid=%v close_calls=%d", paid, gateway.closeCalls)
	}
}

func TestPrepareTimeoutDefersLocalCloseToOrderTransaction(t *testing.T) {
	service, repo, gateway := newPaymentSyncService(alipayTradeStateClosed)
	paid, err := service.PrepareOrderForTimeoutCancel(context.Background(), 2)
	if err != nil {
		t.Fatalf("prepare timeout returned error: %v", err)
	}
	if paid || gateway.closeCalls != 0 || repo.payment.Status != model.PaymentStatusPending {
		t.Fatalf("local payment closed before order transaction: paid=%v payment=%+v", paid, repo.payment)
	}
}

func TestPrepareTimeoutKeepsPaidAlipayOrder(t *testing.T) {
	service, repo, gateway := newPaymentSyncService(alipayTradeStatePaid)
	paid, err := service.PrepareOrderForTimeoutCancel(context.Background(), 2)
	if err != nil {
		t.Fatalf("prepare timeout returned error: %v", err)
	}
	if !paid || gateway.closeCalls != 0 || repo.order.Status != model.OrderStatusPaid {
		t.Fatalf("unexpected paid reconciliation: paid=%v close_calls=%d order=%+v", paid, gateway.closeCalls, repo.order)
	}
}

func TestPrepareTimeoutRepairsOrderWhenLocalPaymentAlreadyPaid(t *testing.T) {
	service, repo, gateway := newPaymentSyncService(alipayTradeStateUnknown)
	now := time.Now()
	repo.payment.Status = model.PaymentStatusPaid
	repo.payment.PaidAt = &now

	paid, err := service.PrepareOrderForTimeoutCancel(context.Background(), 2)
	if err != nil {
		t.Fatalf("prepare timeout returned error: %v", err)
	}
	if !paid || repo.order.Status != model.OrderStatusPaid || gateway.closeCalls != 0 {
		t.Fatalf("unexpected local payment repair: paid=%v order=%+v close_calls=%d", paid, repo.order, gateway.closeCalls)
	}
}
