package pricing

import (
	"reflect"
	"testing"
)

func TestAllocateOrderItems(t *testing.T) {
	tests := []struct {
		name      string
		subtotals []int64
		discount  int64
		want      []OrderItemAmount
	}{
		{
			name:      "no discount",
			subtotals: []int64{100, 200},
			want:      []OrderItemAmount{{PayableAmount: 100}, {PayableAmount: 200}},
		},
		{
			name:      "rounding remainder goes to final item",
			subtotals: []int64{100, 100, 100},
			discount:  100,
			want: []OrderItemAmount{
				{DiscountAmount: 33, PayableAmount: 67},
				{DiscountAmount: 33, PayableAmount: 67},
				{DiscountAmount: 34, PayableAmount: 66},
			},
		},
		{
			name:      "uneven prices",
			subtotals: []int64{199, 301},
			discount:  101,
			want: []OrderItemAmount{
				{DiscountAmount: 40, PayableAmount: 159},
				{DiscountAmount: 61, PayableAmount: 240},
			},
		},
		{
			name:      "zero price item does not receive remainder",
			subtotals: []int64{100, 0},
			discount:  1,
			want: []OrderItemAmount{
				{DiscountAmount: 1, PayableAmount: 99},
				{},
			},
		},
		{
			name:      "full discount",
			subtotals: []int64{99, 1},
			discount:  100,
			want: []OrderItemAmount{
				{DiscountAmount: 99},
				{DiscountAmount: 1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := AllocateOrderItems(test.subtotals, test.discount)
			if err != nil {
				t.Fatalf("AllocateOrderItems returned error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("unexpected allocation: got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestAllocateOrderItemsRejectsInvalidAmounts(t *testing.T) {
	for _, test := range []struct {
		name      string
		subtotals []int64
		discount  int64
	}{
		{name: "negative subtotal", subtotals: []int64{-1}, discount: 0},
		{name: "negative discount", subtotals: []int64{100}, discount: -1},
		{name: "discount exceeds goods", subtotals: []int64{100}, discount: 101},
		{name: "goods overflow", subtotals: []int64{maxInt64, 1}, discount: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := AllocateOrderItems(test.subtotals, test.discount); err == nil {
				t.Fatal("expected allocation error")
			}
		})
	}
}

func TestAllocateOrderItemsDoesNotOverflowDuringMultiplication(t *testing.T) {
	got, err := AllocateOrderItems([]int64{maxInt64 - 1, 1}, maxInt64-1)
	if err != nil {
		t.Fatalf("AllocateOrderItems returned error: %v", err)
	}
	if got[0].DiscountAmount != maxInt64-2 || got[1].DiscountAmount != 1 {
		t.Fatalf("unexpected large allocation: %+v", got)
	}
}

func TestCalculateSubtotalAndSumSubtotalsRejectOverflow(t *testing.T) {
	if got, err := CalculateSubtotal(1990, 3); err != nil || got != 5970 {
		t.Fatalf("unexpected subtotal: got %d, err=%v", got, err)
	}
	if _, err := CalculateSubtotal(maxInt64, 2); err == nil {
		t.Fatal("expected subtotal overflow error")
	}
	if _, err := SumSubtotals([]int64{maxInt64, 1}); err == nil {
		t.Fatal("expected goods amount overflow error")
	}
}
