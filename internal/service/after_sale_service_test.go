package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakeAfterSaleRepository struct {
	afterSales        map[int64]model.AfterSale
	orders            map[int64]model.Order
	items             map[int64]model.OrderItem
	payments          map[int64]model.Payment
	allocations       map[int64]model.PaymentAllocation
	trades            map[int64]model.Trade
	settlementEntries []model.SettlementEntry
	refunds           map[int64]model.Refund
	nextAfterSaleID   int64
	nextRefundID      int64
	transactions      int
}

func (r *fakeAfterSaleRepository) Transaction(_ context.Context, fn func(repository.AfterSaleRepository) error) error {
	r.transactions++
	return fn(r)
}

func (r *fakeAfterSaleRepository) Create(_ context.Context, value *model.AfterSale) error {
	r.nextAfterSaleID++
	value.ID = r.nextAfterSaleID
	value.CreatedAt = time.Now()
	value.UpdatedAt = value.CreatedAt
	r.afterSales[value.ID] = *value
	return nil
}

func (r *fakeAfterSaleRepository) FindByIDAndUserID(_ context.Context, id, userID int64) (*model.AfterSale, error) {
	value, ok := r.afterSales[id]
	if !ok || value.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindByIDAndMerchantID(_ context.Context, id, merchantID int64) (*model.AfterSale, error) {
	value, ok := r.afterSales[id]
	if !ok || value.MerchantID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindForUpdateByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.AfterSale, error) {
	return r.FindByIDAndMerchantID(ctx, id, merchantID)
}

func (r *fakeAfterSaleRepository) FindAfterSaleForUpdate(_ context.Context, id int64) (*model.AfterSale, error) {
	value, ok := r.afterSales[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindOrderByIDAndUserID(_ context.Context, orderID, userID int64) (*model.Order, error) {
	value, ok := r.orders[orderID]
	if !ok || value.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindOrderItem(_ context.Context, orderID, orderItemID int64) (*model.OrderItem, error) {
	value, ok := r.items[orderItemID]
	if !ok || value.OrderID != orderID {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindLatestByOrderItem(_ context.Context, orderItemID int64) (*model.AfterSale, error) {
	var latest model.AfterSale
	found := false
	for _, value := range r.afterSales {
		if value.OrderItemID == orderItemID && (!found || value.ID > latest.ID) {
			latest, found = value, true
		}
	}
	if !found {
		return nil, gorm.ErrRecordNotFound
	}
	return &latest, nil
}

func (r *fakeAfterSaleRepository) FindOrder(_ context.Context, orderID int64) (*model.Order, error) {
	value, ok := r.orders[orderID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindOrderForUpdate(ctx context.Context, orderID int64) (*model.Order, error) {
	return r.FindOrder(ctx, orderID)
}

func (r *fakeAfterSaleRepository) FindItem(_ context.Context, orderItemID int64) (*model.OrderItem, error) {
	value, ok := r.items[orderItemID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindPaidPaymentByOrderID(_ context.Context, orderID int64) (*model.Payment, error) {
	for _, value := range r.payments {
		if value.OrderID != nil && *value.OrderID == orderID && (value.Status == model.PaymentStatusPaid || value.Status == model.PaymentStatusPartiallyRefunded) {
			payment := value
			return &payment, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeAfterSaleRepository) FindPaidPaymentAllocationByOrderID(_ context.Context, orderID int64) (*model.Payment, *model.PaymentAllocation, error) {
	for _, allocation := range r.allocations {
		if allocation.OrderID != orderID {
			continue
		}
		payment, ok := r.payments[allocation.PaymentID]
		if !ok || payment.Status != model.PaymentStatusPaid && payment.Status != model.PaymentStatusPartiallyRefunded && payment.Status != model.PaymentStatusRefunded {
			continue
		}
		allocationCopy := allocation
		paymentCopy := payment
		return &paymentCopy, &allocationCopy, nil
	}
	return nil, nil, gorm.ErrRecordNotFound
}

func (r *fakeAfterSaleRepository) FindPaymentByID(_ context.Context, id int64) (*model.Payment, error) {
	value, ok := r.payments[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindPaymentForUpdate(ctx context.Context, id int64) (*model.Payment, error) {
	return r.FindPaymentByID(ctx, id)
}

func (r *fakeAfterSaleRepository) FindPaymentAllocationByID(_ context.Context, id int64) (*model.PaymentAllocation, error) {
	value, ok := r.allocations[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindPaymentAllocationForUpdate(ctx context.Context, id int64) (*model.PaymentAllocation, error) {
	return r.FindPaymentAllocationByID(ctx, id)
}

func (r *fakeAfterSaleRepository) FindRefundByAfterSaleID(_ context.Context, afterSaleID int64) (*model.Refund, error) {
	for _, value := range r.refunds {
		if value.AfterSaleID == afterSaleID {
			refund := value
			return &refund, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeAfterSaleRepository) FindRefundByID(_ context.Context, id int64) (*model.Refund, error) {
	value, ok := r.refunds[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeAfterSaleRepository) FindRefundForUpdate(ctx context.Context, id int64) (*model.Refund, error) {
	return r.FindRefundByID(ctx, id)
}

func (r *fakeAfterSaleRepository) ListRefundIDsForReconciliation(_ context.Context, now time.Time, limit int) ([]int64, error) {
	ids := make([]int64, 0)
	for id, value := range r.refunds {
		if value.Status != model.RefundStatusPending && value.Status != model.RefundStatusUnknown {
			continue
		}
		if value.NextRetryAt != nil && value.NextRetryAt.After(now) {
			continue
		}
		ids = append(ids, id)
		if len(ids) == limit {
			break
		}
	}
	return ids, nil
}

func (r *fakeAfterSaleRepository) CreateRefund(_ context.Context, refund *model.Refund) error {
	r.nextRefundID++
	refund.ID = r.nextRefundID
	refund.CreatedAt = time.Now()
	refund.UpdatedAt = refund.CreatedAt
	r.refunds[refund.ID] = *refund
	return nil
}

func (r *fakeAfterSaleRepository) CreateSettlementEntries(_ context.Context, entries []model.SettlementEntry) error {
	for _, entry := range entries {
		found := false
		for _, existing := range r.settlementEntries {
			if existing.EntryNo == entry.EntryNo {
				found = true
				break
			}
		}
		if !found {
			entry.ID = int64(len(r.settlementEntries) + 1)
			r.settlementEntries = append(r.settlementEntries, entry)
		}
	}
	return nil
}

func (r *fakeAfterSaleRepository) MarkRefundPending(_ context.Context, refundID int64) error {
	value, ok := r.refunds[refundID]
	if !ok || value.Status != model.RefundStatusFailed {
		return fmt.Errorf("refund state changed")
	}
	value.Status = model.RefundStatusPending
	value.FailureReason = ""
	value.LastError = ""
	value.NextRetryAt = nil
	r.refunds[refundID] = value
	return nil
}

func (r *fakeAfterSaleRepository) MarkRefundSucceeded(_ context.Context, refundID int64, transactionID string, refundedAt time.Time) error {
	value, ok := r.refunds[refundID]
	if !ok || value.Status != model.RefundStatusPending && value.Status != model.RefundStatusUnknown {
		return fmt.Errorf("refund state changed")
	}
	value.Status = model.RefundStatusSucceeded
	value.TransactionID = transactionID
	value.RefundedAt = &refundedAt
	value.FailureReason = ""
	value.LastError = ""
	value.LastAttemptAt = &refundedAt
	value.NextRetryAt = nil
	r.refunds[refundID] = value
	return nil
}

func (r *fakeAfterSaleRepository) MarkRefundFailed(_ context.Context, refundID int64, reason string, attemptedAt time.Time) error {
	value, ok := r.refunds[refundID]
	if !ok || value.Status != model.RefundStatusPending && value.Status != model.RefundStatusUnknown {
		return fmt.Errorf("refund state changed")
	}
	value.Status = model.RefundStatusFailed
	value.FailureReason = reason
	value.LastError = ""
	value.LastAttemptAt = &attemptedAt
	value.NextRetryAt = nil
	r.refunds[refundID] = value
	return nil
}

func (r *fakeAfterSaleRepository) MarkRefundUnknown(_ context.Context, refundID int64, reason string, attemptedAt, nextRetryAt time.Time) error {
	value, ok := r.refunds[refundID]
	if !ok || value.Status != model.RefundStatusPending && value.Status != model.RefundStatusUnknown {
		return fmt.Errorf("refund state changed")
	}
	value.Status = model.RefundStatusUnknown
	value.FailureReason = ""
	value.LastError = reason
	value.RetryCount++
	value.LastAttemptAt = &attemptedAt
	value.NextRetryAt = &nextRetryAt
	r.refunds[refundID] = value
	return nil
}

func (r *fakeAfterSaleRepository) SumSucceededRefunds(_ context.Context, paymentID int64) (int64, error) {
	var amount int64
	for _, value := range r.refunds {
		if value.PaymentID == paymentID && value.Status == model.RefundStatusSucceeded {
			amount += value.Amount
		}
	}
	return amount, nil
}

func (r *fakeAfterSaleRepository) SumReservedRefundsByAllocation(_ context.Context, allocationID int64, excludeRefundID int64) (int64, error) {
	var amount int64
	for _, value := range r.refunds {
		if value.ID == excludeRefundID || value.PaymentAllocationID == nil || *value.PaymentAllocationID != allocationID {
			continue
		}
		if value.Status == model.RefundStatusPending || value.Status == model.RefundStatusSucceeded || value.Status == model.RefundStatusUnknown {
			amount += value.Amount
		}
	}
	return amount, nil
}

func (r *fakeAfterSaleRepository) IncreaseAllocationRefundedAmount(_ context.Context, allocationID int64, amount int64) error {
	value, ok := r.allocations[allocationID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if value.RefundedAmount+amount > value.Amount {
		return fmt.Errorf("退款金额超过子订单剩余可退金额")
	}
	value.RefundedAmount += amount
	r.allocations[allocationID] = value
	return nil
}

func (r *fakeAfterSaleRepository) UpdatePaymentRefundStatus(_ context.Context, paymentID int64, status int) error {
	value, ok := r.payments[paymentID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	value.Status = status
	r.payments[paymentID] = value
	return nil
}

func (r *fakeAfterSaleRepository) UpdateTradeRefundStatus(_ context.Context, tradeID int64, status int) error {
	value, ok := r.trades[tradeID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	value.Status = status
	r.trades[tradeID] = value
	return nil
}

func (r *fakeAfterSaleRepository) UpdateStatus(_ context.Context, id int64, currentStatuses []int, updates map[string]interface{}) error {
	value, ok := r.afterSales[id]
	if !ok || !containsInt(currentStatuses, value.Status) {
		return fmt.Errorf("售后状态已变更")
	}
	if status, ok := updates["status"].(int); ok {
		value.Status = status
	}
	if activeKey, ok := updates["active_key"].(string); ok {
		value.ActiveKey = activeKey
	}
	if reason, ok := updates["reject_reason"].(string); ok {
		value.RejectReason = reason
	}
	if reviewedBy, ok := updates["reviewed_by"].(int64); ok {
		value.ReviewedBy = reviewedBy
	}
	if reviewedAt, ok := updates["reviewed_at"].(*time.Time); ok {
		value.ReviewedAt = reviewedAt
	}
	if cancelledAt, ok := updates["cancelled_at"].(*time.Time); ok {
		value.CancelledAt = cancelledAt
	}
	if refundedAt, ok := updates["refunded_at"].(*time.Time); ok {
		value.RefundedAt = refundedAt
	}
	value.UpdatedAt = time.Now()
	r.afterSales[id] = value
	return nil
}

func (r *fakeAfterSaleRepository) ListByUserID(_ context.Context, userID int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	return r.list("user", userID, offset, limit, status)
}

func (r *fakeAfterSaleRepository) ListByMerchantID(_ context.Context, merchantID int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	return r.list("merchant", merchantID, offset, limit, status)
}

func (r *fakeAfterSaleRepository) list(scope string, id int64, offset, limit, status int) ([]model.AfterSale, int64, error) {
	values := make([]model.AfterSale, 0)
	for _, value := range r.afterSales {
		matchesScope := scope == "user" && value.UserID == id || scope == "merchant" && value.MerchantID == id
		if matchesScope && (status <= 0 || value.Status == status) {
			values = append(values, value)
		}
	}
	total := int64(len(values))
	if offset >= len(values) {
		return []model.AfterSale{}, total, nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return values[offset:end], total, nil
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func newAfterSaleServiceForTest() (*afterSaleService, *fakeAfterSaleRepository) {
	now := time.Now().Truncate(time.Second)
	tradeID := int64(70)
	commissionRate := 1000
	commissionAmount := int64(800)
	settlementAmount := int64(7200)
	repo := &fakeAfterSaleRepository{
		afterSales: make(map[int64]model.AfterSale),
		orders: map[int64]model.Order{
			10: {ID: 10, TradeID: &tradeID, OrderNo: "ORDER10", UserID: 7, MerchantID: 1, Status: model.OrderStatusPaid, GoodsAmount: 10000, DiscountAmount: 2000, PayableAmount: 8000, CommissionRateBPS: &commissionRate, CommissionAmount: &commissionAmount, SettlementAmount: &settlementAmount, CreatedAt: now, UpdatedAt: now},
		},
		items: map[int64]model.OrderItem{
			20: {ID: 20, OrderID: 10, ProductID: 30, SKUID: 40, ProductName: "测试商品", SKUName: "标准款", Price: 4000, Quantity: 1, Subtotal: 4000, DiscountAmount: 800, PayableAmount: 3200, CreatedAt: now, UpdatedAt: now},
		},
		payments: map[int64]model.Payment{
			50: {ID: 50, TradeID: &tradeID, PaymentNo: "PAY50", OrderID: paymentTestInt64(10), OrderNo: "ORDER10", UserID: 7, MerchantID: paymentTestInt64(1), PayChannel: model.PayChannelMock, Status: model.PaymentStatusPaid, Amount: 8000, CreatedAt: now, UpdatedAt: now},
		},
		allocations: map[int64]model.PaymentAllocation{
			60: {ID: 60, PaymentID: 50, TradeID: tradeID, OrderID: 10, MerchantID: 1, Amount: 8000},
		},
		trades: map[int64]model.Trade{
			tradeID: {ID: tradeID, UserID: 7, Status: model.TradeStatusPaid, PayableAmount: 8000},
		},
		refunds:         make(map[int64]model.Refund),
		nextAfterSaleID: 100,
		nextRefundID:    200,
	}
	return NewAfterSaleService(repo, config.PaymentConfig{}).(*afterSaleService), repo
}

func TestAfterSaleCreateUsesPersistedItemPayableAmount(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	result, err := service.Create(context.Background(), 7, dto.CreateAfterSaleRequest{
		OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly,
		Reason: "商品存在问题", Description: "申请退款", Images: []string{"https://example.com/evidence.jpg"},
	})
	if err != nil {
		t.Fatalf("create after-sale: %v", err)
	}
	if result.RefundAmount != 3200 || result.Status != model.AfterSaleStatusPending {
		t.Fatalf("unexpected after-sale response: %+v", result)
	}
	if repo.afterSales[result.ID].ActiveKey != "10:20" || len(result.Images) != 1 {
		t.Fatalf("unexpected stored after-sale: %+v", repo.afterSales[result.ID])
	}
	if _, err := service.Create(context.Background(), 8, dto.CreateAfterSaleRequest{OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly, Reason: "越权申请"}); err == nil {
		t.Fatal("expected another user to be rejected")
	}
}

func TestAfterSaleCreateUsesRoundingSnapshotInsteadOfRecalculatingRatio(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	repo.orders[10] = model.Order{
		ID: 10, OrderNo: "ORDER10", UserID: 7, MerchantID: 1, Status: model.OrderStatusPaid,
		GoodsAmount: 3, DiscountAmount: 1, PayableAmount: 2,
	}
	repo.items[20] = model.OrderItem{
		ID: 20, OrderID: 10, ProductID: 30, SKUID: 40, ProductName: "尾差商品", SKUName: "标准款",
		Price: 1, Quantity: 1, Subtotal: 1, DiscountAmount: 0, PayableAmount: 1,
	}

	result, err := service.Create(context.Background(), 7, dto.CreateAfterSaleRequest{
		OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly, Reason: "验证优惠尾差退款",
	})
	if err != nil {
		t.Fatalf("create after-sale: %v", err)
	}
	if result.RefundAmount != 1 {
		t.Fatalf("expected persisted payable amount 1, got %d", result.RefundAmount)
	}
}

func TestAfterSaleMerchantApproveMockRefund(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	created, err := service.Create(context.Background(), 7, dto.CreateAfterSaleRequest{
		OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly, Reason: "商品存在问题",
	})
	if err != nil {
		t.Fatalf("create after-sale: %v", err)
	}

	result, err := service.MerchantApprove(context.Background(), 1, 99, created.ID)
	if err != nil {
		t.Fatalf("approve after-sale: %v", err)
	}
	if result.Status != model.AfterSaleStatusRefunded || result.Refund == nil || result.Refund.Status != model.RefundStatusSucceeded {
		t.Fatalf("unexpected approved after-sale: %+v", result)
	}
	if result.Refund.TransactionID == "" || result.Refund.Amount != 3200 {
		t.Fatalf("unexpected refund: %+v", result.Refund)
	}
	if repo.payments[50].Status != model.PaymentStatusPartiallyRefunded {
		t.Fatalf("unexpected payment status: %d", repo.payments[50].Status)
	}
	if repo.transactions != 2 {
		t.Fatalf("approve should use preparation and finalization transactions, got %d", repo.transactions)
	}
}

func TestAfterSaleMerchantActionsRespectStateAndMerchant(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	created, err := service.Create(context.Background(), 7, dto.CreateAfterSaleRequest{
		OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly, Reason: "商品存在问题",
	})
	if err != nil {
		t.Fatalf("create after-sale: %v", err)
	}
	if _, err := service.MerchantApprove(context.Background(), 2, 99, created.ID); err == nil {
		t.Fatal("expected another merchant to be rejected")
	}
	if _, err := service.MerchantReject(context.Background(), 1, 99, created.ID, "凭证不足"); err != nil {
		t.Fatalf("reject after-sale: %v", err)
	}
	if _, err := service.MerchantReject(context.Background(), 1, 99, created.ID, "重复拒绝"); err == nil {
		t.Fatal("expected repeated rejection to fail")
	}
	if repo.afterSales[created.ID].Status != model.AfterSaleStatusRejected {
		t.Fatalf("unexpected after-sale status: %d", repo.afterSales[created.ID].Status)
	}
}

type fakeAlipayRefundGateway struct {
	submitResult refundProviderResult
	queryResult  refundProviderResult
	submitErr    error
	queryErr     error
	submitCalls  int
	queryCalls   int
	refundNos    []string
}

func (g *fakeAlipayRefundGateway) Submit(
	_ context.Context,
	_ model.Payment,
	refund model.Refund,
	_ string,
) (refundProviderResult, error) {
	g.submitCalls++
	g.refundNos = append(g.refundNos, refund.RefundNo)
	return g.submitResult, g.submitErr
}

func (g *fakeAlipayRefundGateway) Query(
	_ context.Context,
	_ model.Payment,
	refund model.Refund,
) (refundProviderResult, error) {
	g.queryCalls++
	g.refundNos = append(g.refundNos, refund.RefundNo)
	return g.queryResult, g.queryErr
}

func createAlipayAfterSaleForTest(t *testing.T) (*afterSaleService, *fakeAfterSaleRepository, *dto.AfterSaleResponse) {
	t.Helper()
	service, repo := newAfterSaleServiceForTest()
	payment := repo.payments[50]
	payment.PayChannel = model.PayChannelAlipay
	repo.payments[50] = payment
	created, err := service.Create(context.Background(), 7, dto.CreateAfterSaleRequest{
		OrderID: 10, OrderItemID: 20, Type: model.AfterSaleTypeRefundOnly, Reason: "商品存在问题",
	})
	if err != nil {
		t.Fatalf("create after-sale: %v", err)
	}
	return service, repo, created
}

func TestAfterSaleRefundTimeoutStaysUnknown(t *testing.T) {
	service, repo, created := createAlipayAfterSaleForTest(t)
	gateway := &fakeAlipayRefundGateway{submitErr: errors.New("request timeout")}
	service.refunds.alipay = gateway

	result, err := service.MerchantApprove(context.Background(), 1, 99, created.ID)
	if err != nil {
		t.Fatalf("approve should return the persisted processing state: %v", err)
	}
	if result.Status != model.AfterSaleStatusRefunding || result.Refund == nil || result.Refund.Status != model.RefundStatusUnknown {
		t.Fatalf("ambiguous refund was not kept processing: %+v", result)
	}
	if result.Refund.RetryCount != 1 || result.Refund.NextRetryAt == nil || !strings.Contains(result.Refund.LastError, "timeout") {
		t.Fatalf("refund retry metadata missing: %+v", result.Refund)
	}
	if repo.afterSales[created.ID].Status == model.AfterSaleStatusRefundFailed {
		t.Fatal("ambiguous provider result was marked as failed")
	}
}

func TestAfterSaleRefundSyncQueriesBeforeIdempotentResubmit(t *testing.T) {
	service, _, created := createAlipayAfterSaleForTest(t)
	gateway := &fakeAlipayRefundGateway{submitErr: errors.New("request timeout")}
	service.refunds.alipay = gateway

	processing, err := service.MerchantApprove(context.Background(), 1, 99, created.ID)
	if err != nil || processing.Refund == nil {
		t.Fatalf("prepare unknown refund: result=%+v err=%v", processing, err)
	}
	refundNo := processing.Refund.RefundNo
	gateway.submitErr = nil
	gateway.queryResult = refundProviderResult{State: refundProviderStateNotFound}
	gateway.submitResult = refundProviderResult{
		State:         refundProviderStateSucceeded,
		TransactionID: "ALI-REFUND-1",
		RefundedAt:    time.Now(),
	}

	result, err := service.MerchantSyncRefund(context.Background(), 1, created.ID)
	if err != nil {
		t.Fatalf("sync refund: %v", err)
	}
	if result.Status != model.AfterSaleStatusRefunded || result.Refund == nil || result.Refund.Status != model.RefundStatusSucceeded {
		t.Fatalf("unexpected synchronized refund: %+v", result)
	}
	if gateway.queryCalls != 1 || gateway.submitCalls != 2 {
		t.Fatalf("expected query before retry: query=%d submit=%d", gateway.queryCalls, gateway.submitCalls)
	}
	for _, value := range gateway.refundNos {
		if value != refundNo {
			t.Fatalf("refund retry changed idempotency key: want=%s got=%s", refundNo, value)
		}
	}
}

func TestAfterSaleRefundDefinitiveRejectionMarksFailed(t *testing.T) {
	service, repo, created := createAlipayAfterSaleForTest(t)
	service.refunds.alipay = &fakeAlipayRefundGateway{submitResult: refundProviderResult{
		State:  refundProviderStateRejected,
		Reason: "ACQ.REFUND_AMT_NOT_EQUAL_TOTAL",
	}}

	if _, err := service.MerchantApprove(context.Background(), 1, 99, created.ID); err == nil {
		t.Fatal("expected definitive provider rejection")
	}
	refund, err := repo.FindRefundByAfterSaleID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("find refund: %v", err)
	}
	if refund.Status != model.RefundStatusFailed || repo.afterSales[created.ID].Status != model.AfterSaleStatusRefundFailed {
		t.Fatalf("definitive rejection state mismatch: refund=%+v after_sale=%+v", refund, repo.afterSales[created.ID])
	}
}

func TestRefundReconciliationJobFinalizesDueRefund(t *testing.T) {
	service, repo, created := createAlipayAfterSaleForTest(t)
	service.refunds.alipay = &fakeAlipayRefundGateway{submitErr: errors.New("request timeout")}
	processing, err := service.MerchantApprove(context.Background(), 1, 99, created.ID)
	if err != nil || processing.Refund == nil {
		t.Fatalf("prepare unknown refund: result=%+v err=%v", processing, err)
	}
	refund, _ := repo.FindRefundByAfterSaleID(context.Background(), created.ID)
	due := time.Now().Add(-time.Minute)
	refund.NextRetryAt = &due
	repo.refunds[refund.ID] = *refund

	gateway := &fakeAlipayRefundGateway{queryResult: refundProviderResult{
		State:         refundProviderStateSucceeded,
		TransactionID: "ALI-REFUND-2",
		RefundedAt:    time.Now(),
	}}
	job := NewRefundReconciliationService(repo, config.PaymentConfig{}).(*refundReconciliationService)
	job.coordinator.alipay = gateway
	report, err := job.Run(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("reconcile refunds: %v", err)
	}
	if report.Scanned != 1 || report.Succeeded != 1 || report.Errors != 0 {
		t.Fatalf("unexpected reconciliation report: %+v", report)
	}
	latest, _ := repo.FindRefundByAfterSaleID(context.Background(), created.ID)
	if latest.Status != model.RefundStatusSucceeded || repo.afterSales[created.ID].Status != model.AfterSaleStatusRefunded {
		t.Fatalf("refund was not finalized: refund=%+v after_sale=%+v", latest, repo.afterSales[created.ID])
	}
}

func TestRefundFinalizationUsesAccumulatedRefundAmount(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	now := time.Now()
	tradeID := int64(70)
	allocationID := int64(60)
	repo.afterSales[101] = model.AfterSale{
		ID: 101, AfterSaleNo: "AS101", OrderID: 10, OrderItemID: 20, UserID: 7, MerchantID: 1,
		Status: model.AfterSaleStatusRefunding, ActiveKey: "10:20", RefundAmount: 3200,
	}
	repo.afterSales[102] = model.AfterSale{
		ID: 102, AfterSaleNo: "AS102", OrderID: 10, OrderItemID: 21, UserID: 7, MerchantID: 1,
		Status: model.AfterSaleStatusRefunded, ActiveKey: "AS102", RefundAmount: 4800, RefundedAt: &now,
	}
	repo.refunds[201] = model.Refund{
		ID: 201, TradeID: &tradeID, RefundNo: "RF201", AfterSaleID: 101, PaymentID: 50, PaymentAllocationID: &allocationID, OrderID: 10,
		UserID: 7, MerchantID: 1, PayChannel: model.PayChannelMock, Amount: 3200, Status: model.RefundStatusPending,
	}
	repo.refunds[202] = model.Refund{
		ID: 202, TradeID: &tradeID, RefundNo: "RF202", AfterSaleID: 102, PaymentID: 50, PaymentAllocationID: &allocationID, OrderID: 10,
		UserID: 7, MerchantID: 1, PayChannel: model.PayChannelMock, Amount: 4800, Status: model.RefundStatusSucceeded,
		TransactionID: "MOCK-RF202", RefundedAt: &now,
	}
	allocation := repo.allocations[allocationID]
	allocation.RefundedAmount = 4800
	repo.allocations[allocationID] = allocation

	if err := service.refunds.finalize(context.Background(), 201, "MOCK-RF201", now); err != nil {
		t.Fatalf("finalize refund: %v", err)
	}
	if repo.payments[50].Status != model.PaymentStatusRefunded {
		t.Fatalf("expected fully refunded payment, got status %d", repo.payments[50].Status)
	}
	if repo.allocations[allocationID].RefundedAmount != 8000 || repo.trades[tradeID].Status != model.TradeStatusRefunded {
		t.Fatalf("allocation or trade refund state mismatch: allocation=%+v trade=%+v", repo.allocations[allocationID], repo.trades[tradeID])
	}
}

func TestRefundApprovalReservesPendingAllocationAmount(t *testing.T) {
	service, repo := newAfterSaleServiceForTest()
	tradeID := int64(70)
	allocationID := int64(60)
	repo.refunds[201] = model.Refund{
		ID: 201, TradeID: &tradeID, RefundNo: "RF-PENDING", AfterSaleID: 999,
		PaymentID: 50, PaymentAllocationID: &allocationID, OrderID: 10,
		UserID: 7, MerchantID: 1, PayChannel: model.PayChannelMock,
		Amount: 5000, Status: model.RefundStatusPending,
	}
	repo.afterSales[101] = model.AfterSale{
		ID: 101, AfterSaleNo: "AS101", OrderID: 10, OrderItemID: 20,
		UserID: 7, MerchantID: 1, Status: model.AfterSaleStatusPending,
		ActiveKey: "10:20", RefundAmount: 3001,
	}

	_, err := service.MerchantApprove(context.Background(), 1, 99, 101)
	if err == nil || !strings.Contains(err.Error(), "剩余可退金额") {
		t.Fatalf("expected reserved allocation amount to reject over-refund, got: %v", err)
	}
	if len(repo.refunds) != 1 || repo.allocations[allocationID].RefundedAmount != 0 {
		t.Fatalf("rejected refund changed allocation state: refunds=%+v allocation=%+v", repo.refunds, repo.allocations[allocationID])
	}
}

func TestYuanAmountMatchesExactFen(t *testing.T) {
	for _, value := range []string{"12.34", "12.340", "0.01"} {
		expected := int64(1234)
		if value == "0.01" {
			expected = 1
		}
		if !yuanAmountMatches(value, expected) {
			t.Fatalf("expected %q to equal %d fen", value, expected)
		}
	}
	if yuanAmountMatches("12.345", 1234) || yuanAmountMatches("invalid", 1234) {
		t.Fatal("invalid amount matched")
	}
}
