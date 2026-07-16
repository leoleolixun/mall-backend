package pricing

import (
	"fmt"
	"math/bits"
)

const maxInt64 = int64(^uint64(0) >> 1)

type OrderItemAmount struct {
	DiscountAmount int64
	PayableAmount  int64
}

func CalculateSubtotal(price int64, quantity int) (int64, error) {
	if price < 0 || quantity <= 0 {
		return 0, fmt.Errorf("商品单价和数量不合法")
	}
	quantity64 := int64(quantity)
	if price > 0 && quantity64 > maxInt64/price {
		return 0, fmt.Errorf("商品小计超出范围")
	}
	return price * quantity64, nil
}

func SumSubtotals(subtotals []int64) (int64, error) {
	var total int64
	for i, subtotal := range subtotals {
		if subtotal < 0 {
			return 0, fmt.Errorf("第 %d 项商品小计不能小于 0", i+1)
		}
		if subtotal > maxInt64-total {
			return 0, fmt.Errorf("商品总金额超出范围")
		}
		total += subtotal
	}
	return total, nil
}

// AllocateOrderItems distributes an order-level discount proportionally. The
// final positive item receives the rounding remainder so the totals stay exact.
func AllocateOrderItems(subtotals []int64, discountAmount int64) ([]OrderItemAmount, error) {
	amounts := make([]OrderItemAmount, len(subtotals))
	if discountAmount < 0 {
		return nil, fmt.Errorf("优惠金额不能小于 0")
	}

	goodsAmount, err := SumSubtotals(subtotals)
	if err != nil {
		return nil, err
	}
	lastPositive := -1
	for i, subtotal := range subtotals {
		if subtotal > 0 {
			lastPositive = i
		}
	}

	if discountAmount > goodsAmount {
		return nil, fmt.Errorf("优惠金额不能超过商品总金额")
	}
	if goodsAmount == 0 {
		return amounts, nil
	}

	remainingDiscount := discountAmount
	for i, subtotal := range subtotals {
		itemDiscount := int64(0)
		switch {
		case i == lastPositive:
			itemDiscount = remainingDiscount
		case subtotal > 0 && discountAmount > 0:
			value, err := multiplyDivideFloor(discountAmount, subtotal, goodsAmount)
			if err != nil {
				return nil, err
			}
			itemDiscount = value
		}

		if itemDiscount < 0 || itemDiscount > subtotal || itemDiscount > remainingDiscount {
			return nil, fmt.Errorf("第 %d 项优惠分摊结果不合法", i+1)
		}
		amounts[i] = OrderItemAmount{
			DiscountAmount: itemDiscount,
			PayableAmount:  subtotal - itemDiscount,
		}
		remainingDiscount -= itemDiscount
	}

	if remainingDiscount != 0 {
		return nil, fmt.Errorf("优惠金额分摊不完整")
	}
	return amounts, nil
}

func multiplyDivideFloor(value, part, total int64) (int64, error) {
	if value < 0 || part < 0 || total <= 0 || value > total || part > total {
		return 0, fmt.Errorf("优惠分摊参数不合法")
	}
	if value == 0 || part == 0 {
		return 0, nil
	}

	high, low := bits.Mul64(uint64(value), uint64(part))
	if high >= uint64(total) {
		return 0, fmt.Errorf("优惠分摊结果超出范围")
	}
	quotient, _ := bits.Div64(high, low, uint64(total))
	if quotient > uint64(maxInt64) {
		return 0, fmt.Errorf("优惠分摊结果超出范围")
	}
	return int64(quotient), nil
}
