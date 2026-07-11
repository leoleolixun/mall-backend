package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/repository"
	"go-mall/internal/service"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}
	if !cfg.Order.CancelExpiredEnabled {
		fmt.Println("expired order job is disabled")
		return
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}

	paymentRepo := repository.NewPaymentRepository(db)
	paymentService := service.NewPaymentService(paymentRepo, cfg.Payment)
	timeoutRepo := repository.NewOrderTimeoutRepository(db)
	timeoutService := service.NewOrderTimeoutService(timeoutRepo, paymentService, cfg.Order)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	report, err := timeoutService.Run(ctx, time.Now())
	fmt.Printf(
		"expired order job completed: scanned=%d cancelled=%d paid=%d skipped=%d failed=%d\n",
		report.Scanned,
		report.Cancelled,
		report.Paid,
		report.Skipped,
		report.Failed,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
