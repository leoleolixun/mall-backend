package service

import (
	"context"
	"strings"
	"testing"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
)

func newPaymentCreateService(repo *fakePaymentRepository) *paymentService {
	return NewPaymentService(repo, config.PaymentConfig{MockEnabled: true}).(*paymentService)
}

func pendingOrder() model.Order {
	return model.Order{
		ID:            2,
		OrderNo:       "ORDER002",
		UserID:        3,
		MerchantID:    1,
		Status:        model.OrderStatusPendingPayment,
		PayableAmount: 100,
	}
}

func TestPaymentCreateSetsActiveOrderAndReusesPendingIntent(t *testing.T) {
	repo := &fakePaymentRepository{order: pendingOrder()}
	service := newPaymentCreateService(repo)
	req := dto.CreatePaymentRequest{
		OrderID:    repo.order.ID,
		PayChannel: model.PayChannelMock,
		PayScene:   model.PaySceneMock,
	}

	first, err := service.Create(context.Background(), repo.order.UserID, req)
	if err != nil {
		t.Fatalf("create payment returned error: %v", err)
	}
	if repo.createCalls != 1 || repo.payment.ActiveOrderID == nil || *repo.payment.ActiveOrderID != repo.order.ID {
		t.Fatalf("payment did not occupy its order: calls=%d payment=%+v", repo.createCalls, repo.payment)
	}

	second, err := service.Create(context.Background(), repo.order.UserID, req)
	if err != nil {
		t.Fatalf("reuse payment returned error: %v", err)
	}
	if repo.createCalls != 1 || second.PaymentNo != first.PaymentNo {
		t.Fatalf("duplicate request created another payment: calls=%d first=%+v second=%+v", repo.createCalls, first, second)
	}
}

