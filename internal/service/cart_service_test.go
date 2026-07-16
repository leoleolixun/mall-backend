package service

import (
	"context"
	"testing"

	"go-mall/internal/dto"
	"go-mall/internal/model"
)

func TestCartServiceSupportsItemsFromMultipleMerchants(t *testing.T) {
	productRepo := newCatalogProductRepositoryStub(
		[]model.Product{
			{ID: 11, MerchantID: 1, Name: "商品一", Status: model.ProductStatusOnSale},
			{ID: 22, MerchantID: 2, Name: "商品二", Status: model.ProductStatusOnSale},
		},
		[]model.ProductSKU{
			{ID: 101, MerchantID: 1, ProductID: 11, Name: "SKU 一", Price: 1200, Stock: 10, Status: model.StatusEnabled},
			{ID: 202, MerchantID: 2, ProductID: 22, Name: "SKU 二", Price: 2300, Stock: 10, Status: model.StatusEnabled},
		},
	)
	merchantRepo := newPublicMerchantRepositoryStub(
		model.Merchant{ID: 1, Name: "商户一", Logo: "one.png", Status: model.StatusEnabled},
		model.Merchant{ID: 2, Name: "商户二", Logo: "two.png", Status: model.StatusEnabled},
	)
	redisClient, _ := newRedisClientForServiceTest(t)
	service := NewCartService(redisClient, productRepo, merchantRepo)
	ctx := context.Background()

	if err := service.Add(ctx, 7, dto.AddCartItemRequest{SKUID: 101, Quantity: 2}); err != nil {
		t.Fatalf("add first merchant item: %v", err)
	}
	if err := service.Add(ctx, 7, dto.AddCartItemRequest{SKUID: 202, Quantity: 3}); err != nil {
		t.Fatalf("add second merchant item: %v", err)
	}
	items, err := service.List(ctx, 7)
	if err != nil {
		t.Fatalf("list cart: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two cart items, got %+v", items)
	}
	if items[0].MerchantID != 1 || items[0].MerchantName != "商户一" || items[0].Subtotal != 2400 || !items[0].Available {
		t.Fatalf("unexpected first merchant cart item: %+v", items[0])
	}
	if items[1].MerchantID != 2 || items[1].MerchantName != "商户二" || items[1].Subtotal != 6900 || !items[1].Available {
		t.Fatalf("unexpected second merchant cart item: %+v", items[1])
	}
}

func TestCartServiceMarksDisabledMerchantUnavailable(t *testing.T) {
	productRepo := newCatalogProductRepositoryStub(
		[]model.Product{{ID: 22, MerchantID: 2, Name: "商品二", Status: model.ProductStatusOnSale}},
		[]model.ProductSKU{{ID: 202, MerchantID: 2, ProductID: 22, Name: "SKU 二", Price: 2300, Stock: 10, Status: model.StatusEnabled}},
	)
	merchantRepo := newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Status: model.StatusEnabled})
	redisClient, _ := newRedisClientForServiceTest(t)
	service := NewCartService(redisClient, productRepo, merchantRepo)
	ctx := context.Background()

	if err := service.Add(ctx, 7, dto.AddCartItemRequest{SKUID: 202, Quantity: 1}); err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	merchant := merchantRepo.merchants[2]
	merchant.Status = 0
	merchantRepo.merchants[2] = merchant

	items, err := service.List(ctx, 7)
	if err != nil {
		t.Fatalf("list cart: %v", err)
	}
	if len(items) != 1 || items[0].Available || items[0].Message != "商户已停用" {
		t.Fatalf("disabled merchant item was not marked unavailable: %+v", items)
	}
	if err := service.Update(ctx, 7, 202, dto.UpdateCartItemRequest{Quantity: 2}); err == nil || err.Error() != "商户不存在或已停用" {
		t.Fatalf("expected disabled merchant update error, got %v", err)
	}
}
