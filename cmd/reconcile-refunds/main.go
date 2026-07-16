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
	if !cfg.Payment.Refund.ReconcileEnabled {
		fmt.Println("refund reconciliation job is disabled")
		return
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}

	job := service.NewRefundReconciliationService(repository.NewAfterSaleRepository(db), cfg.Payment)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	report, err := job.Run(ctx, time.Now())
	fmt.Printf(
		"refund reconciliation completed: scanned=%d succeeded=%d processing=%d failed=%d errors=%d\n",
		report.Scanned,
		report.Succeeded,
		report.Processing,
		report.Failed,
		report.Errors,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
