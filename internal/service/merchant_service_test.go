package service

import (
	"context"
	"errors"
	"sort"
	"testing"

	"go-mall/internal/dto"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

type publicMerchantRepositoryStub struct {
	merchants map[int64]model.Merchant
}

func newPublicMerchantRepositoryStub(merchants ...model.Merchant) *publicMerchantRepositoryStub {
	result := &publicMerchantRepositoryStub{merchants: make(map[int64]model.Merchant, len(merchants))}
	for _, merchant := range merchants {
		result.merchants[merchant.ID] = merchant
	}
	return result
}

func (r *publicMerchantRepositoryStub) ListEnabled(_ context.Context, offset, limit int) ([]model.Merchant, int64, error) {
	merchants := make([]model.Merchant, 0, len(r.merchants))
	for _, merchant := range r.merchants {
		if merchant.Status == model.StatusEnabled {
			merchants = append(merchants, merchant)
		}
	}
	sort.Slice(merchants, func(i, j int) bool { return merchants[i].ID < merchants[j].ID })
	total := int64(len(merchants))
	if offset >= len(merchants) {
		return []model.Merchant{}, total, nil
	}
	end := offset + limit
	if end > len(merchants) {
		end = len(merchants)
	}
	return merchants[offset:end], total, nil
}

func (r *publicMerchantRepositoryStub) FindEnabledByID(_ context.Context, id int64) (*model.Merchant, error) {
	merchant, ok := r.merchants[id]
	if !ok || merchant.Status != model.StatusEnabled {
		return nil, gorm.ErrRecordNotFound
	}
	copy := merchant
	return &copy, nil
}

func (r *publicMerchantRepositoryStub) FindByIDs(_ context.Context, ids []int64) ([]model.Merchant, error) {
	merchants := make([]model.Merchant, 0, len(ids))
	for _, id := range ids {
		if merchant, ok := r.merchants[id]; ok {
			merchants = append(merchants, merchant)
		}
	}
	return merchants, nil
}

func TestMerchantServiceListOnlyReturnsEnabledMerchants(t *testing.T) {
	repo := newPublicMerchantRepositoryStub(
		model.Merchant{ID: 2, Name: "商户二", Status: model.StatusEnabled},
		model.Merchant{ID: 1, Name: "商户一", Status: model.StatusEnabled},
		model.Merchant{ID: 3, Name: "已停用商户", Status: 0},
	)
	service := NewMerchantService(repo)

	page, err := service.List(context.Background(), dto.PublicMerchantListRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list merchants returned error: %v", err)
	}
	if page.Total != 2 || len(page.List) != 1 || page.List[0].ID != 1 {
		t.Fatalf("unexpected merchant page: %+v", page)
	}
}

func TestMerchantServiceDetailHidesDisabledMerchant(t *testing.T) {
	repo := newPublicMerchantRepositoryStub(model.Merchant{ID: 3, Name: "已停用商户", Status: 0})
	service := NewMerchantService(repo)

	_, err := service.Detail(context.Background(), 3)
	if !errors.Is(err, ErrPublicMerchantNotFound) {
		t.Fatalf("expected public merchant not found, got %v", err)
	}
}
