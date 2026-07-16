package trademigration

import "testing"

func TestHistoricalTradeStatus(t *testing.T) {
	tests := []struct {
		name     string
		order    int
		payable  int64
		refunded int64
		expected int
	}{
		{name: "pending", order: orderStatusPendingPayment, payable: 100, expected: tradeStatusPendingPayment},
		{name: "paid", order: 2, payable: 100, expected: tradeStatusPaid},
		{name: "cancelled", order: orderStatusCancelled, payable: 100, expected: tradeStatusClosed},
		{name: "partial refund", order: 2, payable: 100, refunded: 30, expected: tradeStatusPartiallyRefunded},
		{name: "full refund", order: 2, payable: 100, refunded: 100, expected: tradeStatusRefunded},
		{name: "over refund remains refunded", order: 2, payable: 100, refunded: 120, expected: tradeStatusRefunded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := historicalTradeStatus(test.order, test.payable, test.refunded); got != test.expected {
				t.Fatalf("historicalTradeStatus()=%d, expected %d", got, test.expected)
			}
		})
	}
}

func TestValidationReportIssueSummary(t *testing.T) {
	report := ValidationReport{Checks: []CheckResult{
		{Name: "clean", Count: 0},
		{Name: "first", Count: 2},
		{Name: "second", Count: 1},
	}}
	if !report.HasIssues() {
		t.Fatal("expected report to contain issues")
	}
	if got := report.IssueSummary(); got != "first=2, second=1" {
		t.Fatalf("unexpected issue summary: %s", got)
	}
}
