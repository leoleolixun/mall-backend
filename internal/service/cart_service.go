package service

import (
	"context"
	"errors"
	"fmt"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"sort"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func cartKey(userID int64) string {
	return fmt.Sprintf("mall:cart:%d", userID)
}

type CartService interface {
	List(ctx context.Context, userID int64) ([]dto.CartItemResponse, error)
	Add(ctx context.Context, userID int64, req dto.AddCartItemRequest) error
	Update(ctx context.Context, userID int64, skuID int64, req dto.UpdateCartItemRequest) error
	Delete(ctx context.Context, userID int64, skuID int64) error
	Clear(ctx context.Context, userID int64) error
}

type cartService struct {
	redisClient *redis.Client
	productRepo repository.ProductRepository
}

func NewCartService(redisClient *redis.Client, productRepo repository.ProductRepository) CartService {
	return &cartService{
		redisClient: redisClient,
		productRepo: productRepo,
	}
}

type cartEntry struct {
	SKUID    int64
	Quantity int
}

func (s *cartService) Add(ctx context.Context, userID int64, req dto.AddCartItemRequest) error {
	if req.SKUID <= 0 {
		return fmt.Errorf("SKUID不能为空")
	}

	if req.Quantity <= 0 {
		return fmt.Errorf("数量必须大于0")
	}
	sku, product, err := s.getSKUAndProduct(ctx, req.SKUID)
	if err != nil {
		return err
	}

	currentQuantity, err := s.getCurrentQuantity(ctx, userID, req.SKUID)
	if err != nil {
		return err
	}

	newQuantity := currentQuantity + req.Quantity
	if err := checkCartAvailable(sku, product, newQuantity); err != nil {
		return err
	}

	return s.redisClient.HSet(
		ctx,
		cartKey(userID),
		strconv.FormatInt(req.SKUID, 10),
		newQuantity,
	).Err()
}

func (s *cartService) Update(ctx context.Context, userID int64, skuID int64, req dto.UpdateCartItemRequest) error {
	if skuID <= 0 {
		return fmt.Errorf("sku_id 不能为空")
	}
	if req.Quantity <= 0 {
		return fmt.Errorf("购买数量必须大于 0")
	}

	sku, product, err := s.getSKUAndProduct(ctx, skuID)
	if err != nil {
		return err
	}

	if err := checkCartAvailable(sku, product, req.Quantity); err != nil {
		return err
	}

	return s.redisClient.HSet(
		ctx,
		cartKey(userID),
		strconv.FormatInt(skuID, 10),
		req.Quantity,
	).Err()
}

func (s *cartService) Delete(ctx context.Context, userID int64, skuID int64) error {
	if skuID <= 0 {
		return fmt.Errorf("sku_id 不能为空")
	}

	return s.redisClient.HDel(
		ctx,
		cartKey(userID),
		strconv.FormatInt(skuID, 10),
	).Err()
}

func (s *cartService) Clear(ctx context.Context, userID int64) error {
	return s.redisClient.Del(ctx, cartKey(userID)).Err()
}

func (s *cartService) List(ctx context.Context, userID int64) ([]dto.CartItemResponse, error) {
	values, err := s.redisClient.HGetAll(ctx, cartKey(userID)).Result()
	if err != nil {
		return nil, err
	}

	entries := parseCartEntries(values)
	if len(entries) == 0 {
		return []dto.CartItemResponse{}, nil
	}

	skuIDs := make([]int64, 0, len(entries))
	for _, entry := range entries {
		skuIDs = append(skuIDs, entry.SKUID)
	}

	skus, err := s.productRepo.FindSKUsByIDs(ctx, defaultMerchantID, skuIDs)
	if err != nil {
		return nil, err
	}

	skuMap := make(map[int64]model.ProductSKU)
	productIDSet := make(map[int64]struct{})

	for _, sku := range skus {
		skuMap[sku.ID] = sku
		productIDSet[sku.ProductID] = struct{}{}
	}

	productIDs := make([]int64, 0, len(productIDSet))
	for productID := range productIDSet {
		productIDs = append(productIDs, productID)
	}

	products, err := s.productRepo.FindProductsByIDs(ctx, defaultMerchantID, productIDs)
	if err != nil {
		return nil, err
	}

	productMap := make(map[int64]model.Product)
	for _, product := range products {
		productMap[product.ID] = product
	}

	result := make([]dto.CartItemResponse, 0, len(entries))

	for _, entry := range entries {
		item := dto.CartItemResponse{
			SKUID:    entry.SKUID,
			Quantity: entry.Quantity,
		}

		sku, ok := skuMap[entry.SKUID]
		if !ok {
			item.Available = false
			item.Message = "SKU 不存在或已删除"
			result = append(result, item)
			continue
		}

		product, ok := productMap[sku.ProductID]
		if !ok {
			item.ProductID = sku.ProductID
			item.SKUName = sku.Name
			item.SKUImage = sku.Image
			item.Price = sku.Price
			item.Stock = sku.Stock
			item.Subtotal = sku.Price * int64(entry.Quantity)
			item.Available = false
			item.Message = "商品不存在或已删除"
			result = append(result, item)
			continue
		}

		item.ProductID = product.ID
		item.ProductName = product.Name
		item.SKUName = sku.Name
		item.SKUImage = sku.Image
		item.Price = sku.Price
		item.Stock = sku.Stock
		item.Subtotal = sku.Price * int64(entry.Quantity)
		item.Available, item.Message = getCartItemStatus(sku, product, entry.Quantity)

		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SKUID < result[j].SKUID
	})

	return result, nil
}

