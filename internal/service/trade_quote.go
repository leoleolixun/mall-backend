package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/pricing"
	"go-mall/internal/repository"
)

const (
	maxTradeDistinctSKUs = 100
	maxTradeItemQuantity = 999
)

type normalizedTradeInput struct {
	SKUIds           []int64
	Quantities       map[int64]int
	CouponByMerchant map[int64]int64
	UserCouponIDs    []int64
}

type tradeQuoteItem struct {
	Product  model.Product
	SKU      model.ProductSKU
	Response dto.OrderItemResponse
}

type tradeQuoteGroup struct {
	Merchant       model.Merchant
	Items          []tradeQuoteItem
	GoodsAmount    int64
	FreightAmount  int64
	DiscountAmount int64
	PayableAmount  int64
	UserCouponID   int64
	CouponID       int64
}

type tradeQuote struct {
	Address        model.Address
	Groups         []tradeQuoteGroup
	GoodsAmount    int64
	FreightAmount  int64
	DiscountAmount int64
	PayableAmount  int64
}

func normalizeTradeInput(items []dto.OrderRequestItem, selections []dto.MerchantCouponSelection) (normalizedTradeInput, error) {
	if len(items) == 0 {
		return normalizedTradeInput{}, fmt.Errorf("交易商品不能为空")
	}
	quantities := make(map[int64]int)
	for _, item := range items {
		if item.SKUID <= 0 {
			return normalizedTradeInput{}, fmt.Errorf("sku_id 必须大于 0")
		}
		if item.Quantity <= 0 {
			return normalizedTradeInput{}, fmt.Errorf("购买数量必须大于 0")
		}
		if quantities[item.SKUID] > maxTradeItemQuantity-item.Quantity {
			return normalizedTradeInput{}, fmt.Errorf("SKU %d 购买数量不能超过 %d", item.SKUID, maxTradeItemQuantity)
		}
		quantities[item.SKUID] += item.Quantity
	}
	if len(quantities) > maxTradeDistinctSKUs {
		return normalizedTradeInput{}, fmt.Errorf("一次交易最多包含 %d 个不同 SKU", maxTradeDistinctSKUs)
	}
	skuIDs := make([]int64, 0, len(quantities))
	for skuID := range quantities {
		skuIDs = append(skuIDs, skuID)
	}
	sort.Slice(skuIDs, func(i, j int) bool { return skuIDs[i] < skuIDs[j] })

	couponByMerchant := make(map[int64]int64)
	userCouponSet := make(map[int64]struct{})
	for _, selection := range selections {
		if selection.MerchantID <= 0 {
			return normalizedTradeInput{}, fmt.Errorf("M8 暂不支持平台优惠券")
		}
		if selection.UserCouponID <= 0 {
			return normalizedTradeInput{}, fmt.Errorf("user_coupon_id 必须大于 0")
		}
		if _, exists := couponByMerchant[selection.MerchantID]; exists {
			return normalizedTradeInput{}, fmt.Errorf("商户 %d 只能选择一张优惠券", selection.MerchantID)
		}
		if _, exists := userCouponSet[selection.UserCouponID]; exists {
			return normalizedTradeInput{}, fmt.Errorf("同一张用户优惠券不能重复选择")
		}
		couponByMerchant[selection.MerchantID] = selection.UserCouponID
		userCouponSet[selection.UserCouponID] = struct{}{}
	}
	userCouponIDs := make([]int64, 0, len(userCouponSet))
	for id := range userCouponSet {
		userCouponIDs = append(userCouponIDs, id)
	}
	sort.Slice(userCouponIDs, func(i, j int) bool { return userCouponIDs[i] < userCouponIDs[j] })

	return normalizedTradeInput{
		SKUIds:           skuIDs,
		Quantities:       quantities,
		CouponByMerchant: couponByMerchant,
		UserCouponIDs:    userCouponIDs,
	}, nil
}

