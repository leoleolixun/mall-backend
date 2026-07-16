package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakeSettlementRepository struct {
	merchants    map[int64]model.Merchant
	orders       map[int64]model.Order
	allocations  map[int64]model.PaymentAllocation
	refunds      map[int64]model.Refund
	entries      map[int64]model.SettlementEntry
	settlements  map[int64]model.MerchantSettlement
	nextEntryID  int64
	nextSettleID int64
}

func (r *fakeSettlementRepository) Transaction(_ context.Context, fn func(repository.SettlementRepository) error) error {
	return fn(r)
}

func (r *fakeSettlementRepository) ListCompletedOrdersMissingEntries(_ context.Context, limit int) ([]model.Order, error) {
	values := make([]model.Order, 0)
	for _, order := range r.orders {
		if order.Status != model.OrderStatusCompleted || order.CompletedAt == nil {
			continue
		}
		saleFound := false
		commissionFound := false
		for _, entry := range r.entries {
			if entry.EntryNo == fmt.Sprintf("SALE-%d", order.ID) {
				saleFound = true
			}
			if entry.EntryNo == fmt.Sprintf("COMMISSION-%d", order.ID) {
				commissionFound = true
			}
		}
		needsCommission := order.CommissionAmount != nil && *order.CommissionAmount > 0
		if !saleFound || (needsCommission && !commissionFound) {
			values = append(values, order)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (r *fakeSettlementRepository) ListSucceededRefundsMissingEntries(_ context.Context, limit int) ([]model.Refund, error) {
	values := make([]model.Refund, 0)
	for _, refund := range r.refunds {
		if refund.Status != model.RefundStatusSucceeded {
			continue
		}
		found := false
		for _, entry := range r.entries {
			if entry.EntryNo == fmt.Sprintf("REFUND-%d", refund.ID) {
				found = true
				break
			}
		}
		if !found {
			values = append(values, refund)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].UpdatedAt.Equal(values[j].UpdatedAt) {
			return values[i].ID < values[j].ID
		}
		return values[i].UpdatedAt.Before(values[j].UpdatedAt)
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (r *fakeSettlementRepository) FindOrderByID(_ context.Context, orderID int64) (*model.Order, error) {
	value, ok := r.orders[orderID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeSettlementRepository) FindPaymentAllocationByID(_ context.Context, allocationID int64) (*model.PaymentAllocation, error) {
	value, ok := r.allocations[allocationID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeSettlementRepository) SumSucceededRefundsBefore(
	_ context.Context,
	allocationID int64,
	updatedAt time.Time,
	refundID int64,
) (int64, error) {
	var amount int64
	for _, refund := range r.refunds {
		if refund.PaymentAllocationID == nil || *refund.PaymentAllocationID != allocationID || refund.Status != model.RefundStatusSucceeded {
			continue
		}
		if refund.UpdatedAt.Before(updatedAt) || (refund.UpdatedAt.Equal(updatedAt) && refund.ID < refundID) {
			amount += refund.Amount
		}
	}
	return amount, nil
}

func (r *fakeSettlementRepository) CreateEntries(_ context.Context, entries []model.SettlementEntry) error {
	for _, entry := range entries {
		duplicate := false
		for _, existing := range r.entries {
			if existing.EntryNo == entry.EntryNo {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		r.nextEntryID++
		entry.ID = r.nextEntryID
		entry.CreatedAt = time.Now()
		r.entries[entry.ID] = entry
	}
	return nil
}

func (r *fakeSettlementRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	value, ok := r.merchants[merchantID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeSettlementRepository) ListEntriesByMerchantID(_ context.Context, merchantID int64, entryType string, offset, limit int) ([]model.SettlementEntry, int64, error) {
	values := make([]model.SettlementEntry, 0)
	for _, entry := range r.entries {
		if entry.MerchantID == merchantID && (entryType == "" || entry.EntryType == entryType) {
			values = append(values, entry)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID > values[j].ID })
	total := int64(len(values))
	if offset >= len(values) {
		return []model.SettlementEntry{}, total, nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return values[offset:end], total, nil
}

func (r *fakeSettlementRepository) ListSettlementsByMerchantID(_ context.Context, merchantID int64, status, offset, limit int) ([]model.MerchantSettlement, int64, error) {
	values := make([]model.MerchantSettlement, 0)
	for _, settlement := range r.settlements {
		if settlement.MerchantID == merchantID && (status <= 0 || settlement.Status == status) {
			values = append(values, settlement)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID > values[j].ID })
	total := int64(len(values))
	if offset >= len(values) {
		return []model.MerchantSettlement{}, total, nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return values[offset:end], total, nil
}

func (r *fakeSettlementRepository) FindSettlementByIDAndMerchantID(_ context.Context, id, merchantID int64) (*model.MerchantSettlement, error) {
	value, ok := r.settlements[id]
	if !ok || value.MerchantID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	return &value, nil
}

func (r *fakeSettlementRepository) FindSettlementByPeriod(_ context.Context, merchantID int64, start, end time.Time) (*model.MerchantSettlement, error) {
	for _, value := range r.settlements {
		if value.MerchantID == merchantID && value.PeriodStart.Equal(start) && value.PeriodEnd.Equal(end) {
			copy := value
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeSettlementRepository) FindEntriesBySettlementID(_ context.Context, merchantID, settlementID int64) ([]model.SettlementEntry, error) {
	values := make([]model.SettlementEntry, 0)
	for _, entry := range r.entries {
		if entry.MerchantID == merchantID && entry.SettlementID != nil && *entry.SettlementID == settlementID {
			values = append(values, entry)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (r *fakeSettlementRepository) FindEligibleEntriesForUpdate(_ context.Context, merchantID int64, periodEnd, completedBefore time.Time) ([]model.SettlementEntry, error) {
	values := make([]model.SettlementEntry, 0)
	for _, entry := range r.entries {
		if entry.MerchantID != merchantID || entry.SettlementID != nil || !entry.AvailableAt.Before(periodEnd) || entry.OrderID == nil {
			continue
		}
		order := r.orders[*entry.OrderID]
		if order.Status == model.OrderStatusCompleted && order.CompletedAt != nil && !order.CompletedAt.After(completedBefore) {
			values = append(values, entry)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (r *fakeSettlementRepository) CreateSettlement(_ context.Context, settlement *model.MerchantSettlement) error {
	r.nextSettleID++
	settlement.ID = r.nextSettleID
	settlement.CreatedAt = time.Now()
	settlement.UpdatedAt = settlement.CreatedAt
	r.settlements[settlement.ID] = *settlement
	return nil
}

func (r *fakeSettlementRepository) AttachEntries(_ context.Context, entryIDs []int64, settlementID int64) error {
	for _, id := range entryIDs {
		entry, ok := r.entries[id]
		if !ok || entry.SettlementID != nil {
			return fmt.Errorf("entry state changed")
		}
		value := settlementID
		entry.SettlementID = &value
		r.entries[id] = entry
	}
	return nil
}

func newSettlementServiceForTest(now time.Time) (*settlementService, *fakeSettlementRepository) {
	completedAt := now.Add(-10 * 24 * time.Hour)
	rate := 1000
	commission := int64(1000)
	net := int64(9000)
	repo := &fakeSettlementRepository{
		merchants: map[int64]model.Merchant{1: {ID: 1, Name: "商户一", Status: model.StatusEnabled}},
		orders: map[int64]model.Order{10: {
			ID: 10, UserID: 7, MerchantID: 1, Status: model.OrderStatusCompleted,
			PayableAmount: 10000, CommissionRateBPS: &rate, CommissionAmount: &commission,
			SettlementAmount: &net, CompletedAt: &completedAt,
		}},
		allocations: make(map[int64]model.PaymentAllocation), refunds: make(map[int64]model.Refund),
		entries: make(map[int64]model.SettlementEntry), settlements: make(map[int64]model.MerchantSettlement),
	}
	service := NewSettlementService(repo, config.SettlementConfig{HoldDays: 7, BatchSize: 100}).(*settlementService)
	service.now = func() time.Time { return now }
	return service, repo
}

func TestSettlementAccrualAndGenerationReconcileExactly(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	service, repo := newSettlementServiceForTest(now)
	report, err := service.AccrueCompletedOrders(context.Background())
	if err != nil || report.Accrued != 1 || len(repo.entries) != 2 {
		t.Fatalf("accrue completed order: report=%+v entries=%+v err=%v", report, repo.entries, err)
	}
	if second, err := service.AccrueCompletedOrders(context.Background()); err != nil || second.Scanned != 0 || len(repo.entries) != 2 {
		t.Fatalf("accrual is not idempotent: report=%+v entries=%+v err=%v", second, repo.entries, err)
	}
	orderID := int64(10)
	refundID := int64(20)
	if err := repo.CreateEntries(context.Background(), []model.SettlementEntry{
		{EntryNo: "REFUND-20", MerchantID: 1, OrderID: &orderID, RefundID: &refundID, EntryType: model.SettlementEntryRefund, Amount: -2000, AvailableAt: now.Add(-time.Hour)},
		{EntryNo: "COMMISSION-REFUND-20", MerchantID: 1, OrderID: &orderID, RefundID: &refundID, EntryType: model.SettlementEntryCommissionRefund, Amount: 200, AvailableAt: now.Add(-time.Hour)},
	}); err != nil {
		t.Fatalf("seed refund entries: %v", err)
	}

	start := now.Add(-30 * 24 * time.Hour)
	settlement, err := service.Generate(context.Background(), 1, start, now)
	if err != nil {
		t.Fatalf("generate settlement: %v", err)
	}
	if settlement.GrossAmount != 10000 || settlement.CommissionAmount != 800 || settlement.RefundAmount != 2000 || settlement.NetAmount != 7200 || len(settlement.Entries) != 4 {
		t.Fatalf("settlement does not reconcile: %+v", settlement)
	}
	retried, err := service.Generate(context.Background(), 1, start, now)
	if err != nil || retried.ID != settlement.ID || len(repo.settlements) != 1 {
		t.Fatalf("settlement generation is not idempotent: first=%+v retry=%+v err=%v", settlement, retried, err)
	}
}

func TestSettlementAccrualRepairsMissingCommissionEntry(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	service, repo := newSettlementServiceForTest(now)
	orderID := int64(10)
	if err := repo.CreateEntries(context.Background(), []model.SettlementEntry{{
		EntryNo: "SALE-10", MerchantID: 1, OrderID: &orderID,
		EntryType: model.SettlementEntrySale, Amount: 10000, AvailableAt: now.Add(-72 * time.Hour),
	}}); err != nil {
		t.Fatalf("seed sale entry: %v", err)
	}

	report, err := service.AccrueCompletedOrders(context.Background())
	if err != nil || report.Scanned != 1 || report.Accrued != 1 || len(repo.entries) != 2 {
		t.Fatalf("repair missing commission entry: report=%+v entries=%+v err=%v", report, repo.entries, err)
	}
	found := false
	for _, entry := range repo.entries {
		if entry.EntryNo == "COMMISSION-10" && entry.Amount == -1000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing commission entry was not repaired: %+v", repo.entries)
	}
}

func TestSettlementAccrualBackfillsSucceededRefundEntries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	service, repo := newSettlementServiceForTest(now)
	tradeID := int64(70)
	allocationID := int64(60)
	order := repo.orders[10]
	order.TradeID = &tradeID
	repo.orders[10] = order
	repo.allocations[allocationID] = model.PaymentAllocation{
		ID: allocationID, PaymentID: 50, TradeID: tradeID, OrderID: 10,
		MerchantID: 1, Amount: 10000, RefundedAmount: 2000,
	}
	refundedAt := now.Add(-48 * time.Hour)
	repo.refunds[20] = model.Refund{
		ID: 20, TradeID: &tradeID, RefundNo: "RF20", PaymentID: 50,
		PaymentAllocationID: &allocationID, OrderID: 10, MerchantID: 1,
		Amount: 2000, Status: model.RefundStatusSucceeded, RefundedAt: &refundedAt,
		CreatedAt: refundedAt, UpdatedAt: refundedAt,
	}

	report, err := service.AccrueCompletedOrders(context.Background())
	if err != nil || report.Scanned != 2 || report.Accrued != 2 || len(repo.entries) != 4 {
		t.Fatalf("backfill succeeded refund entries: report=%+v entries=%+v err=%v", report, repo.entries, err)
	}
	wantAmounts := map[string]int64{"REFUND-20": -2000, "COMMISSION-REFUND-20": 200}
	for entryNo, want := range wantAmounts {
		found := false
		for _, entry := range repo.entries {
			if entry.EntryNo == entryNo && entry.Amount == want && entry.AvailableAt.Equal(refundedAt) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("entry %s=%d was not backfilled: %+v", entryNo, want, repo.entries)
		}
	}
	if second, err := service.AccrueCompletedOrders(context.Background()); err != nil || second.Scanned != 0 || len(repo.entries) != 4 {
		t.Fatalf("refund accrual is not idempotent: report=%+v entries=%+v err=%v", second, repo.entries, err)
	}
}

func TestSettlementMerchantQueriesAreIsolated(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	service, _ := newSettlementServiceForTest(now)
	if _, err := service.Detail(context.Background(), 2, 1); err == nil {
		t.Fatal("other merchant read settlement detail")
	}
	if _, err := service.ListEntries(context.Background(), 1, dto.SettlementEntryListRequest{EntryType: "unknown"}); err == nil {
		t.Fatal("invalid entry type was accepted")
	}
}
