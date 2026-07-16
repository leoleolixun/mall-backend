package service

import (
	"context"
	"errors"
	"fmt"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type FavoriteService interface {
	List(ctx context.Context, userID int64, req dto.FavoriteListRequest) (*dto.PageResponse[dto.ProductListItem], error)
	Add(ctx context.Context, userID, productID int64) error
	Delete(ctx context.Context, userID, productID int64) error
}

type favoriteService struct {
	repo         repository.FavoriteRepository
	merchantRepo repository.MerchantRepository
}

func NewFavoriteService(repo repository.FavoriteRepository, merchantRepo repository.MerchantRepository) FavoriteService {
	return &favoriteService{repo: repo, merchantRepo: merchantRepo}
}

func (s *favoriteService) List(ctx context.Context, userID int64, req dto.FavoriteListRequest) (*dto.PageResponse[dto.ProductListItem], error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户参数不合法")
	}
	page, pageSize := normalizePage(req.Page, req.PageSize)
	products, prices, total, err := s.repo.ListProducts(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	merchantIDs := make([]int64, 0, len(products))
	merchantIDSet := make(map[int64]struct{})
	for _, product := range products {
		if _, exists := merchantIDSet[product.MerchantID]; exists {
			continue
		}
		merchantIDSet[product.MerchantID] = struct{}{}
		merchantIDs = append(merchantIDs, product.MerchantID)
	}
	merchants, err := s.merchantRepo.FindByIDs(ctx, merchantIDs)
	if err != nil {
		return nil, err
	}
	merchantMap := make(map[int64]model.Merchant, len(merchants))
	for _, merchant := range merchants {
		merchantMap[merchant.ID] = merchant
	}
	list := make([]dto.ProductListItem, 0, len(products))
	for _, product := range products {
		merchant := merchantMap[product.MerchantID]
		list = append(list, dto.ProductListItem{
			ID:           product.ID,
			MerchantID:   product.MerchantID,
			MerchantName: merchant.Name,
			MerchantLogo: merchant.Logo,
			CategoryID:   product.CategoryID,
			Name:         product.Name,
			Cover:        product.Cover,
			MinPrice:     prices[product.ID],
		})
	}
	return &dto.PageResponse[dto.ProductListItem]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *favoriteService) Add(ctx context.Context, userID, productID int64) error {
	if userID <= 0 || productID <= 0 {
		return fmt.Errorf("收藏参数不合法")
	}
	product, err := s.repo.FindOnSaleProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("商品不存在或已下架")
		}
		return err
	}
	return s.repo.Create(ctx, &model.FavoriteProduct{UserID: userID, MerchantID: product.MerchantID, ProductID: productID})
}

func (s *favoriteService) Delete(ctx context.Context, userID, productID int64) error {
	if userID <= 0 || productID <= 0 {
		return fmt.Errorf("收藏参数不合法")
	}
	return s.repo.Delete(ctx, userID, productID)
}
