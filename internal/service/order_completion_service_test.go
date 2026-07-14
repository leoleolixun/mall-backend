package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/config"
)

type fakeOrderCompletionRepository struct {
	ids       []int64
	completed map[int64]bool
	failID    int64
	before    time.Time
}

func (r *fakeOrderCompletionRepository) ListExpiredShippedOrderIDs(_ context.Context, before time.Time, _ int) ([]int64, error) {
	r.before = before
	return append([]int64(nil), r.ids...), nil
}

func (r *fakeOrderCompletionRepository) CompleteShippedOrder(_ context.Context, id int64, _ time.Time) (bool, error) {
	if id == r.failID {
		return false, fmt.Errorf("db error")
	}
	return r.completed[id], nil
}

func TestOrderCompletionServiceCompletesExpiredShipments(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	repo := &fakeOrderCompletionRepository{ids: []int64{1, 2, 3}, completed: map[int64]bool{1: true}, failID: 3}
	service := NewOrderCompletionService(repo, config.OrderConfig{ShippedAutoCompleteDays: 7, CompleteBatchSize: 50})
	report, err := service.Run(context.Background(), now)
	if err == nil {
		t.Fatal("expected joined failure")
	}
	if report.Scanned != 3 || report.Completed != 1 || report.Skipped != 1 || report.Failed != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !repo.before.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("unexpected cutoff: %s", repo.before)
	}
}
