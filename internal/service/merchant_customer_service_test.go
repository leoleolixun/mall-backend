package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type fakeMerchantCustomerRepository struct {
	merchant model.Merchant
	records  map[int64]repository.MerchantCustomerRecord
	orders   map[int64][]model.Order
}

func (r *fakeMerchantCustomerRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if r.merchant.ID != merchantID {
		return nil, fmt.Errorf("not found")
	}
	merchant := r.merchant
	return &merchant, nil
}

func (r *fakeMerchantCustomerRepository) Overview(_ context.Context, merchantID int64, since time.Time) (*repository.MerchantCustomerOverview, error) {
	if merchantID != r.merchant.ID {
		return nil, fmt.Errorf("not found")
	}
	result := &repository.MerchantCustomerOverview{}
	for _, record := range r.records {
		result.TotalCustomers++
		result.TotalPaidAmount += record.TotalPaidAmount
		if record.PaidOrders >= 2 {
			result.RepeatCustomers++
		}
		if !record.FirstPaidAt.Before(since) {
			result.NewCustomers30D++
		}
		if !record.LastPaidAt.Before(since) {
			result.ActiveCustomers30D++
		}
	}
	return result, nil
}

func (r *fakeMerchantCustomerRepository) List(_ context.Context, merchantID int64, offset int, limit int, keyword string, repeatOnly bool) ([]repository.MerchantCustomerRecord, int64, error) {
	if merchantID != r.merchant.ID {
		return nil, 0, fmt.Errorf("not found")
	}
	result := make([]repository.MerchantCustomerRecord, 0)
	for _, record := range r.records {
		if repeatOnly && record.PaidOrders < 2 {
			continue
		}
		if keyword != "" && record.Nickname != keyword && record.Mobile != keyword {
			continue
		}
		result = append(result, record)
	}
	total := int64(len(result))
	if offset >= len(result) {
		return []repository.MerchantCustomerRecord{}, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (r *fakeMerchantCustomerRepository) Find(_ context.Context, merchantID int64, userID int64) (*repository.MerchantCustomerRecord, error) {
	record, ok := r.records[userID]
	if merchantID != r.merchant.ID || !ok {
		return nil, fmt.Errorf("not found")
	}
	return &record, nil
}

func (r *fakeMerchantCustomerRepository) ListRecentOrders(_ context.Context, merchantID int64, userID int64, limit int) ([]model.Order, error) {
	if merchantID != r.merchant.ID {
		return nil, fmt.Errorf("not found")
	}
	orders := r.orders[userID]
	if len(orders) > limit {
		orders = orders[:limit]
	}
	return orders, nil
}

func newMerchantCustomerServiceForTest() (*merchantCustomerService, *fakeMerchantCustomerRepository) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.Local)
	repo := &fakeMerchantCustomerRepository{
		merchant: model.Merchant{ID: 3, Name: "测试商户", Status: model.StatusEnabled},
		records: map[int64]repository.MerchantCustomerRecord{
			11: {
				UserID: 11, Nickname: "复购顾客", Mobile: "13812345678", UserStatus: model.StatusEnabled,
				PaidOrders: 3, TotalPaidAmount: 30000, FirstPaidAt: now.AddDate(0, 0, -40), LastPaidAt: now.AddDate(0, 0, -2), RegisteredAt: now.AddDate(0, -3, 0),
			},
			12: {
				UserID: 12, Nickname: "新顾客", Mobile: "13900001111", UserStatus: model.StatusEnabled,
				PaidOrders: 1, TotalPaidAmount: 10000, FirstPaidAt: now.AddDate(0, 0, -5), LastPaidAt: now.AddDate(0, 0, -5), RegisteredAt: now.AddDate(0, 0, -10),
			},
		},
		orders: map[int64][]model.Order{
			11: {{ID: 101, OrderNo: "ORDER101", MerchantID: 3, UserID: 11, Status: model.OrderStatusPaid, PayableAmount: 10000, CreatedAt: now.AddDate(0, 0, -2)}},
		},
	}
	service := NewMerchantCustomerService(repo).(*merchantCustomerService)
	service.now = func() time.Time { return now }
	return service, repo
}

func TestMerchantCustomerOverview(t *testing.T) {
	service, _ := newMerchantCustomerServiceForTest()
	overview, err := service.Overview(context.Background(), 3)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.TotalCustomers != 2 || overview.RepeatCustomers != 1 || overview.RepeatRateBPS != 5000 {
		t.Fatalf("unexpected repeat metrics: %+v", overview)
	}
	if overview.NewCustomers30D != 1 || overview.ActiveCustomers30D != 2 || overview.AveragePaidAmount != 20000 {
		t.Fatalf("unexpected customer metrics: %+v", overview)
	}
}

func TestMerchantCustomerListMasksMobileAndFiltersRepeat(t *testing.T) {
	service, _ := newMerchantCustomerServiceForTest()
	result, err := service.List(context.Background(), 3, dto.MerchantCustomerListRequest{Page: 1, PageSize: 20, RepeatOnly: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].MobileMasked != "138****5678" || !result.List[0].IsRepeat {
		t.Fatalf("unexpected customer list: %+v", result)
	}
}

func TestMerchantCustomerDetailRejectsAnotherMerchant(t *testing.T) {
	service, _ := newMerchantCustomerServiceForTest()
	if _, err := service.Detail(context.Background(), 4, 11); err == nil {
		t.Fatal("expected another merchant to be rejected")
	}
}

func TestMerchantCustomerDetailReturnsRecentOrders(t *testing.T) {
	service, _ := newMerchantCustomerServiceForTest()
	result, err := service.Detail(context.Background(), 3, 11)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if result.Customer.UserID != 11 || len(result.RecentOrders) != 1 || result.RecentOrders[0].OrderNo != "ORDER101" {
		t.Fatalf("unexpected customer detail: %+v", result)
	}
}
