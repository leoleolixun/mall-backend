package service

import (
	"context"
	"sort"
	"testing"

	"go-mall/internal/dto"
	"go-mall/internal/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type catalogProductRepositoryStub struct {
	products        map[int64]model.Product
	skus            map[int64]model.ProductSKU
	detailFindCalls int
}

func newCatalogProductRepositoryStub(products []model.Product, skus []model.ProductSKU) *catalogProductRepositoryStub {
	repo := &catalogProductRepositoryStub{
		products: make(map[int64]model.Product, len(products)),
		skus:     make(map[int64]model.ProductSKU, len(skus)),
	}
	for _, product := range products {
		repo.products[product.ID] = product
	}
	for _, sku := range skus {
		repo.skus[sku.ID] = sku
	}
	return repo
}

func (r *catalogProductRepositoryStub) ListOnSale(_ context.Context, merchantID, categoryID int64, _ string, offset, limit int) ([]model.Product, int64, error) {
	products := make([]model.Product, 0, len(r.products))
	for _, product := range r.products {
		if product.Status != model.ProductStatusOnSale || (merchantID > 0 && product.MerchantID != merchantID) || (categoryID > 0 && product.CategoryID != categoryID) {
			continue
		}
		products = append(products, product)
	}
	sort.Slice(products, func(i, j int) bool { return products[i].ID > products[j].ID })
	total := int64(len(products))
	if offset >= len(products) {
		return []model.Product{}, total, nil
	}
	end := offset + limit
	if end > len(products) {
		end = len(products)
	}
	return products[offset:end], total, nil
}

func (r *catalogProductRepositoryStub) FindOnSaleByID(_ context.Context, merchantID, id int64) (*model.Product, error) {
	r.detailFindCalls++
	product, ok := r.products[id]
	if !ok || product.Status != model.ProductStatusOnSale || (merchantID > 0 && product.MerchantID != merchantID) {
		return nil, gorm.ErrRecordNotFound
	}
	copy := product
	return &copy, nil
}

func (r *catalogProductRepositoryStub) ListEnabledSKUs(_ context.Context, merchantID, productID int64) ([]model.ProductSKU, error) {
	result := make([]model.ProductSKU, 0)
	for _, sku := range r.skus {
		if sku.ProductID == productID && sku.Status == model.StatusEnabled && (merchantID == 0 || sku.MerchantID == merchantID) {
			result = append(result, sku)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *catalogProductRepositoryStub) FindMinPrices(_ context.Context, merchantID int64, productIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64)
	for _, sku := range r.skus {
		if sku.Status != model.StatusEnabled || (merchantID > 0 && sku.MerchantID != merchantID) || !containsInt64(productIDs, sku.ProductID) {
			continue
		}
		price, exists := result[sku.ProductID]
		if !exists || sku.Price < price {
			result[sku.ProductID] = sku.Price
		}
	}
	return result, nil
}

func (r *catalogProductRepositoryStub) FindSKUsByIDs(_ context.Context, merchantID int64, skuIDs []int64) ([]model.ProductSKU, error) {
	result := make([]model.ProductSKU, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		sku, ok := r.skus[skuID]
		if ok && (merchantID == 0 || sku.MerchantID == merchantID) {
			result = append(result, sku)
		}
	}
	return result, nil
}

func (r *catalogProductRepositoryStub) FindProductsByIDs(_ context.Context, merchantID int64, productIDs []int64) ([]model.Product, error) {
	result := make([]model.Product, 0, len(productIDs))
	for _, productID := range productIDs {
		product, ok := r.products[productID]
		if ok && (merchantID == 0 || product.MerchantID == merchantID) {
			result = append(result, product)
		}
	}
	return result, nil
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func newRedisClientForServiceTest(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, server
}

func TestProductServiceListsProductsAcrossEnabledMerchants(t *testing.T) {
	productRepo := newCatalogProductRepositoryStub(
		[]model.Product{
			{ID: 11, MerchantID: 1, CategoryID: 1, Name: "商品一", Status: model.ProductStatusOnSale},
			{ID: 22, MerchantID: 2, CategoryID: 2, Name: "商品二", Status: model.ProductStatusOnSale},
		},
		[]model.ProductSKU{
			{ID: 101, MerchantID: 1, ProductID: 11, Price: 1200, Status: model.StatusEnabled},
			{ID: 202, MerchantID: 2, ProductID: 22, Price: 2300, Status: model.StatusEnabled},
		},
	)
	merchantRepo := newPublicMerchantRepositoryStub(
		model.Merchant{ID: 1, Name: "商户一", Logo: "one.png", Status: model.StatusEnabled},
		model.Merchant{ID: 2, Name: "商户二", Logo: "two.png", Status: model.StatusEnabled},
	)
	redisClient, _ := newRedisClientForServiceTest(t)
	service := NewProductService(productRepo, merchantRepo, redisClient)

	page, err := service.List(context.Background(), dto.ProductListRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list products returned error: %v", err)
	}
	if page.Total != 2 || len(page.List) != 2 {
		t.Fatalf("unexpected product page: %+v", page)
	}
	if page.List[0].MerchantID != 2 || page.List[0].MerchantName != "商户二" || page.List[0].MinPrice != 2300 {
		t.Fatalf("unexpected cross-merchant product: %+v", page.List[0])
	}

	merchantPage, err := service.List(context.Background(), dto.ProductListRequest{Page: 1, PageSize: 10, MerchantID: 1})
	if err != nil {
		t.Fatalf("list merchant products returned error: %v", err)
	}
	if merchantPage.Total != 1 || len(merchantPage.List) != 1 || merchantPage.List[0].MerchantID != 1 {
		t.Fatalf("merchant filter was not applied: %+v", merchantPage)
	}
}

func TestProductDetailCacheDoesNotExposeDisabledMerchant(t *testing.T) {
	productRepo := newCatalogProductRepositoryStub(
		[]model.Product{{ID: 22, MerchantID: 2, CategoryID: 2, Name: "商品二", Status: model.ProductStatusOnSale}},
		[]model.ProductSKU{{ID: 202, MerchantID: 2, ProductID: 22, Price: 2300, Status: model.StatusEnabled}},
	)
	merchantRepo := newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Status: model.StatusEnabled})
	redisClient, _ := newRedisClientForServiceTest(t)
	service := NewProductService(productRepo, merchantRepo, redisClient)

	if _, err := service.Detail(context.Background(), 22); err != nil {
		t.Fatalf("first product detail returned error: %v", err)
	}
	merchant := merchantRepo.merchants[2]
	merchant.Name = "商户二新名称"
	merchantRepo.merchants[2] = merchant
	cached, err := service.Detail(context.Background(), 22)
	if err != nil || cached.MerchantName != "商户二新名称" {
		t.Fatalf("cached detail did not refresh merchant metadata: detail=%+v err=%v", cached, err)
	}
	merchant = merchantRepo.merchants[2]
	merchant.Status = 0
	merchantRepo.merchants[2] = merchant
	delete(productRepo.products, 22)

	if _, err := service.Detail(context.Background(), 22); err == nil {
		t.Fatal("cached product from disabled merchant was returned")
	}
	if productRepo.detailFindCalls != 2 {
		t.Fatalf("disabled merchant should invalidate cache path, detail calls=%d", productRepo.detailFindCalls)
	}
}