func buildTradeQuote(
	ctx context.Context,
	repo repository.TradeRepository,
	userID int64,
	addressID int64,
	input normalizedTradeInput,
	forUpdate bool,
	now time.Time,
) (*tradeQuote, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if addressID <= 0 {
		return nil, fmt.Errorf("收货地址不能为空")
	}
	address, err := repo.FindAddress(ctx, addressID, userID)
	if err != nil {
		return nil, fmt.Errorf("收货地址不存在")
	}

	// Creation locks all SKU rows in ascending ID order before any stock mutation.
	skus, err := repo.FindSKUsByIDs(ctx, input.SKUIds, forUpdate)
	if err != nil {
		return nil, err
	}
	skuMap := make(map[int64]model.ProductSKU, len(skus))
	productIDSet := make(map[int64]struct{})
	merchantIDSet := make(map[int64]struct{})
	for _, sku := range skus {
		skuMap[sku.ID] = sku
		productIDSet[sku.ProductID] = struct{}{}
		merchantIDSet[sku.MerchantID] = struct{}{}
	}
	for _, skuID := range input.SKUIds {
		if _, exists := skuMap[skuID]; !exists {
			return nil, fmt.Errorf("SKU %d 不存在", skuID)
		}
	}

	productIDs := sortedInt64Set(productIDSet)
	products, err := repo.FindProductsByIDs(ctx, productIDs, forUpdate)
	if err != nil {
		return nil, err
	}
	productMap := make(map[int64]model.Product, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	merchantIDs := sortedInt64Set(merchantIDSet)
	merchants, err := repo.FindMerchantsByIDs(ctx, merchantIDs, forUpdate)
	if err != nil {
		return nil, err
	}
	merchantMap := make(map[int64]model.Merchant, len(merchants))
	for _, merchant := range merchants {
		merchantMap[merchant.ID] = merchant
	}

	groupsByMerchant := make(map[int64]*tradeQuoteGroup, len(merchantIDs))
	for _, skuID := range input.SKUIds {
		sku := skuMap[skuID]
		quantity := input.Quantities[skuID]
		if sku.Status != model.StatusEnabled {
			return nil, fmt.Errorf("SKU %d 已下架", skuID)
		}
		if sku.Stock < quantity {
			return nil, fmt.Errorf("SKU %d 库存不足", skuID)
		}
		product, exists := productMap[sku.ProductID]
		if !exists || product.MerchantID != sku.MerchantID {
			return nil, fmt.Errorf("SKU %d 的商品引用不一致", skuID)
		}
		if product.Status != model.ProductStatusOnSale {
			return nil, fmt.Errorf("商品 %d 已下架", product.ID)
		}
		merchant, exists := merchantMap[sku.MerchantID]
		if !exists || merchant.Status != model.StatusEnabled {
			return nil, fmt.Errorf("商户 %d 不存在或已停用", sku.MerchantID)
		}
		subtotal, err := pricing.CalculateSubtotal(sku.Price, quantity)
		if err != nil {
			return nil, err
		}
		group := groupsByMerchant[merchant.ID]
		if group == nil {
			group = &tradeQuoteGroup{Merchant: merchant}
			groupsByMerchant[merchant.ID] = group
		}
		group.Items = append(group.Items, tradeQuoteItem{
			Product: product,
			SKU:     sku,
			Response: dto.OrderItemResponse{
				ProductID:   product.ID,
				SKUID:       sku.ID,
				ProductName: product.Name,
				SKUName:     sku.Name,
				SKUImage:    sku.Image,
				Price:       sku.Price,
				Quantity:    quantity,
				Subtotal:    subtotal,
			},
		})
	}

	for merchantID := range input.CouponByMerchant {
		if groupsByMerchant[merchantID] == nil {
			return nil, fmt.Errorf("商户 %d 不在本次交易中，不能使用其优惠券", merchantID)
		}
	}

	userCouponMap := make(map[int64]model.UserCoupon)
	couponMap := make(map[int64]model.Coupon)
	if len(input.UserCouponIDs) > 0 {
		userCoupons, err := repo.FindUserCouponsByIDs(ctx, userID, input.UserCouponIDs, forUpdate)
		if err != nil {
			return nil, err
		}
		couponIDSet := make(map[int64]struct{})
		for _, userCoupon := range userCoupons {
			userCouponMap[userCoupon.ID] = userCoupon
			couponIDSet[userCoupon.CouponID] = struct{}{}
		}
		for _, id := range input.UserCouponIDs {
			if _, exists := userCouponMap[id]; !exists {
				return nil, fmt.Errorf("用户优惠券 %d 不存在", id)
			}
		}
		coupons, err := repo.FindCouponsByIDs(ctx, sortedInt64Set(couponIDSet), forUpdate)
		if err != nil {
			return nil, err
		}
		for _, coupon := range coupons {
			couponMap[coupon.ID] = coupon
		}
	}

	quote := &tradeQuote{Address: *address}
	groupGoodsAmounts := make([]int64, 0, len(merchantIDs))
	groupFreightAmounts := make([]int64, 0, len(merchantIDs))
	groupDiscountAmounts := make([]int64, 0, len(merchantIDs))
	groupPayableAmounts := make([]int64, 0, len(merchantIDs))
	for _, merchantID := range merchantIDs {
		group := groupsByMerchant[merchantID]
		subtotals := make([]int64, len(group.Items))
		for index := range group.Items {
			subtotals[index] = group.Items[index].Response.Subtotal
		}
		group.GoodsAmount, err = pricing.SumSubtotals(subtotals)
		if err != nil {
			return nil, err
		}
		if userCouponID := input.CouponByMerchant[merchantID]; userCouponID > 0 {
			userCoupon := userCouponMap[userCouponID]
			if userCoupon.Status != model.UserCouponStatusUnused || userCoupon.MerchantID != merchantID {
				return nil, fmt.Errorf("商户 %d 的用户优惠券不可用", merchantID)
			}
			coupon, exists := couponMap[userCoupon.CouponID]
			if !exists || coupon.MerchantID != merchantID || coupon.MerchantID == 0 || coupon.Status != model.CouponStatusActive {
				return nil, fmt.Errorf("商户 %d 的优惠券不可用", merchantID)
			}
			if now.Before(coupon.StartAt) || !now.Before(coupon.EndAt) || group.GoodsAmount < coupon.ThresholdAmount {
				return nil, fmt.Errorf("商户 %d 的优惠券不满足使用条件", merchantID)
			}
			group.UserCouponID = userCoupon.ID
			group.CouponID = coupon.ID
			group.DiscountAmount = coupon.DiscountAmount
			if group.DiscountAmount > group.GoodsAmount {
				group.DiscountAmount = group.GoodsAmount
			}
		}
		itemAmounts, err := pricing.AllocateOrderItems(subtotals, group.DiscountAmount)
		if err != nil {
			return nil, err
		}
		for index := range group.Items {
			group.Items[index].Response.DiscountAmount = itemAmounts[index].DiscountAmount
			group.Items[index].Response.PayableAmount = itemAmounts[index].PayableAmount
		}
		group.PayableAmount = group.GoodsAmount + group.FreightAmount - group.DiscountAmount
		quote.Groups = append(quote.Groups, *group)
		groupGoodsAmounts = append(groupGoodsAmounts, group.GoodsAmount)
		groupFreightAmounts = append(groupFreightAmounts, group.FreightAmount)
		groupDiscountAmounts = append(groupDiscountAmounts, group.DiscountAmount)
		groupPayableAmounts = append(groupPayableAmounts, group.PayableAmount)
	}
	quote.GoodsAmount, err = pricing.SumSubtotals(groupGoodsAmounts)
	if err != nil {
		return nil, err
	}
	quote.FreightAmount, err = pricing.SumSubtotals(groupFreightAmounts)
	if err != nil {
		return nil, err
	}
	quote.DiscountAmount, err = pricing.SumSubtotals(groupDiscountAmounts)
	if err != nil {
		return nil, err
	}
	quote.PayableAmount, err = pricing.SumSubtotals(groupPayableAmounts)
	if err != nil {
		return nil, err
	}
	return quote, nil
}

func sortedInt64Set(values map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func quotePreviewResponse(quote *tradeQuote, token string) *dto.TradePreviewResponse {
	groups := make([]dto.TradeMerchantGroupResponse, 0, len(quote.Groups))
	for _, group := range quote.Groups {
		items := make([]dto.OrderItemResponse, 0, len(group.Items))
		for _, item := range group.Items {
			items = append(items, item.Response)
		}
		groups = append(groups, dto.TradeMerchantGroupResponse{
			MerchantID:     group.Merchant.ID,
			MerchantName:   group.Merchant.Name,
			MerchantLogo:   group.Merchant.Logo,
			Items:          items,
			GoodsAmount:    group.GoodsAmount,
			FreightAmount:  group.FreightAmount,
			DiscountAmount: group.DiscountAmount,
			PayableAmount:  group.PayableAmount,
			UserCouponID:   group.UserCouponID,
		})
	}
	return &dto.TradePreviewResponse{
		IdempotencyToken: token,
		Address: dto.AddressResponse{
			ID:            quote.Address.ID,
			ReceiverName:  quote.Address.ReceiverName,
			ReceiverPhone: quote.Address.ReceiverPhone,
			Province:      quote.Address.Province,
			City:          quote.Address.City,
			District:      quote.Address.District,
			Detail:        quote.Address.Detail,
			IsDefault:     quote.Address.IsDefault,
		},
		MerchantGroups: groups,
		GoodsAmount:    quote.GoodsAmount,
		FreightAmount:  quote.FreightAmount,
		DiscountAmount: quote.DiscountAmount,
		PayableAmount:  quote.PayableAmount,
	}
}

func tradeQuoteFingerprint(quote *tradeQuote) (string, error) {
	payload := quotePreviewResponse(quote, "")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("生成交易预览指纹: %w", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}
