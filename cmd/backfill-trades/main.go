package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/migration"
	"go-mall/internal/trademigration"
	migrationfiles "go-mall/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "M8 trade migration failed:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "config file path")
	command := flag.String("command", "source", "source, backfill or verify")
	batchSize := flag.Int("batch-size", 100, "orders per transaction, 1-1000")
	timeout := flag.Duration("timeout", 30*time.Minute, "command timeout")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		return fmt.Errorf("init MySQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	defer sqlDB.Close()

	items, err := migration.LoadFiles(migrationfiles.Files)
	if err != nil {
		return err
	}
	runner, err := migration.NewRunner(sqlDB, items)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	statuses, err := runner.Status(ctx)
	if err != nil {
		return fmt.Errorf("verify migration history: %w", err)
	}
	for _, status := range statuses {
		if !status.Applied {
			return fmt.Errorf("migration %06d_%s is pending; apply all schema migrations before backfill", status.Migration.Version, status.Migration.Name)
		}
	}

	service, err := trademigration.NewService(sqlDB)
	if err != nil {
		return err
	}
	switch *command {
	case "source":
		report, err := service.ValidateSource(ctx)
		if err != nil {
			return err
		}
		printReport("M8 legacy source validation", report)
		if report.HasIssues() {
			return fmt.Errorf("legacy source validation failed: %s", report.IssueSummary())
		}
	case "backfill":
		if os.Getenv("MALL_ALLOW_M8_BACKFILL") != "1" {
			return fmt.Errorf("refusing historical data mutation: set MALL_ALLOW_M8_BACKFILL=1 explicitly")
		}
		result, err := service.Backfill(ctx, *batchSize)
		if err != nil {
			return err
		}
		fmt.Printf("backfill completed: orders=%d trades=%d payments=%d allocations=%d refunds=%d\n", result.Orders, result.Trades, result.Payments, result.Allocations, result.Refunds)
		report, err := service.ValidateConsistency(ctx)
		if err != nil {
			return err
		}
		printReport("M8 post-backfill consistency", report)
		if report.HasIssues() {
			return fmt.Errorf("post-backfill validation failed: %s", report.IssueSummary())
		}
	case "verify":
		report, err := service.ValidateConsistency(ctx)
		if err != nil {
			return err
		}
		printReport("M8 trade consistency", report)
		if report.HasIssues() {
			return fmt.Errorf("trade consistency validation failed: %s", report.IssueSummary())
		}
	default:
		return fmt.Errorf("unsupported command %q", *command)
	}
	return nil
}

func printReport(title string, report trademigration.ValidationReport) {
	fmt.Println(title)
	fmt.Println("generated_at:", report.GeneratedAt.Format(time.RFC3339))
	for _, metric := range report.Metrics {
		fmt.Printf("metric %-40s %d\n", metric.Name, metric.Count)
	}
	for _, check := range report.Checks {
		state := "ok"
		if check.Count > 0 {
			state = "FAILED"
		}
		fmt.Printf("check  %-40s %-6s count=%d %s\n", check.Name, state, check.Count, check.Description)
	}
}
