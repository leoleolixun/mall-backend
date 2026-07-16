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
	migrationfiles "go-mall/migrations"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	command := flag.String("command", "status", "status, verify, up or down")
	steps := flag.Int("steps", 1, "number of migrations to roll back")
	timeout := flag.Duration("timeout", 10*time.Minute, "command timeout")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Errorf("get sql db failed: %w", err))
	}
	migrations, err := migration.LoadFiles(migrationfiles.Files)
	if err != nil {
		panic(err)
	}
	runner, err := migration.NewRunner(sqlDB, migrations)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	switch *command {
	case "status":
		statuses, err := runner.Status(ctx)
		if err != nil {
			panic(err)
		}
		for _, status := range statuses {
			state := "pending"
			if status.Applied {
				state = "applied " + status.AppliedAt.Format(time.RFC3339)
			}
			fmt.Printf("%06d %-48s %s checksum=%s\n", status.Migration.Version, status.Migration.Name, state, status.Migration.Checksum[:12])
		}
	case "verify":
		if err := runner.Verify(ctx); err != nil {
			panic(err)
		}
		fmt.Println("schema migration history verified")
	case "up":
		requireEnv("MALL_ALLOW_SCHEMA_MIGRATION")
		completed, err := runner.Up(ctx)
		if err != nil {
			panic(err)
		}
		for _, item := range completed {
			fmt.Printf("applied %06d_%s\n", item.Version, item.Name)
		}
		fmt.Printf("schema migration completed: %d applied\n", len(completed))
	case "down":
		requireEnv("MALL_ALLOW_SCHEMA_ROLLBACK")
		completed, err := runner.Down(ctx, *steps)
		if err != nil {
			panic(err)
		}
		for _, item := range completed {
			fmt.Printf("rolled back %06d_%s\n", item.Version, item.Name)
		}
	default:
		panic(fmt.Errorf("unsupported command %q", *command))
	}
}

func requireEnv(name string) {
	if os.Getenv(name) != "1" {
		panic(fmt.Errorf("refusing schema mutation: set %s=1 explicitly", name))
	}
}
