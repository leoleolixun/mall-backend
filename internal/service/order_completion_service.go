package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/repository"
)

type OrderCompletionReport struct {
	Scanned   int
	Completed int
	Skipped   int
	Failed    int
}
type OrderCompletionService interface {
	Run(ctx context.Context, now time.Time) (OrderCompletionReport, error)
}
type orderCompletionService struct {
	repo      repository.OrderCompletionRepository
	timeout   time.Duration
	batchSize int
}

func NewOrderCompletionService(repo repository.OrderCompletionRepository, cfg config.OrderConfig) OrderCompletionService {
	days := cfg.ShippedAutoCompleteDays
	if days <= 0 {
		days = 10
	}
	batch := cfg.CompleteBatchSize
	if batch <= 0 {
		batch = 100
	}
	if batch > 1000 {
		batch = 1000
	}
	return &orderCompletionService{repo: repo, timeout: time.Duration(days) * 24 * time.Hour, batchSize: batch}
}
func (s *orderCompletionService) Run(ctx context.Context, now time.Time) (OrderCompletionReport, error) {
	var report OrderCompletionReport
	ids, err := s.repo.ListExpiredShippedOrderIDs(ctx, now.Add(-s.timeout), s.batchSize)
	if err != nil {
		return report, err
	}
	report.Scanned = len(ids)
	var failures []error
	for _, id := range ids {
		completed, err := s.repo.CompleteShippedOrder(ctx, id, now)
		if err != nil {
			report.Failed++
			failures = append(failures, fmt.Errorf("订单 %d 自动完成失败: %w", id, err))
			continue
		}
		if completed {
			report.Completed++
		} else {
			report.Skipped++
		}
	}
	return report, errors.Join(failures...)
}
