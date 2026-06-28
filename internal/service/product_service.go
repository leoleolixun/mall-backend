package service

import (
	"context"
	"encoding/json"
	"fmt"
	"go-mall/internal/dto"
	"go-mall/internal/repository"
	"time"

	"github.com/redis/go-redis/v9"
)

// 商品相关的服务接口
type ProductService interface {
	// 获取商品列表，支持分页、分类和关键字搜索
	List(ctx context.Context, req dto.ProductListRequest) (*dto.PageResponse[dto.ProductListItem], error)
	// 获取商品详情
	Detail(ctx context.Context, id int64) (*dto.ProductDetailResponse, error)
	// 获取商品的 SKU 列表
	SKUs(ctx context.Context, productID int64) ([]dto.SKUResponse, error)
}

type productService struct {
	productRepo repository.ProductRepository
	redis       *redis.Client
}

const productDetailCacheTTL = 5 * time.Minute
const emptyCacheTTL = time.Minute

func NewProductService(productRepo repository.ProductRepository, redis *redis.Client) ProductService {
	return &productService{
		productRepo: productRepo,
		redis:       redis,
	}
}

func (s *productService) List(ctx context.Context, req dto.ProductListRequest) (*dto.PageResponse[dto.ProductListItem], error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 50 {
		req.PageSize = 50
	}
	offset := (req.Page - 1) * req.PageSize

	products, total, err := s.productRepo.ListOnSale(
		ctx,
		defaultMerchantID,
		req.CategoryID,
		req.Keyword,
		offset,
		req.PageSize,
	)
	if err != nil {
		return nil, err
	}

	productIDs := make([]int64, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}

	minPrices, err := s.productRepo.FindMinPrices(ctx, defaultMerchantID, productIDs)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ProductListItem, 0, len(products))
	for _, product := range products {
		items = append(items, dto.ProductListItem{
			ID:         product.ID,
			MerchantID: product.MerchantID,
			CategoryID: product.CategoryID,
			Name:       product.Name,
			Cover:      product.Cover,
			MinPrice:   minPrices[product.ID],
		})
	}

	return &dto.PageResponse[dto.ProductListItem]{
		List:     items,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}

func (s *productService) Detail(ctx context.Context, id int64) (*dto.ProductDetailResponse, error) {
	cacheKey := fmt.Sprintf("mall:product:detail:%d", id)
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var cachedDetail dto.ProductDetailResponse
		if err := json.Unmarshal([]byte(cached), &cachedDetail); err == nil {
			return &cachedDetail, nil
		}
	}

	product, err := s.productRepo.FindOnSaleByID(ctx, defaultMerchantID, id)
	if err != nil {
		return nil, err
	}

	skus, err := s.SKUs(ctx, product.ID)
	if err != nil {
		return nil, err
	}

	detail := &dto.ProductDetailResponse{
		ID:          product.ID,
		MerchantID:  product.MerchantID,
		CategoryID:  product.CategoryID,
		Name:        product.Name,
		Cover:       product.Cover,
		Description: product.Description,
		SKUs:        skus,
	}

	payload, err := json.Marshal(detail)
	if err == nil {
		_ = s.redis.Set(ctx, cacheKey, payload, productDetailCacheTTL).Err()
	}

	return detail, nil
}

func (s *productService) SKUs(ctx context.Context, productID int64) ([]dto.SKUResponse, error) {
	skus, err := s.productRepo.ListEnabledSKUs(ctx, defaultMerchantID, productID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.SKUResponse, 0, len(skus))
	for _, sku := range skus {
		result = append(result, dto.SKUResponse{
			ID:    sku.ID,
			Name:  sku.Name,
			Image: sku.Image,
			Price: sku.Price,
			Stock: sku.Stock,
		})
	}

	return result, nil
}
