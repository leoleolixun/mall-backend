package service

import (
	"context"
	"testing"

	"go-mall/internal/dto"
	"go-mall/internal/model"

	"gorm.io/gorm"
)

type fakeFavoriteRepository struct {
	products     map[int64]model.Product
	favorites    map[int64]map[int64]bool
	lastFavorite *model.FavoriteProduct
}

func newFakeFavoriteRepository() *fakeFavoriteRepository {
	return &fakeFavoriteRepository{
		products:  make(map[int64]model.Product),
		favorites: make(map[int64]map[int64]bool),
	}
}

func (r *fakeFavoriteRepository) FindOnSaleProduct(_ context.Context, productID int64) (*model.Product, error) {
	product, ok := r.products[productID]
	if !ok || product.Status != model.ProductStatusOnSale {
		return nil, gorm.ErrRecordNotFound
	}
	return &product, nil
}

func (r *fakeFavoriteRepository) Create(_ context.Context, favorite *model.FavoriteProduct) error {
	copy := *favorite
	r.lastFavorite = &copy
	if r.favorites[favorite.UserID] == nil {
		r.favorites[favorite.UserID] = make(map[int64]bool)
	}
	r.favorites[favorite.UserID][favorite.ProductID] = true
	return nil
}

func (r *fakeFavoriteRepository) Delete(_ context.Context, userID, productID int64) error {
	delete(r.favorites[userID], productID)
	return nil
}

func (r *fakeFavoriteRepository) ListProducts(_ context.Context, userID int64, offset, limit int) ([]model.Product, map[int64]int64, int64, error) {
	products := make([]model.Product, 0)
	prices := make(map[int64]int64)
	for productID := range r.favorites[userID] {
		product, ok := r.products[productID]
		if !ok || product.Status != model.ProductStatusOnSale {
			continue
		}
		products = append(products, product)
		prices[productID] = productID * 100
	}
	total := int64(len(products))
	if offset >= len(products) {
		return []model.Product{}, prices, total, nil
	}
	end := offset + limit
	if end > len(products) {
		end = len(products)
	}
	return products[offset:end], prices, total, nil
}

func TestFavoriteServiceAddAndDelete(t *testing.T) {
	repo := newFakeFavoriteRepository()
	repo.products[3] = model.Product{ID: 3, MerchantID: 2, Status: model.ProductStatusOnSale}
	merchantRepo := newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Status: model.StatusEnabled})
	service := NewFavoriteService(repo, merchantRepo)

	if err := service.Add(context.Background(), 7, 3); err != nil {
		t.Fatalf("add favorite returned error: %v", err)
	}
	if !repo.favorites[7][3] {
		t.Fatal("favorite was not created")
	}
	if repo.lastFavorite == nil || repo.lastFavorite.MerchantID != 2 {
		t.Fatalf("favorite did not use product merchant: %+v", repo.lastFavorite)
	}
	if err := service.Add(context.Background(), 7, 3); err != nil {
		t.Fatalf("idempotent add returned error: %v", err)
	}
	if err := service.Delete(context.Background(), 7, 3); err != nil {
		t.Fatalf("delete favorite returned error: %v", err)
	}
	if repo.favorites[7][3] {
		t.Fatal("favorite was not deleted")
	}
	if err := service.Delete(context.Background(), 7, 3); err != nil {
		t.Fatalf("idempotent delete returned error: %v", err)
	}
}

func TestFavoriteServiceRejectsUnavailableProduct(t *testing.T) {
	repo := newFakeFavoriteRepository()
	repo.products[3] = model.Product{ID: 3, MerchantID: defaultMerchantID, Status: model.ProductStatusOffSale}
	service := NewFavoriteService(repo, newPublicMerchantRepositoryStub())

	err := service.Add(context.Background(), 7, 3)
	if err == nil || err.Error() != "商品不存在或已下架" {
		t.Fatalf("expected unavailable product error, got %v", err)
	}
}

func TestFavoriteServiceListMapsProductPage(t *testing.T) {
	repo := newFakeFavoriteRepository()
	repo.products[3] = model.Product{ID: 3, MerchantID: 2, CategoryID: 2, Name: "测试商品", Cover: "https://example.com/product.jpg", Status: model.ProductStatusOnSale}
	repo.favorites[7] = map[int64]bool{3: true}
	service := NewFavoriteService(repo, newPublicMerchantRepositoryStub(model.Merchant{ID: 2, Name: "商户二", Logo: "merchant.png", Status: model.StatusEnabled}))

	page, err := service.List(context.Background(), 7, dto.FavoriteListRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list favorites returned error: %v", err)
	}
	if page.Total != 1 || len(page.List) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	item := page.List[0]
	if item.ID != 3 || item.Name != "测试商品" || item.MinPrice != 300 || item.MerchantID != 2 || item.MerchantName != "商户二" {
		t.Fatalf("unexpected favorite item: %+v", item)
	}
}
