package service

import (
	"strings"
	"testing"

	"go-mall/internal/dto"
	"go-mall/internal/model"
)

func TestNormalizeTradeInputAggregatesAndSorts(t *testing.T) {
	input, err := normalizeTradeInput(
		[]dto.OrderRequestItem{{SKUID: 9, Quantity: 1}, {SKUID: 2, Quantity: 2}, {SKUID: 9, Quantity: 3}},
		[]dto.MerchantCouponSelection{{MerchantID: 8, UserCouponID: 80}, {MerchantID: 3, UserCouponID: 30}},
	)
	if err != nil {
		t.Fatalf("normalize trade input: %v", err)
	}
	if len(input.SKUIds) != 2 || input.SKUIds[0] != 2 || input.SKUIds[1] != 9 || input.Quantities[9] != 4 {
		t.Fatalf("unexpected normalized items: %+v", input)
	}
	if len(input.UserCouponIDs) != 2 || input.UserCouponIDs[0] != 30 || input.UserCouponIDs[1] != 80 {
		t.Fatalf("unexpected normalized coupons: %+v", input.UserCouponIDs)
	}
}

func TestNormalizeTradeInputRejectsPlatformAndDuplicateMerchantCoupons(t *testing.T) {
	items := []dto.OrderRequestItem{{SKUID: 1, Quantity: 1}}
	_, err := normalizeTradeInput(items, []dto.MerchantCouponSelection{{MerchantID: 0, UserCouponID: 1}})
	if err == nil || !strings.Contains(err.Error(), "平台优惠券") {
		t.Fatalf("expected platform coupon error, got %v", err)
	}
	_, err = normalizeTradeInput(items, []dto.MerchantCouponSelection{{MerchantID: 1, UserCouponID: 1}, {MerchantID: 1, UserCouponID: 2}})
	if err == nil || !strings.Contains(err.Error(), "只能选择一张") {
		t.Fatalf("expected duplicate merchant coupon error, got %v", err)
	}
}

func TestCalculateCommissionAmountUsesBasisPointsWithoutOverflow(t *testing.T) {
	amount, err := calculateCommissionAmount(12345, 375)
	if err != nil || amount != 462 {
		t.Fatalf("unexpected commission: amount=%d err=%v", amount, err)
	}
	amount, err = calculateCommissionAmount(9_000_000_000_000_000_000, 10000)
	if err != nil || amount != 9_000_000_000_000_000_000 {
		t.Fatalf("large commission overflowed: amount=%d err=%v", amount, err)
	}
	if _, err := calculateCommissionAmount(100, 10001); err == nil {
		t.Fatal("invalid commission rate was accepted")
	}
}

func TestTradeQuoteFingerprintIncludesAddressAndPriceSnapshots(t *testing.T) {
	quote := &tradeQuote{
		Address: model.Address{ID: 1, ReceiverName: "张三", ReceiverPhone: "13800000000", Province: "浙江省", City: "杭州市", District: "西湖区", Detail: "文三路 1 号"},
		Groups: []tradeQuoteGroup{{
			Merchant:    model.Merchant{ID: 1, Name: "商户一"},
			Items:       []tradeQuoteItem{{Response: dto.OrderItemResponse{ProductID: 1, SKUID: 2, ProductName: "商品", SKUName: "规格", Price: 100, Quantity: 2, Subtotal: 200, PayableAmount: 200}}},
			GoodsAmount: 200, PayableAmount: 200,
		}},
		GoodsAmount: 200, PayableAmount: 200,
	}
	base, err := tradeQuoteFingerprint(quote)
	if err != nil {
		t.Fatalf("fingerprint quote: %v", err)
	}
	quote.Address.Detail = "文三路 2 号"
	addressChanged, err := tradeQuoteFingerprint(quote)
	if err != nil {
		t.Fatalf("fingerprint changed address: %v", err)
	}
	if addressChanged == base {
		t.Fatal("address snapshot change did not change fingerprint")
	}
	quote.Address.Detail = "文三路 1 号"
	quote.Groups[0].Items[0].Response.Price = 101
	priceChanged, err := tradeQuoteFingerprint(quote)
	if err != nil {
		t.Fatalf("fingerprint changed price: %v", err)
	}
	if priceChanged == base {
		t.Fatal("price snapshot change did not change fingerprint")
	}
}
