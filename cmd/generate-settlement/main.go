package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/repository"
	"go-mall/internal/service"
)

func main() {
	configPath := flag.String("config", "config.yaml", "configuration file path")
	merchantID := flag.Int64("merchant-id", 0, "merchant ID")
	periodStartText := flag.String("period-start", "", "RFC3339 period start")
	periodEndText := flag.String("period-end", "", "RFC3339 period end")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fatal("load config", err)
	}
	if !cfg.Settlement.Enabled {
		fatal("generate settlement", fmt.Errorf("settlement.enabled is false"))
	}
	if *merchantID <= 0 {
		fatal("parse merchant", fmt.Errorf("merchant-id must be greater than zero"))
	}
	periodStart, err := time.Parse(time.RFC3339, *periodStartText)
	if err != nil {
		fatal("parse period-start", err)
	}
	periodEnd, err := time.Parse(time.RFC3339, *periodEndText)
	if err != nil {
		fatal("parse period-end", err)
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		fatal("init mysql", err)
	}
	settlementService := service.NewSettlementService(repository.NewSettlementRepository(db), cfg.Settlement)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for {
		report, err := settlementService.AccrueCompletedOrders(ctx)
		if err != nil {
			fatal("accrue completed orders", err)
		}
		fmt.Printf("settlement accrual: scanned=%d accrued=%d skipped=%d\n", report.Scanned, report.Accrued, report.Skipped)
		if report.Scanned == 0 {
			break
		}
	}
	settlement, err := settlementService.Generate(ctx, *merchantID, periodStart, periodEnd)
	if err != nil {
		fatal("generate settlement", err)
	}
	fmt.Printf(
		"settlement generated: id=%d no=%s merchant=%d gross=%d commission=%d refund=%d net=%d entries=%d\n",
		settlement.ID, settlement.SettlementNo, settlement.MerchantID,
		settlement.GrossAmount, settlement.CommissionAmount, settlement.RefundAmount,
		settlement.NetAmount, len(settlement.Entries),
	)
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s failed: %v\n", action, err)
	os.Exit(1)
}