func (s *cartService) getCurrentQuantity(ctx context.Context, userID int64, skuID int64) (int, error) {
	value, err := s.redisClient.HGet(ctx, cartKey(userID), strconv.FormatInt(skuID, 10)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	quantity, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("购物车数量格式错误")
	}

	return quantity, nil
}

func (s *cartService) getSKUAndProduct(ctx context.Context, skuID int64) (*model.ProductSKU, *model.Product, error) {
	skus, err := s.productRepo.FindSKUsByIDs(ctx, defaultMerchantID, []int64{skuID})
	if err != nil {
		return nil, nil, err
	}
	if len(skus) == 0 {
		return nil, nil, fmt.Errorf("SKU 不存在")
	}

	sku := skus[0]

	products, err := s.productRepo.FindProductsByIDs(ctx, defaultMerchantID, []int64{sku.ProductID})
	if err != nil {
		return nil, nil, err
	}
	if len(products) == 0 {
		return nil, nil, fmt.Errorf("商品不存在")
	}

	product := products[0]

	return &sku, &product, nil
}

func parseCartEntries(values map[string]string) []cartEntry {
	entries := make([]cartEntry, 0, len(values))

	for skuIDText, quantityText := range values {
		skuID, err := strconv.ParseInt(skuIDText, 10, 64)
		if err != nil {
			continue
		}

		quantity, err := strconv.Atoi(quantityText)
		if err != nil || quantity <= 0 {
			continue
		}

		entries = append(entries, cartEntry{
			SKUID:    skuID,
			Quantity: quantity,
		})
	}

	return entries
}

func checkCartAvailable(sku *model.ProductSKU, product *model.Product, quantity int) error {
	available, message := getCartItemStatus(*sku, *product, quantity)
	if !available {
		return errors.New(message)
	}

	return nil
}

func getCartItemStatus(sku model.ProductSKU, product model.Product, quantity int) (bool, string) {
	if sku.Status != model.StatusEnabled {
		return false, "SKU 已下架"
	}

	if product.Status != model.ProductStatusOnSale {
		return false, "商品已下架"
	}

	if sku.Stock < quantity {
		return false, "库存不足"
	}

	return true, ""
}