func TestPaymentCreateRejectsDifferentPendingPaymentMethod(t *testing.T) {
	order := pendingOrder()
	activeID := order.ID
	repo := &fakePaymentRepository{
		order: order,
		payment: model.Payment{
			ID:            1,
			PaymentNo:     "PAY001",
			OrderID:       paymentTestInt64(order.ID),
			ActiveOrderID: &activeID,
			OrderNo:       order.OrderNo,
			UserID:        order.UserID,
			MerchantID:    paymentTestInt64(order.MerchantID),
			PayChannel:    model.PayChannelAlipay,
			PayScene:      model.PaySceneAlipayPage,
			Status:        model.PaymentStatusPending,
			Amount:        order.PayableAmount,
		},
	}

	_, err := newPaymentCreateService(repo).Create(context.Background(), order.UserID, dto.CreatePaymentRequest{
		OrderID:    order.ID,
		PayChannel: model.PayChannelMock,
	})
	if err == nil || !strings.Contains(err.Error(), "已有其他支付方式的待支付单") {
		t.Fatalf("expected payment method conflict, got: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("conflicting payment method created a payment: %d", repo.createCalls)
	}
}

func TestPaymentCreateRejectsDuplicatePendingPayments(t *testing.T) {
	order := pendingOrder()
	repo := &fakePaymentRepository{
		order: order,
		pendingPayments: []model.Payment{
			{ID: 1, OrderID: paymentTestInt64(order.ID), UserID: order.UserID, Status: model.PaymentStatusPending},
			{ID: 2, OrderID: paymentTestInt64(order.ID), UserID: order.UserID, Status: model.PaymentStatusPending},
		},
	}

	_, err := newPaymentCreateService(repo).Create(context.Background(), order.UserID, dto.CreatePaymentRequest{
		OrderID:    order.ID,
		PayChannel: model.PayChannelMock,
	})
	if err == nil || !strings.Contains(err.Error(), "多个待支付单") {
		t.Fatalf("expected duplicate pending payments to be rejected, got: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("duplicate pending state created a payment: %d", repo.createCalls)
	}
}

func TestMockCompleteRejectsNonMockPayment(t *testing.T) {
	order := pendingOrder()
	activeID := order.ID
	repo := &fakePaymentRepository{
		order: order,
		payment: model.Payment{
			ID:            1,
			PaymentNo:     "PAY001",
			OrderID:       paymentTestInt64(order.ID),
			ActiveOrderID: &activeID,
			OrderNo:       order.OrderNo,
			UserID:        order.UserID,
			MerchantID:    paymentTestInt64(order.MerchantID),
			PayChannel:    model.PayChannelAlipay,
			PayScene:      model.PaySceneAlipayPage,
			Status:        model.PaymentStatusPending,
			Amount:        order.PayableAmount,
		},
	}

	_, err := newPaymentCreateService(repo).MockComplete(context.Background(), order.UserID, repo.payment.PaymentNo)
	if err == nil || !strings.Contains(err.Error(), "只有模拟支付单") {
		t.Fatalf("expected non-mock payment to be rejected, got: %v", err)
	}
	if repo.payment.Status != model.PaymentStatusPending || repo.order.Status != model.OrderStatusPendingPayment {
		t.Fatalf("non-mock payment state changed: payment=%+v order=%+v", repo.payment, repo.order)
	}
}

func newPendingTradePaymentRepo() *fakePaymentRepository {
	tradeID := int64(10)
	return &fakePaymentRepository{
		trade: model.Trade{
			ID: tradeID, TradeNo: "TRADE010", UserID: 3,
			Status: model.TradeStatusPendingPayment, PayableAmount: 300,
		},
		tradeOrders: []model.Order{
			{ID: 21, TradeID: &tradeID, UserID: 3, MerchantID: 1, Status: model.OrderStatusPendingPayment, PayableAmount: 100},
			{ID: 22, TradeID: &tradeID, UserID: 3, MerchantID: 2, Status: model.OrderStatusPendingPayment, PayableAmount: 200},
		},
	}
}

func TestPaymentCreateTradeCreatesImmutableAllocationsAndReusesIntent(t *testing.T) {
	repo := newPendingTradePaymentRepo()
	service := newPaymentCreateService(repo)
	req := dto.CreatePaymentRequest{TradeID: repo.trade.ID, PayChannel: model.PayChannelMock}

	first, err := service.Create(context.Background(), repo.trade.UserID, req)
	if err != nil {
		t.Fatalf("create trade payment returned error: %v", err)
	}
	if repo.payment.TradeID == nil || *repo.payment.TradeID != repo.trade.ID || repo.payment.OrderID != nil ||
		repo.payment.ActiveTradeID == nil || *repo.payment.ActiveTradeID != repo.trade.ID {
		t.Fatalf("payment does not use trade as aggregate root: %+v", repo.payment)
	}
	if len(first.Allocations) != 2 || len(repo.allocations) != 2 {
		t.Fatalf("unexpected allocations: response=%+v stored=%+v", first.Allocations, repo.allocations)
	}
	var total int64
	for _, allocation := range repo.allocations {
		total += allocation.Amount
	}
	if total != repo.trade.PayableAmount {
		t.Fatalf("allocation total %d does not equal trade amount %d", total, repo.trade.PayableAmount)
	}

	second, err := service.Create(context.Background(), repo.trade.UserID, req)
	if err != nil {
		t.Fatalf("reuse trade payment returned error: %v", err)
	}
	if repo.createCalls != 1 || second.PaymentNo != first.PaymentNo {
		t.Fatalf("duplicate trade request created another payment: calls=%d first=%+v second=%+v", repo.createCalls, first, second)
	}
}

func TestMockCompleteTradePaysTradeAndAllChildOrders(t *testing.T) {
	repo := newPendingTradePaymentRepo()
	service := newPaymentCreateService(repo)
	created, err := service.Create(context.Background(), repo.trade.UserID, dto.CreatePaymentRequest{
		TradeID: repo.trade.ID, PayChannel: model.PayChannelMock,
	})
	if err != nil {
		t.Fatalf("create trade payment: %v", err)
	}

	paid, err := service.MockComplete(context.Background(), repo.trade.UserID, created.PaymentNo)
	if err != nil {
		t.Fatalf("complete trade payment: %v", err)
	}
	if paid.Status != model.PaymentStatusPaid || repo.trade.Status != model.TradeStatusPaid || repo.payment.ActiveTradeID != nil {
		t.Fatalf("trade payment did not converge: response=%+v payment=%+v trade=%+v", paid, repo.payment, repo.trade)
	}
	for _, order := range repo.tradeOrders {
		if order.Status != model.OrderStatusPaid {
			t.Fatalf("child order was not paid: %+v", order)
		}
	}
}

func TestLegacyOrderPaymentMapsSingleOrderTradeToAggregatePayment(t *testing.T) {
	tradeID := int64(10)
	order := pendingOrder()
	order.TradeID = &tradeID
	repo := &fakePaymentRepository{
		order:       order,
		trade:       model.Trade{ID: tradeID, UserID: order.UserID, Status: model.TradeStatusPendingPayment, PayableAmount: order.PayableAmount},
		tradeOrders: []model.Order{order},
	}

	payment, err := newPaymentCreateService(repo).Create(context.Background(), order.UserID, dto.CreatePaymentRequest{
		OrderID: order.ID, PayChannel: model.PayChannelMock,
	})
	if err != nil {
		t.Fatalf("single-order compatibility payment returned error: %v", err)
	}
	if payment.TradeID == nil || *payment.TradeID != tradeID || len(payment.Allocations) != 1 || payment.Allocations[0].OrderID != order.ID {
		t.Fatalf("legacy order payment was not mapped to aggregate payment: %+v", payment)
	}
}

func TestLegacyOrderPaymentRejectsMultiMerchantTradeChildOrder(t *testing.T) {
	repo := newPendingTradePaymentRepo()
	order := repo.tradeOrders[0]
	repo.order = order

	_, err := newPaymentCreateService(repo).Create(context.Background(), order.UserID, dto.CreatePaymentRequest{
		OrderID: order.ID, PayChannel: model.PayChannelMock,
	})
	if err == nil || !strings.Contains(err.Error(), "多商户交易") {
		t.Fatalf("expected multi-merchant child payment to be rejected, got: %v", err)
	}
}

func TestPaymentCreateRequiresExactlyOneAggregateID(t *testing.T) {
	repo := &fakePaymentRepository{order: pendingOrder()}
	service := newPaymentCreateService(repo)
	for _, req := range []dto.CreatePaymentRequest{
		{PayChannel: model.PayChannelMock},
		{OrderID: 2, TradeID: 10, PayChannel: model.PayChannelMock},
	} {
		if _, err := service.Create(context.Background(), 3, req); err == nil || !strings.Contains(err.Error(), "必须且只能填写一个") {
			t.Fatalf("expected aggregate id validation error, got: %v", err)
		}
	}
}
