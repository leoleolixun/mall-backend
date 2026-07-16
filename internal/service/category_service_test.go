package service

import (
	"context"
	"errors"
	"testing"

	"go-mall/internal/model"
)

type categoryRepositoryStub struct {
	categories []model.Category
	calls      int
}

func (r *categoryRepositoryStub) ListEnabled(_ context.Context, merchantID int64) ([]model.Category, error) {
	r.calls++
	result := make([]model.Category, 0, len(r.categories))
	for _, category := range r.categories {
		if category.MerchantID == merchantID && category.Status == model.StatusEnabled {
			result = append(result, category)
		}
	}
	return result, nil
}

func TestCategoryServiceReturnsMerchantCategoryMetadata(t *testing.T) {
	categoryRepo := &categoryRepositoryStub{categories: []model.Category{{ID: 10, MerchantID: 2, ParentID: 5, Name: "分类", Sort: 8, Status: model.StatusEnabled}}}
	merchantRepo := newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Status: model.StatusEnabled})
	service := NewCategoryService(categoryRepo, merchantRepo)

	items, err := service.ListByMerchant(context.Background(), 2)
	if err != nil {
		t.Fatalf("list categories returned error: %v", err)
	}
	if len(items) != 1 || items[0].MerchantID != 2 || items[0].ParentID != 5 {
		t.Fatalf("unexpected categories: %+v", items)
	}
}

func TestCategoryServiceDoesNotQueryDisabledMerchantCategories(t *testing.T) {
	categoryRepo := &categoryRepositoryStub{}
	merchantRepo := newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Status: 0})
	service := NewCategoryService(categoryRepo, merchantRepo)

	_, err := service.ListByMerchant(context.Background(), 2)
	if !errors.Is(err, ErrPublicMerchantNotFound) {
		t.Fatalf("expected merchant not found, got %v", err)
	}
	if categoryRepo.calls != 0 {
		t.Fatalf("disabled merchant categories were queried %d times", categoryRepo.calls)
	}
}
