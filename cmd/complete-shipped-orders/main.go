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
	if !cfg.Order.AutoCompleteEnabled {
		fmt.Println("shipped order auto-complete job is disabled")
		return
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}
	job := service.NewOrderCompletionService(repository.NewOrderCompletionRepository(db), cfg.Order)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	report, err := job.Run(ctx, time.Now())
	fmt.Printf("shipped order job completed: scanned=%d completed=%d skipped=%d failed=%d\n", report.Scanned, report.Completed, report.Skipped, report.Failed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
